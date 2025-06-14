package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

var (
	wm          = NewWorkerManager()
	cm          = NewCircuitManager(3)
	dnsC        = newDNSCache(5 * time.Minute)
	connected   bool
	connLogs    []string
	generalLogs []string
	connLogger  *logWriter
	genLogger   *logWriter
	lastIP      string
)

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
func monitorIP(interval time.Duration) {
	ip := getLocalIP()
	lastIP = ip
	addLog(&generalLogs, genLogger, "initial ip "+ip)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		current := getLocalIP()
		if current != lastIP {
			addLog(&generalLogs, genLogger, "ip changed to "+current)
			lastIP = current
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

	dst := filepath.Join(configDir(), "torrc")
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
		json.NewEncoder(w).Encode(Status{Connected: connected, Workers: wm.List(), Config: getConfig()})
	})

	mux.HandleFunc("/connect", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Entry  string   `json:"entry"`
			Middle string   `json:"middle"`
			Exit   string   `json:"exit"`
			CFList []string `json:"cflist"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		connected = true
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
		connected = false
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
			if err := saveConfig(configDir()); err != nil {
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

	return mux
}

func main() {
	cfg := configDir()
	os.MkdirAll(cfg, 0700)
	logDir := filepath.Join(cfg, "logs")
	var err error
	connLogger, err = newLogWriter(logDir, "connection")
	if err != nil {
		log.Printf("log writer error: %v", err)
	}
	genLogger, err = newLogWriter(logDir, "general")
	if err != nil {
		log.Printf("log writer error: %v", err)
	}
	wm.Load(filepath.Join(cfg, "workers.json"))
	loadConfig(cfg)
	enableBBRv2()
	wm.StartHealthChecker(30 * time.Second)
	go monitorIP(10 * time.Second)
	cm.Start(30 * time.Second)

	addr := "127.0.0.1:9472"
	log.Printf("starting server on %s", addr)
	log.Fatal(http.ListenAndServe(addr, newServer()))
}
