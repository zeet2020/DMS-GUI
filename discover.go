package main

import (
	"bytes"
	"context"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

// DiscoveredHost is a device found on the local network during a discovery
// scan, shown so the user can identify it before adding its IP to the
// allowed-clients list.
type DiscoveredHost struct {
	IP           string `json:"ip"`
	MAC          string `json:"mac"`
	Vendor       string `json:"vendor"`
	Hostname     string `json:"hostname"`
	FriendlyName string `json:"friendlyName"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	OSGuess      string `json:"osGuess"`
}

const (
	ssdpWindow   = 2500 * time.Millisecond
	castWindow   = 2000 * time.Millisecond
	dnsPerHostTO = 800 * time.Millisecond
)

// DiscoverHosts scans the local network so the user can identify devices —
// smart TVs and Cast-enabled Android TVs in particular — before typing an IP
// into the allow list. Best-effort, not a full inventory: the ARP/neighbor
// table only holds hosts this machine has recently talked to, SSDP/Cast only
// reach devices that speak those protocols, reverse DNS only resolves names
// the router has registered, and the MAC vendor table identifies the Wi-Fi
// chip maker, which is not always the branded device (a Fire TV's Wi-Fi
// module can be made by a different company than Amazon).
func (a *App) DiscoverHosts() []DiscoveredHost {
	var arpEntries []arpEntry
	var ssdpResults map[string]ssdpHost
	var castResults map[string]castHost

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		arpEntries, _ = readARPTable()
	}()
	go func() {
		defer wg.Done()
		ssdpResults = ssdpDiscover(ssdpWindow)
	}()
	go func() {
		defer wg.Done()
		castResults = mdnsCastDiscover(castWindow)
	}()
	wg.Wait()

	byIP := map[string]*DiscoveredHost{}
	var order []string
	host := func(ip string) *DiscoveredHost {
		h, ok := byIP[ip]
		if !ok {
			h = &DiscoveredHost{IP: ip}
			byIP[ip] = h
			order = append(order, ip)
		}
		return h
	}

	for _, e := range arpEntries {
		// Linux's neighbor table also lists IPv6 entries; the allow list is
		// IPv4-keyed (IPv6 clients are matched to it by MAC), so skip them.
		if ip := net.ParseIP(e.IP); ip == nil || ip.To4() == nil {
			continue
		}
		h := host(e.IP)
		h.MAC = e.MAC
		h.Vendor = lookupVendor(e.MAC)
	}
	for ip, s := range ssdpResults {
		h := host(ip)
		h.OSGuess = s.Server
		h.FriendlyName = s.FriendlyName
		h.Manufacturer = s.Manufacturer
		h.Model = s.ModelName
	}
	for ip, c := range castResults {
		h := host(ip)
		if h.FriendlyName == "" {
			h.FriendlyName = c.Name
		}
		if h.Model == "" {
			h.Model = c.Model
		}
		if h.Manufacturer == "" {
			h.Manufacturer = "Google Cast"
		}
	}

	// Every key already exists in byIP by this point, so the lookups below
	// only ever read the map and write their own host's fields — no shared
	// mutable state between goroutines, no lock needed.
	var dnsWg sync.WaitGroup
	for _, ip := range order {
		h := byIP[ip]
		dnsWg.Add(1)
		go func() {
			defer dnsWg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), dnsPerHostTO)
			defer cancel()
			if names, err := net.DefaultResolver.LookupAddr(ctx, h.IP); err == nil && len(names) > 0 {
				h.Hostname = strings.TrimSuffix(names[0], ".")
			}
		}()
	}
	dnsWg.Wait()

	result := make([]DiscoveredHost, 0, len(order))
	for _, ip := range order {
		result = append(result, *byIP[ip])
	}
	sort.Slice(result, func(i, j int) bool { return ipLess(result[i].IP, result[j].IP) })
	return result
}

// ipLess orders IPv4 addresses numerically (falling back to a string
// comparison for anything unparsable) so the discovered list reads like a
// normal subnet listing instead of ARP-table-arrival order.
func ipLess(a, b string) bool {
	ipA, ipB := net.ParseIP(a), net.ParseIP(b)
	if ipA == nil || ipB == nil {
		return a < b
	}
	if a4, b4 := ipA.To4(), ipB.To4(); a4 != nil && b4 != nil {
		return bytes.Compare(a4, b4) < 0
	}
	return bytes.Compare(ipA, ipB) < 0
}
