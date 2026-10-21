package report

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/RaidenMercury/sre-network-discovery/pkg/protocol"
)

type DeviceEntry struct {
	ID                string    `json:"id"`
	SwitchName        string    `json:"switch_name"`
	SwitchIP          string    `json:"switch_ip"`
	Port              string    `json:"port"`
	PortDescription   string    `json:"port_description"`
	VLAN              int       `json:"vlan"`
	Model             string    `json:"model"`
	Protocol          string    `json:"protocol"`
	ChassisID         string    `json:"chassis_id"`
	SystemDescription string    `json:"system_description"`
	LocalAdapter      string    `json:"local_adapter"`
	LocalMAC          string    `json:"local_mac"`
	FirstSeen         time.Time `json:"first_seen"`
	LastSeen          time.Time `json:"last_seen"`
	TimesSeen         int       `json:"times_seen"`
}

type Inventory struct {
	GeneratedAt time.Time     `json:"generated_at"`
	TotalScans  int           `json:"total_scans"`
	Devices     []DeviceEntry `json:"devices"`
}

func NewInventory() *Inventory {
	return &Inventory{
		GeneratedAt: time.Now(),
		Devices:     make([]DeviceEntry, 0),
	}
}

func deviceKey(d *protocol.DiscoveryResult) string {
	name := strings.TrimSpace(strings.ToUpper(d.SwitchName))
	port := strings.TrimSpace(strings.ToLower(d.Port))
	if name != "" && port != "" {
		return fmt.Sprintf("%s/%s", name, port)
	}
	chassis := strings.TrimSpace(strings.ToUpper(d.ChassisID))
	if chassis != "" && port != "" {
		return fmt.Sprintf("%s/%s", chassis, port)
	}
	if d.SwitchIP != "" {
		return fmt.Sprintf("%s/%s", d.SwitchIP, port)
	}
	return fmt.Sprintf("%s/%s", d.SourceMAC, port)
}

func (inv *Inventory) MergeDevice(d protocol.DiscoveryResult, adapter, mac string) {
	key := deviceKey(&d)
	now := time.Now()

	for i := range inv.Devices {
		if inv.Devices[i].ID == key {
			inv.Devices[i].LastSeen = now
			inv.Devices[i].TimesSeen++
			if d.SwitchIP != "" {
				inv.Devices[i].SwitchIP = d.SwitchIP
			}
			if d.VLAN != 0 {
				inv.Devices[i].VLAN = d.VLAN
			}
			if d.Model != "" {
				inv.Devices[i].Model = d.Model
			}
			if d.PortDescription != "" {
				inv.Devices[i].PortDescription = d.PortDescription
			}
			if d.SystemDescription != "" {
				inv.Devices[i].SystemDescription = d.SystemDescription
			}
			inv.Devices[i].LocalAdapter = adapter
			inv.Devices[i].LocalMAC = mac
			return
		}
	}

	inv.Devices = append(inv.Devices, DeviceEntry{
		ID:                key,
		SwitchName:        d.SwitchName,
		SwitchIP:          d.SwitchIP,
		Port:              d.Port,
		PortDescription:   d.PortDescription,
		VLAN:              d.VLAN,
		Model:             d.Model,
		Protocol:          d.Protocol,
		ChassisID:         d.ChassisID,
		SystemDescription: d.SystemDescription,
		LocalAdapter:      adapter,
		LocalMAC:          mac,
		FirstSeen:         now,
		LastSeen:          now,
		TimesSeen:         1,
	})
}

func (inv *Inventory) MergeResults(results []protocol.DiscoveryResult, adapter, mac string) {
	inv.TotalScans++
	inv.GeneratedAt = time.Now()
	for _, res := range results {
		inv.MergeDevice(res, adapter, mac)
	}
	sort.Slice(inv.Devices, func(i, j int) bool {
		if inv.Devices[i].SwitchName == inv.Devices[j].SwitchName {
			return inv.Devices[i].Port < inv.Devices[j].Port
		}
		return inv.Devices[i].SwitchName < inv.Devices[j].SwitchName
	})
}

func LoadInventory(path string) (*Inventory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewInventory(), nil
		}
		return nil, err
	}
	var inv Inventory
	if err := json.Unmarshal(data, &inv); err != nil {
		return nil, err
	}
	return &inv, nil
}

func (inv *Inventory) SaveJSON(path string) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}
	data, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (inv *Inventory) SaveMarkdown(path string) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	var sb strings.Builder
	sb.WriteString("# SRE Network Discovery - Cumulative Inventory Report\n\n")
	sb.WriteString(fmt.Sprintf("**Fecha del reporte:** %s  \n", inv.GeneratedAt.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**Total escaneos acumulados:** %d  \n", inv.TotalScans))
	sb.WriteString(fmt.Sprintf("**Dispositivos únicos detectados:** %d  \n\n", len(inv.Devices)))

	sb.WriteString("| Switch Name | IP Gestión | Puerto Switch | VLAN | Modelo | Protocolo | Adaptador Local | Detecciones | Última Vez |\n")
	sb.WriteString("|:---|:---|:---|:---|:---|:---|:---|:---|:---|\n")

	for _, d := range inv.Devices {
		vlanStr := "-"
		if d.VLAN != 0 {
			vlanStr = strconv.Itoa(d.VLAN)
		}
		ipStr := d.SwitchIP
		if ipStr == "" {
			ipStr = "-"
		}
		modelStr := d.Model
		if modelStr == "" {
			modelStr = "-"
		}
		sb.WriteString(fmt.Sprintf("| **%s** | `%s` | `%s` | %s | %s | %s | %s | %d | %s |\n",
			d.SwitchName, ipStr, d.Port, vlanStr, modelStr, d.Protocol, d.LocalAdapter, d.TimesSeen, d.LastSeen.Format("15:04:05")))
	}

	return os.WriteFile(path, []byte(sb.String()), 0644)
}

func (inv *Inventory) SaveCSV(path string) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	w := csv.NewWriter(file)
	defer w.Flush()

	header := []string{
		"SwitchName", "SwitchIP", "Port", "PortDescription", "VLAN",
		"Model", "Protocol", "ChassisID", "LocalAdapter", "LocalMAC",
		"TimesSeen", "FirstSeen", "LastSeen",
	}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, d := range inv.Devices {
		row := []string{
			d.SwitchName,
			d.SwitchIP,
			d.Port,
			d.PortDescription,
			strconv.Itoa(d.VLAN),
			d.Model,
			d.Protocol,
			d.ChassisID,
			d.LocalAdapter,
			d.LocalMAC,
			strconv.Itoa(d.TimesSeen),
			d.FirstSeen.Format(time.RFC3339),
			d.LastSeen.Format(time.RFC3339),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}
