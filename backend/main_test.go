package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStatus(t *testing.T) {
	wm = NewWorkerManager()
	cfg = Config{OBFS4: true, PreWarm: true}
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Status{Connected: false, Workers: wm.List(), Config: getConfig()})
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/status", nil)
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestWorkerCRUD(t *testing.T) {
	wm = NewWorkerManager()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/workers", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(wm.List())
		case http.MethodPost:
			var req struct{ URL string }
			json.NewDecoder(r.Body).Decode(&req)
			_ = wm.Add(req.URL)
			w.WriteHeader(http.StatusCreated)
		case http.MethodDelete:
			var req struct{ URL string }
			json.NewDecoder(r.Body).Decode(&req)
			wm.Remove(req.URL)
			w.WriteHeader(http.StatusOK)
		}
	})

	// Add worker
	postReq := httptest.NewRequest(http.MethodPost, "/workers", strings.NewReader(`{"URL":"`+srv.URL+`"}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, postReq)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	// List workers
	getReq := httptest.NewRequest(http.MethodGet, "/workers", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, getReq)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var got []Worker
	json.NewDecoder(w.Body).Decode(&got)
	if len(got) != 1 || got[0].URL != srv.URL {
		t.Fatalf("unexpected workers list: %v", got)
	}
}

func TestWorkerHealthCheck(t *testing.T) {
	// healthy server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wm = NewWorkerManager()
	if err := wm.Add(srv.URL); err != nil {
		t.Fatalf("add: %v", err)
	}
	if len(wm.List()) != 1 || !wm.List()[0].Active {
		t.Fatal("worker not active")
	}

	// make server unhealthy
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	wm.CheckAll()
	if wm.List()[0].Active {
		t.Fatal("expected inactive worker")
	}
}

func TestWorkerNext(t *testing.T) {
	// two healthy servers
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv1.Close()
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv2.Close()

	wm = NewWorkerManager()
	if err := wm.Add(srv1.URL); err != nil {
		t.Fatalf("add1: %v", err)
	}
	if err := wm.Add(srv2.URL); err != nil {
		t.Fatalf("add2: %v", err)
	}

	url, ok := wm.Next()
	if !ok || url != srv1.URL {
		t.Fatalf("expected %s, got %s", srv1.URL, url)
	}
	url, ok = wm.Next()
	if !ok || url != srv2.URL {
		t.Fatalf("expected %s, got %s", srv2.URL, url)
	}

	// make first server unhealthy and check failover
	srv1.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	wm.CheckAll()
	url, ok = wm.Next()
	if !ok || url != srv2.URL {
		t.Fatalf("expected failover to %s, got %s", srv2.URL, url)
	}
}

func TestWorkerPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "workers.json")
	wm = NewWorkerManager()
	if err := wm.Load(path); err != nil {
		t.Fatalf("load: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	if err := wm.Add(srv.URL); err != nil {
		t.Fatalf("add: %v", err)
	}
	wm2 := NewWorkerManager()
	if err := wm2.Load(path); err != nil {
		t.Fatalf("load2: %v", err)
	}
	if len(wm2.List()) != 1 || wm2.List()[0].URL != srv.URL {
		t.Fatalf("persistence failed: %v", wm2.List())
	}
}

func TestTorrcUpload(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("TORWELL84_CONFIG", dir)
	defer os.Unsetenv("TORWELL84_CONFIG")

	tor := filepath.Join(dir, "tor")
	os.WriteFile(tor, []byte("#!/bin/sh\nexit 0"), 0755)
	os.Setenv("TOR_BINARY", tor)
	defer os.Unsetenv("TOR_BINARY")

	handler := newServer()

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	fw, _ := w.CreateFormFile("file", "torrc")
	fw.Write([]byte("SocksPort 9050"))
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/torrc", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("upload failed: %d", resp.Code)
	}

	if _, err := os.Stat(filepath.Join(dir, "torrc")); err != nil {
		t.Fatalf("torrc not saved: %v", err)
	}
}

func TestConfigEndpoints(t *testing.T) {
	cfgDir := t.TempDir()
	os.Setenv("TORWELL84_CONFIG", cfgDir)
	defer os.Unsetenv("TORWELL84_CONFIG")
	loadConfig(cfgDir)

	handler := newServer()

	// get default config
	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get config failed: %d", w.Code)
	}
	var c Config
	json.NewDecoder(w.Body).Decode(&c)
	if !c.OBFS4 || !c.PreWarm {
		t.Fatalf("unexpected default config %+v", c)
	}

	// update config
	body := strings.NewReader(`{"obfs4":false}`)
	req = httptest.NewRequest(http.MethodPost, "/config", body)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("post config failed: %d", w.Code)
	}

	// verify saved value
	req = httptest.NewRequest(http.MethodGet, "/config", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	json.NewDecoder(w.Body).Decode(&c)
	if c.OBFS4 != false {
		t.Fatalf("config not updated: %+v", c)
	}
}

func TestConnectAndLogs(t *testing.T) {
	wm = NewWorkerManager()
	connected = false
	connLogs = nil
	generalLogs = nil

	connLogger, _ = newLogWriter(t.TempDir(), "conn")
	genLogger, _ = newLogWriter(t.TempDir(), "gen")
	mux := http.NewServeMux()
	mux.HandleFunc("/connect", func(w http.ResponseWriter, r *http.Request) {
		connected = true
		addLog(&connLogs, connLogger, "connected")
		addLog(&generalLogs, genLogger, "connected")
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(Status{Connected: connected, Workers: wm.List()})
	})
	mux.HandleFunc("/logs/general", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(generalLogs)
	})

	// connect
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/connect", nil)
	mux.ServeHTTP(w, req)
	if !connected {
		t.Fatal("expected connected state")
	}

	// status
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/status", nil)
	mux.ServeHTTP(w, req)
	var st Status
	json.NewDecoder(w.Body).Decode(&st)
	if !st.Connected {
		t.Fatal("status not connected")
	}

	// logs
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/logs/general", nil)
	mux.ServeHTTP(w, req)
	var logs []string
	json.NewDecoder(w.Body).Decode(&logs)
	if len(logs) == 0 {
		t.Fatal("expected logs")
	}
	connLogger.Close()
	genLogger.Close()
}

func TestLogWriter(t *testing.T) {
	dir := t.TempDir()
	lw, err := newLogWriter(dir, "test")
	if err != nil {
		t.Fatalf("newLogWriter: %v", err)
	}
	lw.Write("one")
	lw.Write("two")
	lw.Close()
	data, err := os.ReadFile(filepath.Join(dir, "test.log"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !bytes.Contains(data, []byte("one")) || !bytes.Contains(data, []byte("two")) {
		t.Fatalf("log entries missing: %s", data)
	}
}

func TestCircuitManager(t *testing.T) {
	cm := NewCircuitManager(2)
	first := cm.Next()
	second := cm.Next()
	if first.ID == second.ID {
		t.Fatalf("expected different circuits")
	}
	third := cm.Next()
	if third.ID == second.ID {
		t.Fatalf("rotation failed")
	}
}

func TestDNSCache(t *testing.T) {
	d := newDNSCache(time.Second)
	addrs1, err := d.LookupHost("localhost")
	if err != nil || len(addrs1) == 0 {
		t.Fatalf("lookup failed: %v", err)
	}
	addrs2, err := d.LookupHost("localhost")
	if err != nil {
		t.Fatalf("lookup2: %v", err)
	}
	if &addrs1[0] == &addrs2[0] {
		t.Fatalf("expected copy of cached slice")
	}
}
