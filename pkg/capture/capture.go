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
		seconds = 62
	}

	script := fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
$sn = "SREDiscovery-" + ([guid]::NewGuid().ToString('N').Substring(0,8))
$etl = Join-Path $env:TEMP ($sn + ".etl")
$started = $false
try {
    Get-NetEventSession -Name "SREDiscovery-*" -ErrorAction SilentlyContinue | Remove-NetEventSession -Confirm:$false -ErrorAction SilentlyContinue
    New-NetEventSession -Name $sn -LocalFilePath $etl -CaptureMode SaveToFile -ErrorAction Stop | Out-Null
    Add-NetEventPacketCaptureProvider -SessionName $sn -CaptureType Physical -TruncationLength 0 -LinkLayerAddress @('01-80-C2-00-00-0E','01-80-C2-00-00-03','01-80-C2-00-00-00','01-00-0C-CC-CC-CC') -ErrorAction Stop | Out-Null
    if ("%s") {
        Add-NetEventNetworkAdapter -Name "%s" -PromiscuousMode $true -ErrorAction Stop | Out-Null
    }
    Start-NetEventSession -Name $sn -ErrorAction Stop | Out-Null
    $started = $true
    Start-Sleep -Seconds %d
    Stop-NetEventSession -Name $sn -ErrorAction Stop | Out-Null
    $started = $false

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
`, adapterName, adapterName, seconds)

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
