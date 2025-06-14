package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config holds user adjustable settings.
type Config struct {
	OBFS4   bool `json:"obfs4"`
	PreWarm bool `json:"prewarm"`
}

var (
	cfg   Config
	cfgMu sync.RWMutex
)

// loadConfig reads configuration from disk.
func loadConfig(dir string) error {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	path := filepath.Join(dir, "config.json")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg = Config{OBFS4: true, PreWarm: true}
			return nil
		}
		return err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return err
	}
	return nil
}

// saveConfig writes the current config to disk.
func saveConfig(dir string) error {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "config.json")
	return os.WriteFile(path, b, 0600)
}

// getConfig returns a copy of current configuration.
func getConfig() Config {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	return cfg
}

// updateConfig merges new settings into the current config.
func updateConfig(c Config) {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	if c.OBFS4 != cfg.OBFS4 {
		cfg.OBFS4 = c.OBFS4
	}
	if c.PreWarm != cfg.PreWarm {
		cfg.PreWarm = c.PreWarm
	}
}
