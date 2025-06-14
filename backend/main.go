package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

var (
	dnsC           = newDNSCache(5 * time.Minute)
	wm             = NewWorkerManager(dnsC)
	cm             = NewCircuitManager(3)
	progress       int
	progMu         sync.RWMutex
	statusMsg      string = "disconnected"
	statusMu       sync.RWMutex
	progressCancel context.CancelFunc
	connLogs       []string
	generalLogs    []string
	connLogger     *logWriter
	genLogger      *logWriter
	lastIP         string
	cfgDir         string
)

func setProgress(p int) {
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	progMu.Lock()
	progress = p
	progMu.Unlock()
}

func setStatus(s string) {
	statusMu.Lock()
	statusMsg = s
	statusMu.Unlock()
}

func getStatus() string {
	statusMu.RLock()
	defer statusMu.RUnlock()
	return statusMsg
}

func getProgress() int {
	progMu.RLock()
	defer progMu.RUnlock()
	return progress
}

func startProgress() {
	if progressCancel != nil {
		progressCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	progressCancel = cancel
	setProgress(0)
	go func() {
		steps := []struct {
			p int
			s string
		}{{10, "connecting"}, {30, "handshake"}, {60, "establishing"}, {80, "almost ready"}, {100, "ready"}}
		for _, step := range steps {
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
				setProgress(step.p)
				setStatus(step.s)
			}
		}
	}()
}

func stopProgress() {
	if progressCancel != nil {
		progressCancel()
		progressCancel = nil
	}
	setProgress(0)
	setStatus("disconnected")
}

func configDir() string {
	if d := os.Getenv("TORWELL84_CONFIG"); d != "" {
		return d
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "."
	}
	return filepath.Join(dir, "torwell84")
}

type Status struct {
	Connected bool     `json:"connected"`
	Progress  int      `json:"progress"`
	Status    string   `json:"status"`
	Workers   []Worker `json:"workers"`
	Config    Config   `json:"config"`
}

func addLog(dst *[]string, lw *logWriter, msg string) {
	entry := time.Now().Format(time.RFC3339) + " " + msg
	*dst = append(*dst, entry)
	if len(*dst) > 1000 {
		*dst = (*dst)[len(*dst)-1000:]
	}
	if lw != nil {
		lw.Write(entry)
	}
}

// monitorIP periodically checks the primary IP address and logs changes.
func monitorIP(ctx context.Context, interval time.Duration) {
	ip := getLocalIP()
	lastIP = ip
	addLog(&generalLogs, genLogger, "initial ip "+ip)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current := getLocalIP()
			if current != lastIP {
				addLog(&generalLogs, genLogger, "ip changed to "+current)
				lastIP = current
			}
		}
	}
}

func getLocalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// handleTorrc verifies and stores an uploaded torrc file.
func handleTorrc(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	tmp, err := os.CreateTemp("", "torrc")
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, file); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	tmp.Close()

	tor := os.Getenv("TOR_BINARY")
	if tor == "" {
		tor = "tor"
	}
	cmd := exec.Command(tor, "-f", tmp.Name(), "--verify-config")
	if err := cmd.Run(); err != nil {
		http.Error(w, "invalid torrc", http.StatusBadRequest)
		return
	}

	dst := filepath.Join(cfgDir, "torrc")
	if err := os.Rename(tmp.Name(), dst); err != nil {
		http.Error(w, "save error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func newServer() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Status{Connected: isConnected(), Progress: getProgress(), Status: getStatus(), Workers: wm.List(), Config: getConfig()})
	})

	mux.HandleFunc("/connect", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Entry  string   `json:"entry"`
			Middle string   `json:"middle"`
			Exit   string   `json:"exit"`
			CFList []string `json:"cflist"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if err := engine.Start(cfgDir); err != nil {
			http.Error(w, "tor start failed", http.StatusInternalServerError)
			return
		}
		setConnected(true)
		setStatus("connecting")
		_ = saveState(cfgDir)
		startProgress()
		c := cm.Next()
		if url, ok := wm.Next(); ok {
			addLog(&connLogs, connLogger, "circuit "+fmt.Sprint(c.ID)+" via "+url)
			addLog(&generalLogs, genLogger, "using worker "+url)
		} else {
			addLog(&connLogs, connLogger, fmt.Sprintf("circuit %d direct", c.ID))
			addLog(&generalLogs, genLogger, "no active worker; direct exit")
		}
		addLog(&generalLogs, genLogger, "circuit entry="+req.Entry+" middle="+req.Middle+" exit="+req.Exit)
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/disconnect", func(w http.ResponseWriter, r *http.Request) {
		_ = engine.Stop()
		stopProgress()
		setConnected(false)
		setStatus("disconnected")
		_ = saveState(cfgDir)
		addLog(&connLogs, connLogger, "disconnected")
		addLog(&generalLogs, genLogger, "disconnected")
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/new-circuit", func(w http.ResponseWriter, r *http.Request) {
		c := cm.Next()
		addLog(&generalLogs, genLogger, fmt.Sprintf("rotated to circuit %d", c.ID))
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/new-identity", func(w http.ResponseWriter, r *http.Request) {
		addLog(&generalLogs, genLogger, "new identity requested")
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/workers", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(wm.List())
		case http.MethodPost:
			var req struct{ URL string }
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			if err := wm.Add(req.URL); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusCreated)
		case http.MethodDelete:
			var req struct{ URL string }
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			wm.Remove(req.URL)
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/workers/test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct{ URL string }
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.URL == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(wm.TestAll())
			return
		}
		if err := wm.Test(req.URL); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(getConfig())
		case http.MethodPost:
			var c Config
			if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			updateConfig(c)
			if err := saveConfig(cfgDir); err != nil {
				http.Error(w, "save error", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/logs/connection", func(w http.ResponseWriter, r *http.Request) {
		_ = r.URL.Query().Get("level") // level currently unused
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(connLogs)
	})

	mux.HandleFunc("/logs/general", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(generalLogs)
	})

	mux.HandleFunc("/torrc", handleTorrc)

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "torwell_connected %d\n", boolInt(isConnected()))
		fmt.Fprintf(w, "torwell_progress %d\n", getProgress())
		fmt.Fprintf(w, "torwell_workers %d\n", len(wm.List()))
	})

	return mux
}

func main() {
	cfgDir = configDir()
	os.MkdirAll(cfgDir, 0700)
	logDir := filepath.Join(cfgDir, "logs")
	var err error
	connLogger, err = newLogWriter(logDir, "connection")
	if err != nil {
		log.Printf("log writer error: %v", err)
	}
	genLogger, err = newLogWriter(logDir, "general")
	if err != nil {
		log.Printf("log writer error: %v", err)
	}
	wm.Load(filepath.Join(cfgDir, "workers.json"))
	loadConfig(cfgDir)
	loadState(cfgDir)
	enableBBRv2()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	wm.StartHealthChecker(ctx, 30*time.Second)
	go monitorIP(ctx, 10*time.Second)
	cm.Start(ctx, 30*time.Second)

	addr := "127.0.0.1:9472"
	srv := &http.Server{Addr: addr, Handler: newServer()}
	go func() {
		log.Printf("starting server on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()
	<-ctx.Done()
	log.Println("shutting down")
	srv.Shutdown(context.Background())
	engine.Stop()
	connLogger.Close()
	genLogger.Close()
}
