package main

import (
	"sync"
	"time"
)

// Circuit represents a single Tor circuit (mock).
type Circuit struct {
	ID int
}

// CircuitManager keeps a pool of pre-warmed circuits.
type CircuitManager struct {
	mu       sync.Mutex
	circuits []Circuit
	size     int
	nextID   int
}

// NewCircuitManager creates a manager that keeps n circuits pre-warmed.
func NewCircuitManager(n int) *CircuitManager {
	cm := &CircuitManager{size: n}
	cm.prewarm()
	return cm
}

// prewarm ensures the pool has the desired number of circuits.
func (cm *CircuitManager) prewarm() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	for len(cm.circuits) < cm.size {
		cm.nextID++
		cm.circuits = append(cm.circuits, Circuit{ID: cm.nextID})
	}
}

// Next returns the next circuit from the pool and triggers pre-warming of a new one.
func (cm *CircuitManager) Next() Circuit {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if len(cm.circuits) == 0 {
		cm.nextID++
		return Circuit{ID: cm.nextID}
	}
	c := cm.circuits[0]
	cm.circuits = cm.circuits[1:]
	cm.nextID++
	cm.circuits = append(cm.circuits, Circuit{ID: cm.nextID})
	return c
}

// Start periodically ensures circuits remain pre-warmed.
func (cm *CircuitManager) Start(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			cm.prewarm()
		}
	}()
}
