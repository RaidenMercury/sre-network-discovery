package gui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/RaidenMercury/sre-network-discovery/pkg/engine"
	"github.com/RaidenMercury/sre-network-discovery/pkg/link"
	"github.com/RaidenMercury/sre-network-discovery/pkg/report"
)

//go:embed web/*
var webFS embed.FS

type Server struct {
	mu            sync.Mutex
	inventoryFile string
	currentInv    *report.Inventory
	runner        *engine.Runner
}

func NewServer(inventoryFile string) *Server {
	inv, err := report.LoadInventory(inventoryFile)
	if err != nil {
		inv = report.NewInventory()
	}
	return &Server{
		inventoryFile: inventoryFile,
		currentInv:    inv,
		runner:        engine.NewRunner(),
	}
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data, err := webFS.ReadFile("web/index.html")
		if err != nil {
			http.Error(w, "Error cargando interfaz web", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	mux.HandleFunc("/api/adapters", func(w http.ResponseWriter, r *http.Request) {
		adapters, err := link.GetEligibleAdapters()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(adapters)
	})

	mux.HandleFunc("/api/validate", func(w http.ResponseWriter, r *http.Request) {
		adapter := r.URL.Query().Get("adapter")
		remote := r.URL.Query().Get("remote")
		val := link.ValidateLink(adapter, remote)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(val)
	})

	mux.HandleFunc("/api/inventory", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s.currentInv)
	})

	mux.HandleFunc("/api/scan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Metodo no permitido", 405)
			return
		}

		var req struct {
			Adapter  string `json:"adapter"`
			Duration int    `json:"duration"`
			Append   bool   `json:"append"`
			Remote   string `json:"remote"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON invalido", 400)
			return
		}

		cfg := engine.Config{
			AdapterName:       req.Adapter,
			Duration:          time.Duration(req.Duration) * time.Second,
			ValidateLinkFirst: true,
			AppendMode:        req.Append,
			OutputPath:        s.inventoryFile,
			ExportMarkdown:    true,
			ExportCSV:         true,
			RemoteURL:         req.Remote,
		}

		inv, err := s.runner.RunOnce(r.Context(), cfg)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		s.mu.Lock()
		s.currentInv = inv
		s.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(inv)
	})

	mux.HandleFunc("/api/clear", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.currentInv = report.NewInventory()
		_ = s.currentInv.SaveJSON(s.inventoryFile)
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})

	mux.HandleFunc("/api/export", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()

		format := r.URL.Query().Get("format")
		switch format {
		case "md":
			w.Header().Set("Content-Disposition", "attachment; filename=sre_inventory.md")
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			tmpFile := s.inventoryFile + ".tmp.md"
			_ = s.currentInv.SaveMarkdown(tmpFile)
			http.ServeFile(w, r, tmpFile)
		case "csv":
			w.Header().Set("Content-Disposition", "attachment; filename=sre_inventory.csv")
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			tmpFile := s.inventoryFile + ".tmp.csv"
			_ = s.currentInv.SaveCSV(tmpFile)
			http.ServeFile(w, r, tmpFile)
		default:
			w.Header().Set("Content-Disposition", "attachment; filename=sre_inventory.json")
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(s.currentInv)
		}
	})

	return mux
}

func Launch(ctx context.Context, inventoryFile string, host string, port int, openWindow bool) error {
	if host == "" {
		host = "0.0.0.0"
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return fmt.Errorf("no se pudo abrir puerto local: %w", err)
	}
	actualPort := listener.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://localhost:%d", actualPort)

	srv := NewServer(inventoryFile)
	httpServer := &http.Server{
		Handler: srv.routes(),
	}

	go func() {
		<-ctx.Done()
		_ = httpServer.Shutdown(context.Background())
	}()

	if openWindow {
		go func() {
			time.Sleep(300 * time.Millisecond)
			openAppWindow(url)
		}()
	}

	fmt.Printf("[+] Interfaz Gráfica activa y accesible en:\n")
	fmt.Printf("    - Local:     http://localhost:%d\n", actualPort)
	fmt.Printf("    - Tailscale: http://100.82.249.16:%d\n", actualPort)
	fmt.Printf("    - LAN:       http://192.168.1.85:%d\n", actualPort)
	if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func openAppWindow(url string) {
	if runtime.GOOS == "windows" {
		edgePaths := []string{
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		}
		for _, p := range edgePaths {
			cmd := exec.Command(p, fmt.Sprintf("--app=%s", url), "--window-size=680,880")
			if err := cmd.Start(); err == nil {
				return
			}
		}
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	} else if runtime.GOOS == "linux" {
		for _, b := range []string{"google-chrome", "chromium", "brave-browser", "xdg-open"} {
			args := []string{url}
			if strings.Contains(b, "chrome") || strings.Contains(b, "chromium") {
				args = []string{fmt.Sprintf("--app=%s", url), "--window-size=680,880"}
			}
			cmd := exec.Command(b, args...)
			if err := cmd.Start(); err == nil {
				return
			}
		}
	} else {
		_ = exec.Command("open", url).Start()
	}
}
