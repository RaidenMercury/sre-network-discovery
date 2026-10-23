package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RaidenMercury/sre-network-discovery/pkg/protocol"
)

func TestInventoryMerge(t *testing.T) {
	inv := NewInventory()

	res1 := []protocol.DiscoveryResult{
		{
			Protocol:   "LLDP",
			SwitchName: "SW-RACK-01",
			SwitchIP:   "192.168.1.10",
			Port:       "GigabitEthernet1/0/1",
			VLAN:       10,
			Model:      "C9300-48P",
		},
	}

	inv.MergeResults(res1, "Ethernet", "AA-BB-CC-DD-EE-FF")
	if len(inv.Devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(inv.Devices))
	}
	if inv.Devices[0].TimesSeen != 1 {
		t.Errorf("expected TimesSeen 1, got %d", inv.Devices[0].TimesSeen)
	}

	// Second scan of same switch and port (cumulative merge)
	res2 := []protocol.DiscoveryResult{
		{
			Protocol:   "LLDP",
			SwitchName: "SW-RACK-01",
			SwitchIP:   "192.168.1.10",
			Port:       "GigabitEthernet1/0/1",
			VLAN:       20, // VLAN changed
		},
		{
			Protocol:   "CDP",
			SwitchName: "SW-RACK-02",
			SwitchIP:   "192.168.1.20",
			Port:       "FastEthernet0/1",
			VLAN:       1,
		},
	}

	inv.MergeResults(res2, "Ethernet", "AA-BB-CC-DD-EE-FF")
	if len(inv.Devices) != 2 {
		t.Fatalf("expected 2 devices after second scan, got %d", len(inv.Devices))
	}
	if inv.TotalScans != 2 {
		t.Errorf("expected 2 total scans, got %d", inv.TotalScans)
	}

	for _, d := range inv.Devices {
		if d.SwitchName == "SW-RACK-01" {
			if d.TimesSeen != 2 {
				t.Errorf("expected SW-RACK-01 TimesSeen 2, got %d", d.TimesSeen)
			}
			if d.VLAN != 20 {
				t.Errorf("expected updated VLAN 20, got %d", d.VLAN)
			}
		}
	}
}

func TestInventorySaveAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sre_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	jsonPath := filepath.Join(tmpDir, "inventory.json")
	mdPath := filepath.Join(tmpDir, "inventory.md")
	csvPath := filepath.Join(tmpDir, "inventory.csv")

	inv := NewInventory()
	inv.MergeResults([]protocol.DiscoveryResult{
		{
			Protocol:   "CDP",
			SwitchName: "CORE-01",
			SwitchIP:   "10.0.0.1",
			Port:       "Gi0/1",
			VLAN:       100,
		},
	}, "Eth0", "11-22-33-44-55-66")

	if err := inv.SaveJSON(jsonPath); err != nil {
		t.Fatalf("failed to save JSON: %v", err)
	}
	if err := inv.SaveMarkdown(mdPath); err != nil {
		t.Fatalf("failed to save Markdown: %v", err)
	}
	mdBytes, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("failed to read generated Markdown: %v", err)
	}
	mdContent := string(mdBytes)
	if !strings.Contains(mdContent, "Punto de red n°1:") {
		t.Errorf("expected 'Punto de red n°1:' in markdown, got:\n%s", mdContent)
	}
	if !strings.Contains(mdContent, "- Switch Name: CORE-01") {
		t.Errorf("expected '- Switch Name: CORE-01' in markdown, got:\n%s", mdContent)
	}
	if !strings.Contains(mdContent, "- Adaptador local: Eth0") {
		t.Errorf("expected '- Adaptador local: Eth0' in markdown, got:\n%s", mdContent)
	}
	if err := inv.SaveCSV(csvPath); err != nil {
		t.Fatalf("failed to save CSV: %v", err)
	}

	loaded, err := LoadInventory(jsonPath)
	if err != nil {
		t.Fatalf("failed to load JSON: %v", err)
	}
	if len(loaded.Devices) != 1 {
		t.Fatalf("expected 1 loaded device, got %d", len(loaded.Devices))
	}
	if loaded.Devices[0].SwitchName != "CORE-01" {
		t.Errorf("expected CORE-01, got %s", loaded.Devices[0].SwitchName)
	}
}

func TestInventoryMergeCDPAndLLDP(t *testing.T) {
	inv := NewInventory()

	// 1. CDP arrives first with full Cisco info
	cdp := protocol.DiscoveryResult{
		Protocol:   "CDP",
		SwitchName: "SW-PISO2-DIST01",
		SwitchIP:   "10.20.30.1",
		Port:       "GigabitEthernet1/0/18",
		Model:      "WS-C2960X-48FPS-L",
		SourceMAC:  "00-11-22-33-44-55",
	}

	// 2. LLDP arrives on same port with VLAN info and Gi1/0/18
	lldp := protocol.DiscoveryResult{
		Protocol:   "LLDP",
		SwitchName: "SW-PISO2-DIST01",
		Port:       "Gi1/0/18",
		VLAN:       40,
		ChassisID:  "00-11-22-33-44-55",
		SourceMAC:  "00-11-22-33-44-55",
	}

	inv.MergeDevice(cdp, "Ethernet", "18-66-DA-34-6B-78")
	inv.MergeDevice(lldp, "Ethernet", "18-66-DA-34-6B-78")

	// Must merge into a SINGLE enriched card, not 2 cards!
	if len(inv.Devices) != 1 {
		t.Fatalf("expected 1 unified device, got %d", len(inv.Devices))
	}

	dev := inv.Devices[0]
	if dev.Protocol != "CDP + LLDP" {
		t.Errorf("expected Protocol 'CDP + LLDP', got '%s'", dev.Protocol)
	}
	if dev.SwitchName != "SW-PISO2-DIST01" {
		t.Errorf("expected SwitchName SW-PISO2-DIST01, got '%s'", dev.SwitchName)
	}
	if dev.Port != "GigabitEthernet1/0/18" {
		t.Errorf("expected Port GigabitEthernet1/0/18, got '%s'", dev.Port)
	}
	if dev.VLAN != 40 {
		t.Errorf("expected VLAN 40, got %d", dev.VLAN)
	}
	if dev.Model != "WS-C2960X-48FPS-L" {
		t.Errorf("expected Model WS-C2960X-48FPS-L, got '%s'", dev.Model)
	}
	if dev.SwitchIP != "10.20.30.1" {
		t.Errorf("expected SwitchIP 10.20.30.1, got '%s'", dev.SwitchIP)
	}
	if dev.TimesSeen != 2 {
		t.Errorf("expected TimesSeen 2, got %d", dev.TimesSeen)
	}
}


