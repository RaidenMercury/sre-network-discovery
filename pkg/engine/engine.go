package engine

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/RaidenMercury/sre-network-discovery/pkg/capture"
	"github.com/RaidenMercury/sre-network-discovery/pkg/link"
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
			return nil, fmt.Errorf("el punto de red no tiene enlace fisico activo (cable desconectado)")
		}
	}

	r.log(&cfg, "[*] Iniciando captura de tramas CDP/LLDP por %v...", cfg.Duration)
	results, err := r.captureEngine.Capture(ctx, selectedAdapter.Name, cfg.Duration)
	if err != nil {
		return nil, err
	}

	r.log(&cfg, "[+] Captura completada. Tramas validas recibidas: %d", len(results))

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

	inv.MergeResults(results, selectedAdapter.Name, selectedAdapter.MAC)

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

	ticker := time.NewTicker(3 * time.Second)
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
	req.Header.Set("User-Agent", "SRE-Network-Discovery/2.0")

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

