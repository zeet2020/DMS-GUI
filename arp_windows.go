//go:build windows

package main

import "os/exec"

// readARPTable reads Windows's ARP cache via `arp -a`.
func readARPTable() ([]arpEntry, error) {
	out, err := exec.Command("arp", "-a").Output()
	if err != nil {
		return nil, err
	}
	return parseWindowsArpOutput(string(out)), nil
}
