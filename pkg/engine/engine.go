package engine

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/RaidenMercury/sre-network-discovery/pkg/capture"
	"github.com/RaidenMercury/sre-network-discovery/pkg/link"
	"github.com/RaidenMercury/sre-network-discovery/pkg/protocol"
	"github.com/RaidenMercury/sre-network-discovery/pkg/report"
)

type Config struct {
	AdapterName       string
	Duration          time.Duration
	ValidateLinkFirst bool
	AutoExtract       bool
	AppendMode        bool
	OutputPath        string
	ExportMarkdown    bool
	ExportCSV         bool
	RemoteURL         string
	Tag               string
	OnStatus          func(msg string)
}

type Runner struct {
	captureEngine capture.PacketCapture
}

func NewRunner() *Runner {
	return &Runner{
		captureEngine: capture.NewCaptureEngine(),
	}
}

func (r *Runner) log(cfg *Config, format string, a ...interface{}) {
	if cfg.OnStatus != nil {
		cfg.OnStatus(fmt.Sprintf(format, a...))
	}
}

func canonicalMAC(s string) string {
	s = strings.ToUpper(s)
	s = strings.ReplaceAll(s, ":", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, ".", "")
	return strings.TrimSpace(s)
}

func (r *Runner) selectAdapter(cfg *Config) (link.AdapterInfo, error) {
	adapters, err := link.GetEligibleAdapters()
	if err != nil {
		return link.AdapterInfo{}, err
	}
	if len(adapters) == 0 {
		return link.AdapterInfo{}, fmt.Errorf("no se detectaron adaptadores de red fisicos activos")
	}

	if cfg.AdapterName != "" {
		for _, a := range adapters {
			if strings.EqualFold(a.Name, cfg.AdapterName) {
				return a, nil
			}
		}
		return link.AdapterInfo{}, fmt.Errorf("adaptador especificado '%s' no encontrado", cfg.AdapterName)
	}

	for _, a := range adapters {
		if a.IsUp && (len(a.IPv4) > 0 || len(a.IPv6) > 0) {
			return a, nil
		}
	}
	for _, a := range adapters {
		if a.IsUp {
			return a, nil
		}
	}

	return adapters[0], nil
}

func (r *Runner) RunOnce(ctx context.Context, cfg Config) (*report.Inventory, error) {
	selectedAdapter, err := r.selectAdapter(&cfg)
	if err != nil {
		return nil, err
	}

	r.log(&cfg, "[+] Adaptador seleccionado: %s (MAC: %s, Up: %v)", selectedAdapter.Name, selectedAdapter.MAC, selectedAdapter.IsUp)

	if cfg.ValidateLinkFirst {
		r.log(&cfg, "[*] Validando punto de conexion de red...")
		val := link.ValidateLink(selectedAdapter.Name, cfg.RemoteURL)
		r.log(&cfg, "    - Estado de enlace: %s", val.StatusSummary)
		if len(val.IPv4Addresses) > 0 {
			r.log(&cfg, "    - Direcciones IPv4: %s", strings.Join(val.IPv4Addresses, ", "))
		}
		if val.Gateway != "" {
			r.log(&cfg, "    - Puerta de enlace: %s (Alcanzable: %v)", val.Gateway, val.GatewayAlive)
		}
		r.log(&cfg, "    - Resolucion DNS: %v | Canal Seguro (TLS): %v", val.DNSResolving, val.SecureChannelOK)

		if !val.IsUp {
			// Margen de espera por si el enlace físico está negociando auto-MDIX o velocidad Gigabit
			time.Sleep(1500 * time.Millisecond)
			val = link.ValidateLink(selectedAdapter.Name, cfg.RemoteURL)
			if !val.IsUp {
				return nil, fmt.Errorf("el punto de red no tiene enlace fisico activo (cable desconectado)")
			}
		}
	}

	r.log(&cfg, "[*] Iniciando captura de tramas CDP/LLDP por %v...", cfg.Duration)
	results, err := r.captureEngine.Capture(ctx, selectedAdapter.Name, cfg.Duration)
	if err != nil {
		return nil, err
	}

	// 1. Recolectar MACs locales y hostname para descarte de auto-paquetes
	localMACs := make(map[string]bool)
	if m := canonicalMAC(selectedAdapter.MAC); m != "" {
		localMACs[m] = true
	}
	allAdapters, _ := link.GetEligibleAdapters()
	for _, a := range allAdapters {
		if m := canonicalMAC(a.MAC); m != "" {
			localMACs[m] = true
		}
	}
	hostname, _ := os.Hostname()

	// 2. Filtrado riguroso de paquetes reflejados por el propio host (Self-Packet Drop)
	var cleanResults []protocol.DiscoveryResult
	for _, res := range results {
		src := canonicalMAC(res.SourceMAC)
		chassis := canonicalMAC(res.ChassisID)
		if (src != "" && localMACs[src]) || (chassis != "" && localMACs[chassis]) {
			r.log(&cfg, "[i] Filtrado auto-anuncio saliente de la propia laptop (%s / %s)", res.SourceMAC, res.Protocol)
			continue
		}
		if hostname != "" && strings.EqualFold(res.SwitchName, hostname) {
			r.log(&cfg, "[i] Filtrado paquete con hostname local (%s)", res.SwitchName)
			continue
		}
		if strings.HasPrefix(res.SwitchName, "Switch (") && res.Port == "" && res.VLAN == 0 {
			r.log(&cfg, "[i] Filtrado anuncio vacio sin puerto ni VLAN (%s)", res.SourceMAC)
			continue
		}
		cleanResults = append(cleanResults, res)
	}

	r.log(&cfg, "[+] Captura completada. Tramas validas recibidas: %d (auto-filtradas: %d)", len(cleanResults), len(results)-len(cleanResults))

	var inv *report.Inventory
	if cfg.AppendMode {
		inv, err = report.LoadInventory(cfg.OutputPath)
		if err != nil {
			r.log(&cfg, "[!] No se pudo cargar inventario previo (%v), creando uno nuevo", err)
			inv = report.NewInventory()
		} else {
			r.log(&cfg, "[+] Modo acumulativo activo: sumando a reporte previo (%d dispositivos existentes)", len(inv.Devices))
		}
	} else {
		inv = report.NewInventory()
	}

	inv.MergeResults(cleanResults, selectedAdapter.Name, selectedAdapter.MAC)

	if cfg.Tag != "" {
		for i := range inv.Devices {
			if inv.Devices[i].Tag == "" {
				inv.Devices[i].Tag = cfg.Tag
			}
		}
	}

	if err := inv.SaveJSON(cfg.OutputPath); err != nil {
		return nil, fmt.Errorf("error guardando reporte JSON: %w", err)
	}
	r.log(&cfg, "[+] Reporte JSON guardado en: %s", cfg.OutputPath)

	if cfg.ExportMarkdown {
		mdPath := strings.TrimSuffix(cfg.OutputPath, ".json") + ".md"
		if err := inv.SaveMarkdown(mdPath); err == nil {
			r.log(&cfg, "[+] Reporte Markdown guardado en: %s", mdPath)
		}
	}

	if cfg.ExportCSV {
		csvPath := strings.TrimSuffix(cfg.OutputPath, ".json") + ".csv"
		if err := inv.SaveCSV(csvPath); err == nil {
			r.log(&cfg, "[+] Reporte CSV guardado en: %s", csvPath)
		}
	}

	if cfg.RemoteURL != "" {
		r.log(&cfg, "[*] Enviando datos al canal seguro remoto (%s)...", cfg.RemoteURL)
		if err := r.pushSecureRemote(ctx, cfg.RemoteURL, inv); err != nil {
			r.log(&cfg, "[!] Error transmitiendo al canal seguro: %v", err)
		} else {
			r.log(&cfg, "[+] Datos transmitidos exitosamente al canal seguro")
		}
	}

	return inv, nil
}

func (r *Runner) RunAutoExtract(ctx context.Context, cfg Config) error {
	r.log(&cfg, "[*] Modo Extraccion Automatica iniciado")
	r.log(&cfg, "[*] Monitorizando insercion de cable y conexion de red...")

	lastCableConnected := false
	var linkTransitions []time.Time

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.log(&cfg, "[-] Deteniendo monitor de extraccion automatica")
			return nil
		case <-ticker.C:
			adapter, err := r.selectAdapter(&cfg)
			if err != nil {
				continue
			}

			val := link.ValidateLink(adapter.Name, cfg.RemoteURL)

			if val.IsUp != lastCableConnected {
				now := time.Now()
				linkTransitions = append(linkTransitions, now)
				// Filtrar transiciones de los ultimos 10s
				var recent []time.Time
				for _, t := range linkTransitions {
					if now.Sub(t) <= 10*time.Second {
						recent = append(recent, t)
					}
				}
				linkTransitions = recent
				if len(linkTransitions) >= 3 {
					r.log(&cfg, "[!] ALERTA CAPA 1: Inestabilidad de enlace detectada (%d microcortes en 10s). Posible cable/roseta con falla.", len(linkTransitions))
				}
			}

			if val.IsUp && !lastCableConnected {
				r.log(&cfg, "")
				r.log(&cfg, "[!] Cable detectado en adaptador %s. Validando canal...", adapter.Name)
				lastCableConnected = true

				time.Sleep(2 * time.Second)

				_, scanErr := r.RunOnce(ctx, cfg)
				if scanErr != nil {
					r.log(&cfg, "[!] Error durante extraccion automatica: %v", scanErr)
				} else {
					r.log(&cfg, "[+] Extraccion completada con exito para este punto de red")
					r.log(&cfg, "[*] Listo para desconectar y probar el siguiente puerto")
				}
			} else if !val.IsUp && lastCableConnected {
				r.log(&cfg, "[-] Cable desconectado de %s. Esperando siguiente puerto...", adapter.Name)
				lastCableConnected = false
			}
		}
	}
}

func (r *Runner) pushSecureRemote(ctx context.Context, endpoint string, inv *report.Inventory) error {
	data, err := json.Marshal(inv)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "SRE-Network-Discovery/2.1")

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("servidor remoto respondio con HTTP %d", resp.StatusCode)
	}

	return nil
}

