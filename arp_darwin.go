//go:build darwin

package main

import "os/exec"

// readARPTable reads macOS's ARP cache via the BSD `arp -a` tool.
func readARPTable() ([]arpEntry, error) {
	out, err := exec.Command("arp", "-a").Output()
	if err != nil {
		return nil, err
	}
	return parseUnixArpOutput(string(out)), nil
}
