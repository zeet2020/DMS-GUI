package main

import (
	"net"
	"testing"
)

func TestMacSharesAllowedIP(t *testing.T) {
	entries := parseIPNeighOutput(`192.168.0.107 dev enp5s0 lladdr aa:bb:cc:dd:ee:ff STALE
fe80::1 dev enp5s0 lladdr aa:bb:cc:dd:ee:ff REACHABLE
fe80::2 dev enp5s0 lladdr 11:22:33:44:55:66 REACHABLE
192.168.0.50 dev enp5s0 lladdr 11:22:33:44:55:66 STALE
`)
	nets := makeIpNets("192.168.0.107")
	if !macSharesAllowedIP(net.ParseIP("fe80::1"), entries, nets) {
		t.Error("IPv6 of allowed device should be admitted")
	}
	if macSharesAllowedIP(net.ParseIP("fe80::2"), entries, nets) {
		t.Error("IPv6 of non-allowed device must be denied")
	}
	if macSharesAllowedIP(net.ParseIP("fe80::9"), entries, nets) {
		t.Error("unknown IPv6 must be denied")
	}
}
