#!/usr/bin/env python3
"""Regenerates oui_table.go from IEEE's public OUI registry.

Usage: python3 scripts/gen_oui.py

Downloads https://standards-oui.ieee.org/oui/oui.txt, filters it down to
vendors likely to show up on a home network (see VENDOR_RULES below), and
writes the result as a Go map literal to oui_table.go. Run this again to pick
up new OUI assignments or to add a vendor to VENDOR_RULES.
"""
import os
import re
import sys
import urllib.request
from collections import Counter

REGISTRY_URL = "https://standards-oui.ieee.org/oui/oui.txt"
REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OUI_TXT = os.path.join(REPO_ROOT, "scripts", ".oui.txt.cache")
OUT_GO = os.path.join(REPO_ROOT, "oui_table.go")

# (regex, short display name) — order matters, first match wins.
VENDOR_RULES = [
    (r"\bapple\b", "Apple"),
    (r"\bgoogle\b", "Google"),
    (r"\bnest labs\b", "Google Nest"),
    (r"\bamazon\b", "Amazon"),
    (r"\bmicrosoft\b", "Microsoft"),
    (r"\bsamsung\b", "Samsung"),
    (r"\bsony\b", "Sony"),
    (r"\blg electronics\b", "LG"),
    (r"\broku\b", "Roku"),
    (r"\braspberry pi\b", "Raspberry Pi"),
    (r"\bespressif\b", "Espressif (IoT)"),
    (r"\bintel corp", "Intel"),
    (r"\bdell inc", "Dell"),
    (r"\bhewlett packard\b|\bhp inc\b", "HP"),
    (r"\blenovo\b", "Lenovo"),
    (r"\basustek\b", "ASUS"),
    (r"\bacer inc\b", "Acer"),
    (r"\btp-link\b", "TP-Link"),
    (r"\bnetgear\b", "Netgear"),
    (r"\bd-link\b", "D-Link"),
    (r"\bbelkin\b", "Belkin"),
    (r"\bubiquiti\b", "Ubiquiti"),
    (r"\bxiaomi\b", "Xiaomi"),
    (r"\bhuawei\b", "Huawei"),
    (r"\bnintendo\b", "Nintendo"),
    (r"\bphilips\b", "Philips"),
    (r"\bring llc\b|\bring, inc", "Ring"),
    (r"\bsonos\b", "Sonos"),
    (r"\blogitech\b", "Logitech"),
    (r"\becobee\b", "Ecobee"),
    (r"\bnvidia\b", "Nvidia"),
    (r"\bvizio\b", "Vizio"),
    (r"\boneplus\b", "OnePlus"),
    (r"\bcisco\b", "Cisco"),
    (r"\bmurata\b", "Murata (chipset)"),
    (r"\brealtek\b", "Realtek (chipset)"),
    (r"\bwyze\b", "Wyze"),
    (r"\banker\b", "Anker"),
    (r"\bmotorola\b", "Motorola"),
    (r"\bhtc corporation\b", "HTC"),
    (r"\bzte corporation\b", "ZTE"),
    (r"\btcl\b", "TCL"),
    (r"\bhonhai|foxconn\b", "Foxconn"),
    (r"\bwistron\b", "Wistron"),
    (r"\bpegatron\b", "Pegatron"),
    (r"\bcompal\b", "Compal"),
    (r"\bmicro-star|\bmsi\b", "MSI"),
    (r"\bgigabyte\b", "Gigabyte"),
    (r"\bactiontec\b", "Actiontec"),
    (r"\barris\b", "Arris"),
    (r"\btechnicolor\b", "Technicolor"),
    (r"\bhon hai precision\b", "Foxconn"),
    (r"\bsagemcom\b", "Sagemcom"),
    (r"\bzyxel\b", "Zyxel"),
]


def classify(org: str):
    low = org.lower()
    for pattern, short in VENDOR_RULES:
        if re.search(pattern, low):
            return short
    return None


def main():
    if not os.path.exists(OUI_TXT):
        print(f"Downloading {REGISTRY_URL} ...", file=sys.stderr)
        urllib.request.urlretrieve(REGISTRY_URL, OUI_TXT)

    entries = {}
    with open(OUI_TXT, encoding="utf-8", errors="ignore") as f:
        for line in f:
            m = re.match(r"^([0-9A-F]{6})\s+\(base 16\)\s+(.+?)\s*$", line.strip())
            if not m:
                continue
            prefix, org = m.group(1), m.group(2)
            short = classify(org)
            if short:
                entries[prefix] = short

    print(f"matched {len(entries)} entries", file=sys.stderr)
    for name, n in Counter(entries.values()).most_common():
        print(f"  {name}: {n}", file=sys.stderr)

    with open(OUT_GO, "w") as out:
        out.write("package main\n\n")
        out.write("// ouiVendors maps a MAC OUI prefix (first 3 octets, uppercase hex, no\n")
        out.write("// separators) to a short vendor name, for the network discovery feature's\n")
        out.write("// device-identification hint. Generated from IEEE's public OUI registry\n")
        out.write(f"// ({REGISTRY_URL}), filtered to vendors\n")
        out.write("// likely to show up on a home network. Regenerate with scripts/gen_oui.py.\n")
        out.write("var ouiVendors = map[string]string{\n")
        for prefix in sorted(entries):
            out.write(f'\t"{prefix}": "{entries[prefix]}",\n')
        out.write("}\n")


if __name__ == "__main__":
    main()
