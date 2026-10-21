package protocol

import (
	"encoding/binary"
	"testing"
)

func TestParseEnvelopeLLDP(t *testing.T) {
	packet := make([]byte, 60)
	copy(packet[0:6], []byte{0x01, 0x80, 0xC2, 0x00, 0x00, 0x0E})
	copy(packet[6:12], []byte{0x00, 0x1A, 0x2B, 0x3C, 0x4D, 0x5E})
	binary.BigEndian.PutUint16(packet[12:14], 0x88CC)

	env := ParseEnvelope(packet)
	if env == nil {
		t.Fatal("expected non-nil envelope")
	}
	if env.Protocol != "LLDP" {
		t.Errorf("expected LLDP, got %s", env.Protocol)
	}
	if env.SrcMAC != "00-1A-2B-3C-4D-5E" {
		t.Errorf("unexpected SrcMAC: %s", env.SrcMAC)
	}
	if env.Offset != 14 {
		t.Errorf("expected offset 14, got %d", env.Offset)
	}
}

func TestParseEnvelopeVlanLLDP(t *testing.T) {
	packet := make([]byte, 60)
	copy(packet[0:6], []byte{0x01, 0x80, 0xC2, 0x00, 0x00, 0x0E})
	copy(packet[6:12], []byte{0x00, 0x1A, 0x2B, 0x3C, 0x4D, 0x5E})
	binary.BigEndian.PutUint16(packet[12:14], 0x8100)
	binary.BigEndian.PutUint16(packet[14:16], 120)
	binary.BigEndian.PutUint16(packet[16:18], 0x88CC)

	env := ParseEnvelope(packet)
	if env == nil {
		t.Fatal("expected non-nil envelope")
	}
	if env.VLAN != 120 {
		t.Errorf("expected VLAN 120, got %d", env.VLAN)
	}
	if env.Offset != 18 {
		t.Errorf("expected offset 18, got %d", env.Offset)
	}
}

func TestParseLLDP(t *testing.T) {
	packet := make([]byte, 100)
	copy(packet[0:6], []byte{0x01, 0x80, 0xC2, 0x00, 0x00, 0x0E})
	copy(packet[6:12], []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55})
	binary.BigEndian.PutUint16(packet[12:14], 0x88CC)

	offset := 14
	putTLV := func(tlvType int, body []byte) {
		hdr := uint16(tlvType<<9) | uint16(len(body)&0x01FF)
		binary.BigEndian.PutUint16(packet[offset:offset+2], hdr)
		offset += 2
		copy(packet[offset:], body)
		offset += len(body)
	}

	putTLV(1, append([]byte{4}, []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}...))
	putTLV(2, append([]byte{5}, []byte("Gi1/0/1")...))
	putTLV(4, []byte("Access Port"))
	putTLV(5, []byte("SW-CORE-01"))
	putTLV(6, []byte("Cisco IOS 15.2"))

	res := ParseLLDP(packet[:offset])
	if res == nil {
		t.Fatal("expected non-nil LLDP result")
	}
	if res.SwitchName != "SW-CORE-01" {
		t.Errorf("expected SwitchName SW-CORE-01, got %s", res.SwitchName)
	}
	if res.Port != "Gi1/0/1" {
		t.Errorf("expected Port Gi1/0/1, got %s", res.Port)
	}
	if res.PortDescription != "Access Port" {
		t.Errorf("expected PortDescription Access Port, got %s", res.PortDescription)
	}
}

func TestParseCDP(t *testing.T) {
	packet := make([]byte, 120)
	copy(packet[0:6], []byte{0x01, 0x00, 0x0C, 0xCC, 0xCC, 0xCC})
	copy(packet[6:12], []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF})
	binary.BigEndian.PutUint16(packet[12:14], 100)
	copy(packet[14:22], []byte{0xAA, 0xAA, 0x03, 0x00, 0x00, 0x0C, 0x20, 0x00})

	offset := 22
	packet[offset] = 0x02
	packet[offset+1] = 180
	binary.BigEndian.PutUint16(packet[offset+2:offset+4], 0x0000)
	offset += 4

	putCDPTLV := func(tlvType uint16, body []byte) {
		binary.BigEndian.PutUint16(packet[offset:offset+2], tlvType)
		binary.BigEndian.PutUint16(packet[offset+2:offset+4], uint16(len(body)+4))
		offset += 4
		copy(packet[offset:], body)
		offset += len(body)
	}

	putCDPTLV(0x0001, []byte("CISCO-DIST-01"))
	putCDPTLV(0x0003, []byte("FastEthernet0/24"))
	putCDPTLV(0x0006, []byte("WS-C2960X-48FPS-L"))

	vlanBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(vlanBytes, 40)
	putCDPTLV(0x000A, vlanBytes)

	res := ParseCDP(packet[:offset])
	if res == nil {
		t.Fatal("expected non-nil CDP result")
	}
	if res.SwitchName != "CISCO-DIST-01" {
		t.Errorf("expected SwitchName CISCO-DIST-01, got %s", res.SwitchName)
	}
	if res.Port != "FastEthernet0/24" {
		t.Errorf("expected Port FastEthernet0/24, got %s", res.Port)
	}
	if res.Model != "WS-C2960X-48FPS-L" {
		t.Errorf("expected Model WS-C2960X-48FPS-L, got %s", res.Model)
	}
	if res.VLAN != 40 {
		t.Errorf("expected VLAN 40, got %d", res.VLAN)
	}
}

