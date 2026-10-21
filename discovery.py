"""
discovery.py
Módulo de descubrimiento de red (CDP/LLDP) vía ETW en Windows.
Expone una API simple: run_discovery(adapter_name) -> (data, error)
"""

import ctypes
import json
import os
import subprocess
import sys
import threading
import time

import psutil

# ============================================================
# CONSTANTES
# ============================================================
IS_WINDOWS = sys.platform == "win32"
APP_AUMID = "raidenmercury.srenetworkdiscovery.scanner.1.0"
BLACKLIST_ADAPTERS = (
    "loopback", "vethernet", "hyper-v", "wsl",
    "vmware", "virtualbox", "docker", "br-",
    "tailscale", "zerotier", "wireguard", "openvpn",
    "nordvpn", "protonvpn", "tap-", "tun",
)


# ============================================================
# UTILIDADES
# ============================================================
def resource_path(relative_path: str) -> str:
    """Devuelve ruta absoluta compatible con PyInstaller."""
    base_path = getattr(sys, "_MEIPASS", os.path.abspath("."))
    return os.path.join(base_path, relative_path)


def get_network_adapters() -> list[str]:
    """Lista adaptadores activos (excluyendo virtuales/loopback)."""
    try:
        stats = psutil.net_if_stats()
        adapters = [
            name for name, s in stats.items()
            if s.isup and not any(b in name.lower() for b in BLACKLIST_ADAPTERS)
        ]
        return adapters or ["Ethernet"]
    except Exception:
        return ["Ethernet"]


def _is_admin() -> bool:
    """Chequea si el proceso tiene privilegios de Administrador."""
    if not IS_WINDOWS:
        return False

    # Método 1: API oficial (funciona en la mayoría de casos)
    try:
        if ctypes.windll.shell32.IsUserAnAdmin():
            return True
    except Exception:
        pass

    # Método 2: prueba empírica - solo administradores reales pueden
    # escribir y borrar en C:\Windows\System32
    try:
        sys32 = os.path.join(os.environ.get("SystemRoot", r"C:\Windows"), "System32")
        test_file = os.path.join(sys32, "_perm_test_etw.tmp")
        with open(test_file, "w") as f:
            f.write("x")
        os.remove(test_file)
        return True
    except Exception:
        pass

    return False


def ensure_admin() -> None:
    """Re-lanza el proceso con privilegios elevados si no los tiene."""
    if not IS_WINDOWS or _is_admin():
        return
    try:
        ctypes.windll.shell32.ShellExecuteW(
            None, "runas", sys.executable, " ".join(sys.argv[1:]), None, 1
        )
        sys.exit()
    except Exception:
        pass


def set_app_user_model_id() -> None:
    """Fija el AUMID para agrupar ventana/icono en la barra de tareas."""
    if not IS_WINDOWS:
        return
    try:
        ctypes.windll.shell32.SetCurrentProcessExplicitAppUserModelID(APP_AUMID)
    except Exception:
        pass


# ============================================================
# POWERHELL SCRIPT (ETW)
# ============================================================
_PS_SCRIPT_TEMPLATE = r"""
function Invoke-SwitchPortDiscovery {
    [CmdletBinding()]
    param(
        [string]$Protocol = 'Both',
        [int]$DurationSeconds = 62,
        [string]$AdapterName
    )
    $ErrorActionPreference = 'Stop'

    function Get-UInt16BE { param([byte[]]$B,[int]$O) return ([int]$B[$O] -shl 8) -bor [int]$B[$O+1] }
    function Get-UInt32BE { param([byte[]]$B,[int]$O) return [uint32](([uint32]$B[$O] -shl 24) -bor ([uint32]$B[$O+1] -shl 16) -bor ([uint32]$B[$O+2] -shl 8) -bor [uint32]$B[$O+3]) }
    function Mac { param([byte[]]$B) if(-not $B){return $null} return (($B|%{$_.ToString('X2')}) -join '-') }
    function Ascii { param([byte[]]$B) if(-not $B){return $null} return ([Text.Encoding]::ASCII.GetString($B)).Trim([char]0).Trim() }
    function Slice { param([byte[]]$B,[int]$O,[int]$L) if($L -le 0){return [byte[]]@()} return [byte[]]$B[$O..($O+$L-1)] }
    function Ip { param([byte[]]$B) if(-not $B -or $B.Length -notin @(4,16)){return $null} try{return ([Net.IPAddress]::new($B)).IPAddressToString}catch{return $null} }

    function Envelope {
        param([byte[]]$P)
        if($P.Length -lt 14){return $null}
        $dst = Mac (Slice $P 0 6); $src = Mac (Slice $P 6 6)
        $f = Get-UInt16BE $P 12; $off = 14; $vlan = $null
        if($f -in @(0x8100,0x88A8) -and $P.Length -ge 18){
            $vlan = (Get-UInt16BE $P 14) -band 0x0FFF
            $f = Get-UInt16BE $P 16; $off = 18
        }
        if($f -eq 0x88CC){return [pscustomobject]@{Proto='LLDP';Dst=$dst;Src=$src;Off=$off;Vlan=$vlan}}
        if($dst -eq '01-00-0C-CC-CC-CC' -and ($off+7) -lt $P.Length){
            $snap = Mac (Slice $P $off 8)
            if($snap -eq 'AA-AA-03-00-00-0C-20-00'){return [pscustomobject]@{Proto='CDP';Dst=$dst;Src=$src;Off=($off+8);Vlan=$vlan}}
        }
        return $null
    }

    function ParseLldp {
        param([byte[]]$P)
        $e = Envelope $P
        if(-not $e -or $e.Proto -ne 'LLDP'){return $null}
        $r = [ordered]@{Protocol='LLDP';SwitchName=$null;Port=$null;PortDescription=$null;VLAN=$null;SwitchIP=$null;Model=$null;ChassisId=$null;SystemDescription=$null;SourceMac=$e.Src}
        $mgmt = New-Object Collections.Generic.List[string]
        $o = [int]$e.Off
        while(($o+1) -lt $P.Length){
            $h = Get-UInt16BE $P $o; $t = $h -shr 9; $l = $h -band 0x01FF; $o += 2
            if($t -eq 0 -or $l -lt 0 -or ($o+$l) -gt $P.Length){break}
            $b = Slice $P $o $l
            switch($t){
                1 { if($b.Length -ge 2){ $r.ChassisId = if($b[0] -eq 4){Mac (Slice $b 1 ($b.Length-1))}else{Ascii (Slice $b 1 ($b.Length-1))} } }
                2 { if($b.Length -ge 2){ $r.Port = if($b[0] -eq 3){Mac (Slice $b 1 ($b.Length-1))}else{Ascii (Slice $b 1 ($b.Length-1))} } }
                4 { $r.PortDescription = Ascii $b }
                5 { $r.SwitchName = Ascii $b }
                6 { $r.SystemDescription = Ascii $b }
                8 {
                    if($b.Length -ge 3){
                        $alen=[int]$b[0]; $ast=[int]$b[1]
                        if($alen -ge 2 -and $alen -lt $b.Length){
                            $ab = Slice $b 2 ($alen-1)
                            if(($ast -eq 1 -and $ab.Length -eq 4) -or ($ast -eq 2 -and $ab.Length -eq 16)){
                                $ip = Ip $ab; if($ip){$mgmt.Add($ip)}
                            }
                        }
                    }
                }
                127 {
                    if($b.Length -ge 4){
                        $oui = Mac (Slice $b 0 3); $st = $b[3]
                        if($oui -eq '00-80-C2' -and $st -eq 1 -and $b.Length -ge 6){$r.VLAN = Get-UInt16BE $b 4}
                        if($oui -eq '00-12-BB' -and $st -eq 10 -and $b.Length -gt 4){$r.Model = Ascii (Slice $b 4 ($b.Length-4))}
                    }
                }
            }
            $o += $l
        }
        if($mgmt.Count -gt 0){
            $v4 = $mgmt | ?{ $_ -match '^\d{1,3}(\.\d{1,3}){3}$' } | select -First 1
            $r.SwitchIP = if($v4){$v4}else{$mgmt[0]}
        }
        if($null -eq $r.VLAN -and $null -ne $e.Vlan){$r.VLAN = $e.Vlan}
        return [pscustomobject]$r
    }

    function CdpAddrs {
        param([byte[]]$B)
        $a = New-Object Collections.Generic.List[string]
        if($B.Length -lt 4){return $a}
        try{$c=[int](Get-UInt32BE $B 0)}catch{return $a}
        $o = 4
        for($i=0; $i -lt $c; $i++){
            if(($o+1) -ge $B.Length){break}
            $pl=[int]$B[$o+1]; $o += 2
            if(($o+$pl+1) -ge $B.Length){break}
            $o += $pl
            $al = Get-UInt16BE $B $o; $o += 2
            if($al -le 0 -or ($o+$al) -gt $B.Length){break}
            $ab = Slice $B $o $al; $o += $al
            if($ab.Length -in @(4,16)){ $ip = Ip $ab; if($ip){$a.Add($ip)} }
        }
        return $a
    }

    function ParseCdp {
        param([byte[]]$P)
        $e = Envelope $P
        if(-not $e -or $e.Proto -ne 'CDP'){return $null}
        $co = [int]$e.Off
        if(($co+3) -ge $P.Length){return $null}
        $r = [ordered]@{Protocol='CDP';SwitchName=$null;Port=$null;PortDescription=$null;VLAN=$null;SwitchIP=$null;Model=$null;ChassisId=$e.Src;SystemDescription=$null;SourceMac=$e.Src}
        $o = $co + 4
        $addrs = New-Object Collections.Generic.List[string]
        while(($o+3) -lt $P.Length){
            $t = Get-UInt16BE $P $o; $tl = Get-UInt16BE $P ($o+2)
            if($tl -lt 4 -or ($o+$tl) -gt $P.Length){break}
            $b = Slice $P ($o+4) ($tl-4)
            switch($t){
                0x0001 { $r.SwitchName = Ascii $b }
                0x0002 { foreach($ip in (CdpAddrs $b)){$addrs.Add($ip)} }
                0x0003 { $r.Port = Ascii $b }
                0x0006 { $r.Model = Ascii $b }
                0x000A { if($b.Length -ge 2){$r.VLAN = Get-UInt16BE $b 0} }
                0x0016 { foreach($ip in (CdpAddrs $b)){ if(-not $addrs.Contains($ip)){$addrs.Add($ip)} } }
            }
            $o += $tl
        }
        if($addrs.Count -gt 0){
            $v4 = $addrs | ?{ $_ -match '^\d{1,3}(\.\d{1,3}){3}$' } | select -First 1
            $r.SwitchIP = if($v4){$v4}else{$addrs[0]}
        }
        if($null -eq $r.VLAN -and $null -ne $e.Vlan){$r.VLAN = $e.Vlan}
        return [pscustomobject]$r
    }

    $isAdmin = $false

    # Metodo 1: chequeo clasico .NET (puede fallar en PS 5.1 con tokens filtrados)
    try {
        $id = [Security.Principal.WindowsIdentity]::GetCurrent()
        $pr = New-Object Security.Principal.WindowsPrincipal $id
        if($pr.IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)){ $isAdmin = $true }
    } catch {}

    # Metodo 2: verificar el SID del grupo Administradores en el token actual
    if(-not $isAdmin){
        try {
            $adminSid = "S-1-5-32-544"
            $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
            foreach($g in $identity.Groups){
                if($g.Value -eq $adminSid){ $isAdmin = $true; break }
            }
        } catch {}
    }

    # Metodo 3: prueba empirica - escribir en System32 (solo admins reales pueden)
    if(-not $isAdmin){
        try {
            $testFile = Join-Path $env:SystemRoot "System32\_etw_perm_test_{0}.tmp" -f ([guid]::NewGuid().ToString('N').Substring(0,6))
            [IO.File]::WriteAllText($testFile, "test")
            Remove-Item -LiteralPath $testFile -Force -ErrorAction SilentlyContinue
            $isAdmin = $true
        } catch {}
    }

    if(-not $isAdmin){ throw "Requiere privilegios de Administrador." }

    if($AdapterName){
        $adapters = @(Get-NetAdapter -Name $AdapterName -ErrorAction SilentlyContinue | ?{ $_.Status -eq 'Up' -and $_.InterfaceType -eq 6 })
        if(-not $adapters){throw "Adaptador '$AdapterName' no encontrado o no esta en estado Up."}
    } else {
        $adapters = @(Get-NetAdapter | ?{ $_.Status -eq 'Up' -and $_.InterfaceType -eq 6 })
    }

    $exist = @(Get-NetEventSession -ErrorAction SilentlyContinue)
    if($exist.Count -gt 0){throw "Ya existe una NetEventSession colgada en el Kernel. Ejecuta 'Remove-NetEventSession -Name *' en una consola PowerShell con permisos elevados."}

    $sn = "SwitchPortDiscovery-{0}" -f ([guid]::NewGuid().ToString('N').Substring(0,8))
    $etl = Join-Path $env:TEMP ("{0}.etl" -f $sn)
    $started = $false
    try{
        New-NetEventSession -Name $sn -LocalFilePath $etl -CaptureMode SaveToFile -ErrorAction Stop | Out-Null
        Add-NetEventPacketCaptureProvider -SessionName $sn -CaptureType Physical -TruncationLength 0 -LinkLayerAddress @('01-80-C2-00-00-0E','01-80-C2-00-00-03','01-80-C2-00-00-00','01-00-0C-CC-CC-CC') -ErrorAction Stop | Out-Null
        foreach($a in $adapters){ Add-NetEventNetworkAdapter -Name $a.Name -PromiscuousMode $true -ErrorAction Stop | Out-Null }
        Start-NetEventSession -Name $sn -ErrorAction Stop | Out-Null
        $started = $true
        Start-Sleep -Seconds $DurationSeconds
        Stop-NetEventSession -Name $sn -ErrorAction Stop | Out-Null
        $started = $false

        $events = @(Get-WinEvent -Path $etl -Oldest -FilterXPath "*[System[EventID=1001]]" -ErrorAction SilentlyContinue)
        $xq = [string[]]@("Event/EventData/Data[@Name='FragmentSize']","Event/EventData/Data[@Name='Fragment']","Event/EventData/Data[@Name='MiniportIfIndex']")
        $sel = [Diagnostics.Eventing.Reader.EventLogPropertySelector]::new($xq)
        $res = New-Object Collections.Generic.List[object]

        foreach($ev in $events){
            $v = $ev.GetPropertyValues($sel)
            if($v.Count -lt 3){continue}
            $frag = [byte[]]$v[1]; $ifx = [int]$v[2]
            $e = Envelope $frag
            if(-not $e){continue}
            $p = switch($e.Proto){ 'LLDP'{ParseLldp $frag}; 'CDP'{ParseCdp $frag} }
            if(-not $p){continue}
            $a = Get-NetAdapter -InterfaceIndex $ifx -ErrorAction SilentlyContinue
            $res.Add([pscustomobject][ordered]@{
                Computer = $env:COMPUTERNAME
                Adapter = if($a){$a.Name}else{"IfIndex $ifx"}
                AdapterMAC = if($a){$a.MacAddress}else{"Desconocida"}
                Protocol = $p.Protocol
                SwitchName = $p.SwitchName
                SwitchIP = $p.SwitchIP
                Port = $p.Port
                PortDescription = $p.PortDescription
                VLAN = $p.VLAN
                Model = $p.Model
                CapturedAt = $ev.TimeCreated
            })
        }
        $res | Sort-Object Adapter,Protocol,SwitchName,Port -Unique
    } finally {
        if($started){ Stop-NetEventSession -Name $sn -ErrorAction SilentlyContinue | Out-Null }
        Remove-NetEventSession -Name $sn -Confirm:$false -ErrorAction SilentlyContinue | Out-Null
        if(Test-Path -LiteralPath $etl){ Remove-Item -LiteralPath $etl -Force -ErrorAction SilentlyContinue }
    }
}

Invoke-SwitchPortDiscovery -AdapterName "__ADAPTER__" -DurationSeconds 62 -Protocol Both | ConvertTo-Json -Depth 5
"""


# ============================================================
# DESCUBRIMIENTO
# ============================================================
def run_discovery_etw(adapter_name: str, timeout: int = 180):
    """
    Ejecuta la captura ETW vía PowerShell.
    Devuelve (data, error). data es list[dict] o None.
    """
    ps_script = _PS_SCRIPT_TEMPLATE.replace("__ADAPTER__", adapter_name)

    try:
        proc = subprocess.Popen(
            ["powershell", "-ExecutionPolicy", "Bypass", "-NoProfile",
             "-NonInteractive", "-Command", "-"],
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            text=True, creationflags=subprocess.CREATE_NO_WINDOW,
        )
    except Exception as e:
        return None, f"Error al invocar PowerShell: {e}"

    try:
        stdout, stderr = proc.communicate(input=ps_script, timeout=timeout)
    except subprocess.TimeoutExpired:
        try:
            proc.kill()
        except Exception:
            pass
        return None, f"Timeout: ETW no respondio en {timeout}s."

    if proc.returncode != 0 or "Requiere privilegios" in stderr or "no encontrado" in stderr:
        msg = stderr.strip() or f"PowerShell termino con codigo {proc.returncode}"
        return None, f"Fallo ETW:\n{msg}"

    if not stdout.strip():
        return [], None

    try:
        raw = json.loads(stdout)
        if isinstance(raw, dict):
            raw = [raw]
        return raw, None
    except json.JSONDecodeError:
        return None, f"Fallo parseo JSON:\n{stdout.strip()[:500]}"


def run_discovery(adapter_name: str):
    """API pública. Bloqueante. Devuelve (data, error)."""
    if not IS_WINDOWS:
        return None, "Este escaneo requiere Windows (ETW)."
    return run_discovery_etw(adapter_name)