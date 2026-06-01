//go:build darwin

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const launchAgentLabel = "com.github.dms-gui"

func executablePath() (string, error) {
	return os.Executable()
}

// launchAgentPath is the per-user LaunchAgent plist, e.g.
// ~/Library/LaunchAgents/com.github.dms-gui.plist.
func launchAgentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist"), nil
}

func isAutoLaunchEnabled() (bool, error) {
	path, err := launchAgentPath()
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func setAutoLaunch(enable bool) error {
	path, err := launchAgentPath()
	if err != nil {
		return err
	}
	if !enable {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	exe, err := executablePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// RunAtLoad launches the app at login; the agent is loaded by launchd on the
	// next login (no launchctl shell-out, so there is no runtime dependency).
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, launchAgentLabel, exe)
	return atomicWriteFile(path, []byte(plist), 0o644)
}
