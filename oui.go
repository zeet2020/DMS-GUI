package main

import "strings"

// lookupVendor maps a MAC's OUI (first 3 octets) to a vendor name using data
// derived from IEEE's public OUI registry (see oui_table.go). Returns "" when
// the prefix isn't in the trimmed table.
func lookupVendor(mac string) string {
	clean := strings.ToUpper(strings.NewReplacer(":", "", "-", "").Replace(mac))
	if len(clean) < 6 {
		return ""
	}
	return ouiVendors[clean[:6]]
}
