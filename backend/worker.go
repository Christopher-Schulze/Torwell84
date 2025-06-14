package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"sync"
	"time"
)

// Worker represents a single Cloudflare Worker HTTPS endpoint.
type Worker struct {
	URL    string
	Active bool
}

// WorkerManager stores and validates worker endpoints.
type WorkerManager struct {
	mu      sync.RWMutex
	workers []Worker
	client  *http.Client
	index   int
	file    string
}

func NewWorkerManager() *WorkerManager {
	return &WorkerManager{
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// StartHealthChecker runs a loop that periodically checks worker health.
func (m *WorkerManager) StartHealthChecker(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			m.CheckAll()
		}
	}()
}

// List returns a copy of the configured workers.
func (m *WorkerManager) List() []Worker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cp := make([]Worker, len(m.workers))
	copy(cp, m.workers)
	return cp
}

// Next returns the next active worker URL using round robin.
// The bool indicates whether a worker was found.
func (m *WorkerManager) Next() (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.workers) == 0 {
		return "", false
	}
	for i := 0; i < len(m.workers); i++ {
		w := m.workers[m.index%len(m.workers)]
		m.index = (m.index + 1) % len(m.workers)
		if w.Active {
			return w.URL, true
		}
	}
	return "", false
}

// Add validates and adds a new endpoint.
func (m *WorkerManager) Add(url string) error {
	if url == "" {
		return errors.New("empty url")
	}
	if err := m.checkHealth(url); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, w := range m.workers {
		if w.URL == url {
			return errors.New("duplicate url")
		}
	}
	m.workers = append(m.workers, Worker{URL: url, Active: true})
	return m.save()
}

// Remove deletes a worker endpoint if present.
func (m *WorkerManager) Remove(url string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, w := range m.workers {
		if w.URL == url {
			m.workers = append(m.workers[:i], m.workers[i+1:]...)
			break
		}
	}
	_ = m.save()
}

// CheckAll updates the Active status of all workers.
func (m *WorkerManager) CheckAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, w := range m.workers {
		if err := m.checkHealth(w.URL); err != nil {
			m.workers[i].Active = false
		} else {
			m.workers[i].Active = true
		}
	}
	_ = m.save()
}

// checkHealth performs a simple GET on /.well-known/healthz.
func (m *WorkerManager) checkHealth(url string) error {
	resp, err := m.client.Get(url + "/.well-known/healthz")
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("health check failed")
	}
	return nil
}

// Load reads workers from the given file if it exists.
func (m *WorkerManager) Load(file string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.file = file
	b, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(b, &m.workers)
}

// save persists current workers to the configured file.
func (m *WorkerManager) save() error {
	if m.file == "" {
		return nil
	}
	b, err := json.MarshalIndent(m.workers, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.file, b, 0600)
}
