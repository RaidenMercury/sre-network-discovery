//go:build windows

package link

import (
	"net"
	"syscall"
	"unsafe"
)

type mibIPForwardRow struct {
	ForwardDest      uint32
	ForwardMask      uint32
	ForwardPolicy    uint32
	ForwardNextHop   uint32
	ForwardIfIndex   uint32
	ForwardType      uint32
	ForwardProto     uint32
	ForwardAge       uint32
	ForwardNextHopAS uint32
	ForwardMetric1   uint32
	ForwardMetric2   uint32
	ForwardMetric3   uint32
	ForwardMetric4   uint32
	ForwardMetric5   uint32
}

// detectGateway consulta la tabla de enrutamiento nativa del kernel de Windows mediante GetBestRoute (iphlpapi.dll).
// CERO ejecuciones de procesos powershell.exe, CERO ventanas emergentes y CERO alertas en EDR/SIEM.
func detectGateway(adapterName string) string {
	mod := syscall.NewLazyDLL("iphlpapi.dll")
	proc := mod.NewProc("GetBestRoute")
	if proc.Find() != nil {
		return ""
	}

	var row mibIPForwardRow
	// Dirección destino de sondeo (8.8.8.8) para resolver la mejor ruta de salida y su NextHop
	dest := uint32(0x08080808)
	r1, _, _ := proc.Call(uintptr(dest), 0, uintptr(unsafe.Pointer(&row)))
	if r1 == 0 && row.ForwardNextHop != 0 {
		ip := net.IPv4(
			byte(row.ForwardNextHop),
			byte(row.ForwardNextHop>>8),
			byte(row.ForwardNextHop>>16),
			byte(row.ForwardNextHop>>24),
		)
		if ip != nil && !ip.IsUnspecified() && !ip.IsLoopback() {
			return ip.String()
		}
	}
	return ""
}

