//go:build linux

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

func ensureWebKitSandbox() {
	if os.Getenv("WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS") != "" ||
		os.Getenv("WEBKIT_FORCE_SANDBOX") != "" {
		return
	}
	if reason := sandboxUnavailable(); reason != "" {
		log.Printf("WebKit sandbox unavailable (%s); starting with the sandbox disabled", reason)
		os.Setenv("WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS", "1")
	}
}

func sandboxUnavailable() string {
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		return "bubblewrap not installed"
	}
	if _, err := exec.LookPath("xdg-dbus-proxy"); err != nil {
		return "xdg-dbus-proxy not installed"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	probe := exec.CommandContext(ctx, bwrap,
		"--unshare-user", "--ro-bind", "/", "/", "/bin/true")
	if out, err := probe.CombinedOutput(); err != nil {
		return fmt.Sprintf("bwrap cannot create namespaces: %v: %s",
			err, strings.TrimSpace(string(out)))
	}
	return ""
}
