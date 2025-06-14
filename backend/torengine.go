package main

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// TorEngine manages the embedded Tor process.
type TorEngine struct {
	mu  sync.Mutex
	cmd *exec.Cmd
}

var engine = &TorEngine{}

// Start launches the tor process using a torrc located in dir if present.
func (t *TorEngine) Start(dir string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cmd != nil {
		return nil
	}
	tor := os.Getenv("TOR_BINARY")
	if tor == "" {
		tor = "tor"
	}
	args := []string{}
	torrc := filepath.Join(dir, "torrc")
	if _, err := os.Stat(torrc); err == nil {
		args = append(args, "-f", torrc)
	}
	// Append OBFS4 bridges if enabled and bridges.txt exists
	c := getConfig()
	if c.OBFS4 {
		bridges := filepath.Join(dir, "bridges.txt")
		if data, err := os.ReadFile(bridges); err == nil {
			args = append(args, "--UseBridges", "1", "--ClientTransportPlugin", "obfs4 exec obfs4proxy")
			scanner := bufio.NewScanner(bytes.NewReader(data))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				args = append(args, "--Bridge", line)
			}
		}
	}
	cmd := exec.Command(tor, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	t.cmd = cmd
	return nil
}

// Stop terminates the running tor process if started.
func (t *TorEngine) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cmd == nil {
		return nil
	}
	err := t.cmd.Process.Kill()
	t.cmd.Wait()
	t.cmd = nil
	return err
}
