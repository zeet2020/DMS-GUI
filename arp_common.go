package main

import (
	"bufio"
	"regexp"
	"strings"
)

// arpEntry is one resolved IP/MAC pair read from the OS's ARP or neighbor
// table.
type arpEntry struct {
	IP  string
	MAC string
}

// normalizeMAC converts a dash- or colon-separated MAC into canonical
// lowercase colon form, e.g. "B8-27-EB-55-95-A0" -> "b8:27:eb:55:95:a0".
func normalizeMAC(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, "-", ":"))
}

// unixArpRe matches the BSD/net-tools "arp -a" line format used by macOS and
// (as a fallback) Linux, e.g.:
//
//	hostname (192.168.0.100) at b8:27:eb:55:95:a0 [ether] on enp5s0
//	? (192.168.0.100) at b8:27:eb:55:95:a0 on en0 ifscope [ethernet]
//
// Incomplete entries ("at (incomplete)") are skipped since they have no MAC.
var unixArpRe = regexp.MustCompile(`\(([0-9a-fA-F.:]+)\)\s+at\s+([0-9a-fA-F:]{11,17})`)

func parseUnixArpOutput(out string) []arpEntry {
	var entries []arpEntry
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		if m := unixArpRe.FindStringSubmatch(sc.Text()); m != nil {
			entries = append(entries, arpEntry{IP: m[1], MAC: normalizeMAC(m[2])})
		}
	}
	return entries
}

// ipNeighRe matches Linux `ip neigh show` lines, e.g.:
//
//	192.168.0.100 dev enp5s0 lladdr b8:27:eb:55:95:a0 STALE
//
// Entries with no resolved lladdr (e.g. state FAILED) are skipped.
var ipNeighRe = regexp.MustCompile(`^(\S+)\s+dev\s+\S+\s+lladdr\s+([0-9a-fA-F:]{11,17})`)

func parseIPNeighOutput(out string) []arpEntry {
	var entries []arpEntry
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		if m := ipNeighRe.FindStringSubmatch(sc.Text()); m != nil {
			entries = append(entries, arpEntry{IP: m[1], MAC: normalizeMAC(m[2])})
		}
	}
	return entries
}

// windowsArpRe matches Windows `arp -a` table rows, e.g.:
//
//	192.168.0.1           34-60-f9-05-99-17     dynamic
var windowsArpRe = regexp.MustCompile(`^\s*(\d+\.\d+\.\d+\.\d+)\s+([0-9a-fA-F-]{11,17})\s+(dynamic|static)`)

func parseWindowsArpOutput(out string) []arpEntry {
	var entries []arpEntry
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		if m := windowsArpRe.FindStringSubmatch(sc.Text()); m != nil {
			entries = append(entries, arpEntry{IP: m[1], MAC: normalizeMAC(m[2])})
		}
	}
	return entries
}
