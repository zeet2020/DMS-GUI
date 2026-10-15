package main

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"net"
	"net/http"
	"sync"
	"time"
)

// ssdpHost is what SSDP revealed about one device: the raw Server header
// (always present when a device replies at all) plus, when its LOCATION
// descriptor could be fetched, the manufacturer/model/friendly name from the
// device's own UPnP description — e.g. "Amazon"/"AFTMM" instead of just
// "Linux/5.4 UPnP/1.0", and far more reliable than guessing from the MAC's
// OUI (which only identifies the Wi-Fi chip vendor, not the branded device).
type ssdpHost struct {
	Server       string
	FriendlyName string
	Manufacturer string
	ModelName    string
}

const ssdpDescribeTimeout = 2 * time.Second

// ssdpDiscover sends an SSDP M-SEARCH multicast, collects replies for the
// given window, then fetches each device's LOCATION descriptor. Only
// identifies UPnP-active devices (smart TVs, routers, other DLNA
// servers/renderers) — a silent laptop or phone won't answer.
func ssdpDiscover(window time.Duration) map[string]ssdpHost {
	results := map[string]ssdpHost{}
	locations := map[string]string{} // ip -> LOCATION url

	addr, err := net.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	if err != nil {
		return results
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return results
	}
	defer conn.Close()

	const msg = "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 2\r\n" +
		"ST: ssdp:all\r\n\r\n"
	if _, err := conn.WriteToUDP([]byte(msg), addr); err != nil {
		return results
	}

	_ = conn.SetReadDeadline(time.Now().Add(window))
	buf := make([]byte, 2048)
	for {
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			break // deadline reached
		}
		resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(buf[:n])), nil)
		if err != nil {
			continue
		}
		ip := from.IP.String()
		h := results[ip]
		if server := resp.Header.Get("Server"); server != "" {
			h.Server = server
		}
		results[ip] = h
		if loc := resp.Header.Get("Location"); loc != "" {
			locations[ip] = loc
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	for ip, loc := range locations {
		ip, loc := ip, loc
		wg.Add(1)
		go func() {
			defer wg.Done()
			desc, err := fetchSSDPDescription(loc)
			if err != nil {
				return
			}
			mu.Lock()
			h := results[ip]
			h.FriendlyName = desc.Device.FriendlyName
			h.Manufacturer = desc.Device.Manufacturer
			h.ModelName = desc.Device.ModelName
			results[ip] = h
			mu.Unlock()
		}()
	}
	wg.Wait()

	return results
}

type ssdpDeviceDesc struct {
	Device struct {
		FriendlyName string `xml:"friendlyName"`
		Manufacturer string `xml:"manufacturer"`
		ModelName    string `xml:"modelName"`
	} `xml:"device"`
}

func fetchSSDPDescription(location string) (*ssdpDeviceDesc, error) {
	client := http.Client{Timeout: ssdpDescribeTimeout}
	resp, err := client.Get(location)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var desc ssdpDeviceDesc
	if err := xml.NewDecoder(resp.Body).Decode(&desc); err != nil {
		return nil, err
	}
	return &desc, nil
}
