package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// State persists runtime information across restarts.
type State struct {
	Connected bool `json:"connected"`
}

var (
	state   State
	stateMu sync.RWMutex
)

// loadState reads state from disk if present.
func loadState(dir string) error {
	stateMu.Lock()
	defer stateMu.Unlock()
	path := filepath.Join(dir, "state.json")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			state = State{}
			return nil
		}
		return err
	}
	if err := json.Unmarshal(b, &state); err != nil {
		return err
	}
	return nil
}

// saveState writes the current state to disk.
func saveState(dir string) error {
	stateMu.RLock()
	defer stateMu.RUnlock()
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "state.json")
	return os.WriteFile(path, b, 0600)
}

func setConnected(c bool) {
	stateMu.Lock()
	state.Connected = c
	stateMu.Unlock()
}

func isConnected() bool {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return state.Connected
}
