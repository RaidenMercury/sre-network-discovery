package link

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

var BlacklistAdapters = []string{
	"loopback", "vethernet", "hyper-v", "wsl",
	"vmware", "virtualbox", "docker", "br-", "veth",
	"tailscale", "zerotier", "wireguard", "openvpn",
	"nordvpn", "protonvpn", "tap-", "tun",
}

type AdapterInfo struct {
	Index        int      `json:"index"`
	Name         string   `json:"name"`
	MAC          string   `json:"mac"`
	IsUp         bool     `json:"is_up"`
	IPv4         []string `json:"ipv4"`
	IPv6         []string `json:"ipv6"`
	Flags        string   `json:"flags"`
}

type LinkValidation struct {
	AdapterName     string   `json:"adapter_name"`
	MACAddress      string   `json:"mac_address"`
	IsUp            bool     `json:"is_up"`
	IPv4Addresses   []string `json:"ipv4_addresses"`
	IPv6Addresses   []string `json:"ipv6_addresses"`
	Gateway         string   `json:"gateway"`
	GatewayAlive    bool     `json:"gateway_alive"`
	DNSResolving    bool     `json:"dns_resolving"`
	SecureChannelOK bool     `json:"secure_channel_ok"`
	ReadyForExtract bool     `json:"ready_for_extract"`
	StatusSummary   string   `json:"status_summary"`
}

func IsBlacklisted(name string) bool {
	lower := strings.ToLower(name)
	for _, b := range BlacklistAdapters {
		if strings.Contains(lower, b) {
			return true
		}
	}
	return false
}

func GetEligibleAdapters() ([]AdapterInfo, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var results []AdapterInfo
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if IsBlacklisted(iface.Name) {
			continue
		}

		info := AdapterInfo{
			Index: iface.Index,
			Name:  iface.Name,
			MAC:   iface.HardwareAddr.String(),
			IsUp:  iface.Flags&net.FlagUp != 0,
			Flags: iface.Flags.String(),
		}

		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ip.To4() != nil {
				info.IPv4 = append(info.IPv4, ip.String())
			} else {
				info.IPv6 = append(info.IPv6, ip.String())
			}
		}

		results = append(results, info)
	}
	return results, nil
}

func ValidateLink(adapterName string, checkSecureEndpoint string) *LinkValidation {
	res := &LinkValidation{
		AdapterName: adapterName,
	}

	iface, err := net.InterfaceByName(adapterName)
	if err != nil {
		res.StatusSummary = fmt.Sprintf("Adaptador '%s' no encontrado: %v", adapterName, err)
		return res
	}

	res.MACAddress = iface.HardwareAddr.String()
	res.IsUp = iface.Flags&net.FlagUp != 0

	if !res.IsUp {
		res.StatusSummary = "Enlace físico desconectado (Link Down / Cable no conectado)"
		return res
	}

	addrs, err := iface.Addrs()
	if err == nil {
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ip.To4() != nil {
				res.IPv4Addresses = append(res.IPv4Addresses, ip.String())
			} else {
				res.IPv6Addresses = append(res.IPv6Addresses, ip.String())
			}
		}
	}

	res.Gateway = detectGateway(iface.Name)
	if res.Gateway != "" {
		res.GatewayAlive = probeGateway(res.Gateway)
	}

	res.DNSResolving = probeDNS()

	if checkSecureEndpoint == "" {
		checkSecureEndpoint = "https://1.1.1.1"
	}
	res.SecureChannelOK = probeSecureChannel(checkSecureEndpoint)

	if res.IsUp && (len(res.IPv4Addresses) > 0 || len(res.IPv6Addresses) > 0) && (res.GatewayAlive || res.DNSResolving || res.SecureChannelOK) {
		res.ReadyForExtract = true
		res.StatusSummary = "Punto de red validado y canal seguro operativo"
	} else if res.IsUp {
		res.ReadyForExtract = true
		res.StatusSummary = "Enlace físico activo (esperando direccionamiento o tráfico L2 CDP/LLDP)"
	} else {
		res.StatusSummary = "Enlace no disponible"
	}

	return res
}

func detectGateway(adapterName string) string {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
			fmt.Sprintf("(Get-NetRoute -InterfaceAlias '%s' -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue | Select-Object -First 1).NextHop", adapterName))
		out, err := cmd.Output()
		if err == nil {
			gw := strings.TrimSpace(string(out))
			if net.ParseIP(gw) != nil {
				return gw
			}
		}
	} else {
		cmd := exec.Command("sh", "-c", "ip route show default | awk '{print $3}' | head -n 1")
		out, err := cmd.Output()
		if err == nil {
			gw := strings.TrimSpace(string(out))
			if net.ParseIP(gw) != nil {
				return gw
			}
		}
	}
	return ""
}

func probeGateway(gateway string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(gateway, "80"), 800*time.Millisecond)
	if err == nil {
		conn.Close()
		return true
	}
	conn, err = net.DialTimeout("tcp", net.JoinHostPort(gateway, "443"), 800*time.Millisecond)
	if err == nil {
		conn.Close()
		return true
	}
	conn, err = net.DialTimeout("tcp", net.JoinHostPort(gateway, "53"), 800*time.Millisecond)
	if err == nil {
		conn.Close()
		return true
	}
	conn, err = net.DialTimeout("udp", net.JoinHostPort(gateway, "53"), 800*time.Millisecond)
	if err == nil {
		conn.Close()
		return true
	}
	return false
}

func probeDNS() bool {
	resolver := &net.Resolver{
		PreferGo: true,
	}
	_, err := resolver.LookupHost(nil, "cloudflare.com")
	return err == nil
}

func probeSecureChannel(endpoint string) bool {
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		},
	}
	resp, err := client.Head(endpoint)
	if err == nil {
		resp.Body.Close()
		return true
	}
	resp, err = client.Get(endpoint)
	if err == nil {
		resp.Body.Close()
		return true
	}
	return false
}
