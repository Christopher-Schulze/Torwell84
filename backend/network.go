package main

import (
	"log"
	"os/exec"
	"runtime"
)

// enableBBRv2 tries to enable the BBRv2 congestion control algorithm on Linux.
func enableBBRv2() {
	if runtime.GOOS != "linux" {
		return
	}
	// first try bbr2 then fall back to bbr
	if err := exec.Command("sysctl", "-w", "net.ipv4.tcp_congestion_control=bbr2").Run(); err != nil {
		if err := exec.Command("sysctl", "-w", "net.ipv4.tcp_congestion_control=bbr").Run(); err != nil {
			log.Printf("BBR enable failed: %v", err)
		}
	}
}
