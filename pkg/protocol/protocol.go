package protocol

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
)

type DiscoveryResult struct {
	Protocol          string `json:"protocol"`
	SwitchName        string `json:"switch_name"`
	SwitchIP          string `json:"switch_ip"`
	Port              string `json:"port"`
	PortDescription   string `json:"port_description"`
	VLAN              int    `json:"vlan"`
	Model             string `json:"model"`
	ChassisID         string `json:"chassis_id"`
	SystemDescription string `json:"system_description"`
	SourceMAC         string `json:"source_mac"`
}

type PacketEnvelope struct {
	Protocol  string
	DstMAC    string
	SrcMAC    string
	Offset    int
	VLAN      int
}

func formatMAC(b []byte) string {
	if len(b) < 6 {
		return ""
	}
	return fmt.Sprintf("%02X-%02X-%02X-%02X-%02X-%02X", b[0], b[1], b[2], b[3], b[4], b[5])
}

func sanitizeASCII(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		if c >= 32 && c <= 126 {
			sb.WriteByte(c)
		} else if c == '\n' || c == '\r' || c == '\t' {
			sb.WriteByte(' ')
		}
	}
	return strings.TrimSpace(sb.String())
}

func ParseEnvelope(data []byte) *PacketEnvelope {
	if len(data) < 14 {
		return nil
	}

	dst := formatMAC(data[0:6])
	src := formatMAC(data[6:12])
	etherType := binary.BigEndian.Uint16(data[12:14])
	offset := 14
	vlan := 0

	if (etherType == 0x8100 || etherType == 0x88A8) && len(data) >= 18 {
		vlan = int(binary.BigEndian.Uint16(data[14:16]) & 0x0FFF)
		etherType = binary.BigEndian.Uint16(data[16:18])
		offset = 18
	}

	if etherType == 0x88CC {
		return &PacketEnvelope{
			Protocol: "LLDP",
			DstMAC:   dst,
			SrcMAC:   src,
			Offset:   offset,
			VLAN:     vlan,
		}
	}

	if dst == "01-00-0C-CC-CC-CC" && len(data) >= offset+8 {
		snap := data[offset : offset+8]
		expectedSNAP := []byte{0xAA, 0xAA, 0x03, 0x00, 0x00, 0x0C, 0x20, 0x00}
		match := true
		for i := 0; i < 8; i++ {
			if snap[i] != expectedSNAP[i] {
				match = false
				break
			}
		}
		if match {
			return &PacketEnvelope{
				Protocol: "CDP",
				DstMAC:   dst,
				SrcMAC:   src,
				Offset:   offset + 8,
				VLAN:     vlan,
			}
		}
	}

	return nil
}

func ParseLLDP(data []byte) *DiscoveryResult {
	env := ParseEnvelope(data)
	if env == nil || env.Protocol != "LLDP" {
		return nil
	}

	res := &DiscoveryResult{
		Protocol:  "LLDP",
		SourceMAC: env.SrcMAC,
		VLAN:      env.VLAN,
	}

	offset := env.Offset
	var mgmtAddrs []string

	for offset+2 <= len(data) {
		hdr := binary.BigEndian.Uint16(data[offset : offset+2])
		tlvType := int(hdr >> 9)
		tlvLen := int(hdr & 0x01FF)
		offset += 2

		if tlvType == 0 || offset+tlvLen > len(data) {
			break
		}

		body := data[offset : offset+tlvLen]
		offset += tlvLen

		switch tlvType {
		case 1:
			if len(body) >= 2 {
				if body[0] == 4 && len(body) >= 7 {
					res.ChassisID = formatMAC(body[1:7])
				} else {
					res.ChassisID = sanitizeASCII(body[1:])
				}
			}
		case 2:
			if len(body) >= 2 {
				if body[0] == 3 && len(body) >= 7 {
					res.Port = formatMAC(body[1:7])
				} else {
					res.Port = sanitizeASCII(body[1:])
				}
			}
		case 4:
			res.PortDescription = sanitizeASCII(body)
		case 5:
			res.SwitchName = sanitizeASCII(body)
		case 6:
			res.SystemDescription = sanitizeASCII(body)
		case 8:
			if len(body) >= 3 {
				addrLen := int(body[0])
				addrSubtype := int(body[1])
				if addrLen >= 2 && 2+addrLen-1 <= len(body) {
					addrBytes := body[2 : 2+addrLen-1]
					if addrSubtype == 1 && len(addrBytes) == 4 {
						mgmtAddrs = append(mgmtAddrs, net.IP(addrBytes).String())
					} else if addrSubtype == 2 && len(addrBytes) == 16 {
						mgmtAddrs = append(mgmtAddrs, net.IP(addrBytes).String())
					}
				}
			}
		case 127:
			if len(body) >= 4 {
				oui := fmt.Sprintf("%02X-%02X-%02X", body[0], body[1], body[2])
				subtype := body[3]
				if oui == "00-80-C2" && subtype == 1 && len(body) >= 6 {
					res.VLAN = int(binary.BigEndian.Uint16(body[4:6]))
				} else if oui == "00-12-BB" && subtype == 10 && len(body) > 4 {
					res.Model = sanitizeASCII(body[4:])
				}
			}
		}
	}

	for _, addr := range mgmtAddrs {
		if strings.Contains(addr, ".") {
			res.SwitchIP = addr
			break
		}
	}
	if res.SwitchIP == "" && len(mgmtAddrs) > 0 {
		res.SwitchIP = mgmtAddrs[0]
	}

	return res
}

func ParseCDP(data []byte) *DiscoveryResult {
	env := ParseEnvelope(data)
	if env == nil || env.Protocol != "CDP" {
		return nil
	}

	offset := env.Offset
	if offset+4 > len(data) {
		return nil
	}

	res := &DiscoveryResult{
		Protocol:  "CDP",
		SourceMAC: env.SrcMAC,
		ChassisID: env.SrcMAC,
		VLAN:      env.VLAN,
	}

	offset += 4
	var addrs []string

	for offset+4 <= len(data) {
		tlvType := binary.BigEndian.Uint16(data[offset : offset+2])
		tlvLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))

		if tlvLen < 4 || offset+tlvLen > len(data) {
			break
		}

		body := data[offset+4 : offset+tlvLen]
		offset += tlvLen

		switch tlvType {
		case 0x0001:
			res.SwitchName = sanitizeASCII(body)
		case 0x0002, 0x0016:
			parsed := parseCDPAddresses(body)
			addrs = append(addrs, parsed...)
		case 0x0003:
			res.Port = sanitizeASCII(body)
		case 0x0005:
			res.SystemDescription = sanitizeASCII(body)
		case 0x0006:
			res.Model = sanitizeASCII(body)
		case 0x000A:
			if len(body) >= 2 {
				res.VLAN = int(binary.BigEndian.Uint16(body[0:2]))
			}
		}
	}

	for _, addr := range addrs {
		if strings.Contains(addr, ".") {
			res.SwitchIP = addr
			break
		}
	}
	if res.SwitchIP == "" && len(addrs) > 0 {
		res.SwitchIP = addrs[0]
	}

	return res
}

func parseCDPAddresses(data []byte) []string {
	var result []string
	if len(data) < 4 {
		return result
	}

	count := int(binary.BigEndian.Uint32(data[0:4]))
	offset := 4

	for i := 0; i < count; i++ {
		if offset+2 > len(data) {
			break
		}
		protoLen := int(data[offset+1])
		offset += 2

		if offset+protoLen > len(data) {
			break
		}
		offset += protoLen

		if offset+2 > len(data) {
			break
		}
		addrLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2

		if addrLen <= 0 || offset+addrLen > len(data) {
			break
		}
		addrBytes := data[offset : offset+addrLen]
		offset += addrLen

		if addrLen == 4 || addrLen == 16 {
			result = append(result, net.IP(addrBytes).String())
		}
	}

	return result
}
