//go:build linux

package main

import "os/exec"

// readARPTable prefers `ip neigh show` (iproute2, present on all modern
// Linux) and falls back to `arp -an` (net-tools) if that yields nothing.
func readARPTable() ([]arpEntry, error) {
	if out, err := exec.Command("ip", "neigh", "show").Output(); err == nil {
		if entries := parseIPNeighOutput(string(out)); len(entries) > 0 {
			return entries, nil
		}
	}
	out, err := exec.Command("arp", "-an").Output()
	if err != nil {
		return nil, err
	}
	return parseUnixArpOutput(string(out)), nil
}
