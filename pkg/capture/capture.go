package capture

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/RaidenMercury/sre-network-discovery/pkg/protocol"
)

type PacketCapture interface {
	Capture(ctx context.Context, adapterName string, duration time.Duration) ([]protocol.DiscoveryResult, error)
}

type WindowsETWCapture struct{}

func NewCaptureEngine() PacketCapture {
	return &WindowsETWCapture{}
}

func (e *WindowsETWCapture) Capture(ctx context.Context, adapterName string, duration time.Duration) ([]protocol.DiscoveryResult, error) {
	if runtime.GOOS != "windows" {
		return nil, fmt.Errorf("la captura de tramas CDP/LLDP por ETW requiere el sistema operativo Windows")
	}

	seconds := int(duration.Seconds())
	if seconds <= 0 {
		seconds = 65
	}

	cleanAdapter := strings.ReplaceAll(adapterName, "'", "''")

	script := fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
$sn = "SREDiscovery-" + ([guid]::NewGuid().ToString('N').Substring(0,8))
$etl = Join-Path $env:TEMP ($sn + ".etl")
$started = $false

try {
    # 1. Limpieza rigurosa de cualquier sesion colgada previa
    Get-NetEventSession -ErrorAction SilentlyContinue | ForEach-Object {
        if ($_.SessionStatus -eq 'Running') {
            Stop-NetEventSession -Name $_.Name -ErrorAction SilentlyContinue | Out-Null
        }
        Remove-NetEventSession -Name $_.Name -Confirm:$false -ErrorAction SilentlyContinue | Out-Null
    }

    # 2. Inicializar nueva sesion ETW
    New-NetEventSession -Name $sn -LocalFilePath $etl -CaptureMode SaveToFile -ErrorAction Stop | Out-Null

    # 3. Proveedor con filtros de multicast CDP y LLDP
    $llAddrs = @('01-80-C2-00-00-0E','01-80-C2-00-00-03','01-80-C2-00-00-00','01-00-0C-CC-CC-CC')
    Add-NetEventPacketCaptureProvider -SessionName $sn -CaptureType Physical -TruncationLength 0 -LinkLayerAddress $llAddrs -ErrorAction Stop | Out-Null

    # 4. Seleccion y vinculacion robusta de interfaces (soporta dongles USB-C, docking y Ethernet)
    $targetAdapters = @()
    if ('%s') {
        $targetAdapters = @(Get-NetAdapter -ErrorAction SilentlyContinue | Where-Object { 
            $_.Name -eq '%s' -or $_.InterfaceAlias -eq '%s' -or $_.InterfaceDescription -like '*%s*' 
        })
    }
    if ($targetAdapters.Count -eq 0) {
        $targetAdapters = @(Get-NetAdapter -ErrorAction SilentlyContinue | Where-Object { 
            $_.Status -eq 'Up' -and $_.InterfaceType -notin @(23, 24) 
        })
    }

    foreach ($ad in $targetAdapters) {
        Add-NetEventNetworkAdapter -Name $ad.Name -PromiscuousMode $true -ErrorAction SilentlyContinue | Out-Null
    }

    Start-NetEventSession -Name $sn -ErrorAction Stop | Out-Null
    $started = $true

    # 5. Espera con margen para superar Spanning Tree y ciclo de broadcast
    Start-Sleep -Seconds %d

    Stop-NetEventSession -Name $sn -ErrorAction Stop | Out-Null
    $started = $false

    # 6. Extraccion de eventos de fragmento de red
    $events = @(Get-WinEvent -Path $etl -Oldest -FilterXPath "*[System[EventID=1001]]" -ErrorAction SilentlyContinue)
    $xq = [string[]]@("Event/EventData/Data[@Name='Fragment']")
    $sel = [Diagnostics.Eventing.Reader.EventLogPropertySelector]::new($xq)
    foreach ($ev in $events) {
        $v = $ev.GetPropertyValues($sel)
        if ($v.Count -ge 1 -and $v[0]) {
            $bytes = [byte[]]$v[0]
            if ($bytes.Length -ge 14) {
                [BitConverter]::ToString($bytes) -replace '-'
            }
        }
    }
} finally {
    if ($started) { Stop-NetEventSession -Name $sn -ErrorAction SilentlyContinue | Out-Null }
    Remove-NetEventSession -Name $sn -Confirm:$false -ErrorAction SilentlyContinue | Out-Null
    if (Test-Path -LiteralPath $etl) { Remove-Item -LiteralPath $etl -Force -ErrorAction SilentlyContinue }
}
`, cleanAdapter, cleanAdapter, cleanAdapter, cleanAdapter, seconds)

	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("fallo en sesion ETW: %s", msg)
	}

	lines := strings.Split(outBuf.String(), "\n")
	var results []protocol.DiscoveryResult
	seenKeys := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) < 28 {
			continue
		}
		raw, err := hex.DecodeString(line)
		if err != nil {
			continue
		}

		env := protocol.ParseEnvelope(raw)
		if env == nil {
			continue
		}

		var res *protocol.DiscoveryResult
		if env.Protocol == "LLDP" {
			res = protocol.ParseLLDP(raw)
		} else if env.Protocol == "CDP" {
			res = protocol.ParseCDP(raw)
		}

		if res != nil && (res.SwitchName != "" || res.Port != "") {
			key := fmt.Sprintf("%s/%s/%s", res.Protocol, res.SwitchName, res.Port)
			if !seenKeys[key] {
				seenKeys[key] = true
				results = append(results, *res)
			}
		}
	}

	return results, nil
}
