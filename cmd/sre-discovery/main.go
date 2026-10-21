package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/RaidenMercury/sre-network-discovery/pkg/engine"
	"github.com/RaidenMercury/sre-network-discovery/pkg/gui"
	"github.com/RaidenMercury/sre-network-discovery/pkg/link"
)

const (
	AppName    = "SRE Network Discovery"
	AppVersion = "2.0.0"
)

func printBanner() {
	fmt.Printf("\n=== %s v%s ===\n", AppName, AppVersion)
	fmt.Println("Descubrimiento de Switches y Validacion de Red (CDP / LLDP)")
	fmt.Println("-----------------------------------------------------------------")
}

func main() {
	guiFlag := flag.Bool("gui", false, "Abrir la interfaz gráfica de usuario interactiva")
	cliFlag := flag.Bool("cli", false, "Forzar modo consola CLI")
	adapterFlag := flag.String("i", "", "Nombre del adaptador de red (vacio para deteccion automatica)")
	durationFlag := flag.Int("d", 62, "Duracion de captura en segundos (default 62s)")
	validateFlag := flag.Bool("v", true, "Validar conexion de punto de red antes de escanear")
	appendFlag := flag.Bool("a", false, "Modo acumulativo: sumar escaneos de red al reporte existente")
	autoFlag := flag.Bool("auto", false, "Modo automatico: monitoriza conexion de cable y extrae al conectar")
	outputFlag := flag.String("o", "sre_network_inventory.json", "Ruta del archivo de salida JSON")
	mdFlag := flag.Bool("md", true, "Generar tambien reporte en formato Markdown (.md)")
	csvFlag := flag.Bool("csv", true, "Generar tambien reporte en formato CSV (.csv)")
	remoteFlag := flag.String("remote", "", "URL HTTPS de canal seguro para envio automatico de datos")
	listFlag := flag.Bool("list", false, "Listar adaptadores de red disponibles y salir")
	portFlag := flag.Int("port", 0, "Puerto local para la interfaz gráfica (0 para aleatorio)")
	hostFlag := flag.String("host", "0.0.0.0", "Dirección IP de escucha para la interfaz web")

	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	shouldRunGUI := false
	if *guiFlag {
		shouldRunGUI = true
	} else if !*cliFlag && len(os.Args) == 1 {
		shouldRunGUI = true
	}

	if shouldRunGUI {
		printBanner()
		fmt.Println("[*] Iniciando Interfaz Gráfica interactiva de escritorio...")
		if err := gui.Launch(ctx, *outputFlag, *hostFlag, *portFlag, false); err != nil && err != context.Canceled {
			fmt.Fprintf(os.Stderr, "[ERROR GUI] %v\n", err)
			os.Exit(1)
		}
		return
	}

	printBanner()

	if *listFlag {
		listAdapters()
		return
	}

	cfg := engine.Config{
		AdapterName:       *adapterFlag,
		Duration:          time.Duration(*durationFlag) * time.Second,
		ValidateLinkFirst: *validateFlag,
		AutoExtract:       *autoFlag,
		AppendMode:        *appendFlag,
		OutputPath:        *outputFlag,
		ExportMarkdown:    *mdFlag,
		ExportCSV:         *csvFlag,
		RemoteURL:         *remoteFlag,
		OnStatus: func(msg string) {
			fmt.Println(msg)
		},
	}

	runner := engine.NewRunner()

	if *autoFlag {
		if err := runner.RunAutoExtract(ctx, cfg); err != nil && err != context.Canceled {
			fmt.Fprintf(os.Stderr, "\n[ERROR] %v\n", err)
			os.Exit(1)
		}
		return
	}

	inv, err := runner.RunOnce(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n[ERROR] %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n-----------------------------------------------------------------")
	fmt.Printf("[+] Resumen de Inventario (%d dispositivo(s) registrados):\n", len(inv.Devices))
	for idx, d := range inv.Devices {
		ip := d.SwitchIP
		if ip == "" {
			ip = "No anunciada"
		}
		vlan := "-"
		if d.VLAN != 0 {
			vlan = fmt.Sprintf("VLAN %d", d.VLAN)
		}
		fmt.Printf("  [%d] Switch: %-20s | Puerto: %-15s | %s\n", idx+1, d.SwitchName, d.Port, vlan)
		fmt.Printf("      IP: %-24s | Modelo: %-15s | Proto: %s\n", ip, d.Model, d.Protocol)
		if d.TimesSeen > 1 {
			fmt.Printf("      Detecciones acumuladas: %d | Ultima vez: %s\n", d.TimesSeen, d.LastSeen.Format("15:04:05"))
		}
	}
	fmt.Println("-----------------------------------------------------------------")
	fmt.Printf("Reporte final guardado en: %s\n\n", *outputFlag)
}

func listAdapters() {
	adapters, err := link.GetEligibleAdapters()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error obteniendo adaptadores: %v\n", err)
		return
	}
	fmt.Println("Adaptadores de red elegibles:")
	for idx, a := range adapters {
		state := "DOWN"
		if a.IsUp {
			state = "UP"
		}
		ips := append(a.IPv4, a.IPv6...)
		ipStr := strings.Join(ips, ", ")
		if ipStr == "" {
			ipStr = "Sin IP asignada"
		}
		fmt.Printf("  [%d] %s (%s) - Estado: %s\n", idx+1, a.Name, a.MAC, state)
		fmt.Printf("      IPs: %s\n", ipStr)
	}
	fmt.Println()
}
