//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const autostartFileName = "dms-gui.desktop"

// executablePath returns the path to launch on autostart. Under an AppImage the
// real, persistent path is in $APPIMAGE; os.Executable points into the
// temporary /tmp/.mount_* squashfs that does not survive a reboot.
func executablePath() (string, error) {
	if p := os.Getenv("APPIMAGE"); p != "" {
		return p, nil
	}
	return os.Executable()
}

// autostartPath is the XDG autostart desktop entry, e.g.
// ~/.config/autostart/dms-gui.desktop.
func autostartPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "autostart", autostartFileName), nil
}

func isAutoLaunchEnabled() (bool, error) {
	path, err := autostartPath()
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
	path, err := autostartPath()
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
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=DMS
Comment=UPnP/DLNA media server
Exec=%q
Terminal=false
X-GNOME-Autostart-enabled=true
`, exe)
	return atomicWriteFile(path, []byte(content), 0o644)
}
