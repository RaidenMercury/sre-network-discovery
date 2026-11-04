//go:build !windows

package link

import (
	"net"
	"os/exec"
	"strings"
)

func detectGateway(adapterName string) string {
	cmd := exec.Command("sh", "-c", "ip route show default | awk '{print $3}' | head -n 1")
	out, err := cmd.Output()
	if err == nil {
		gw := strings.TrimSpace(string(out))
		if net.ParseIP(gw) != nil {
			return gw
		}
	}
	return ""
}

