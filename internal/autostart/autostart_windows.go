//go:build windows

package autostart

import (
	"errors"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName = "NetPulse"
)

// BuildRunCommand quotes the executable path and appends --minimized when requested.
func BuildRunCommand(exePath string, minimized bool) string {
	trimmed := strings.TrimSpace(exePath)
	if trimmed == "" {
		return ""
	}
	cmd := `"` + strings.Trim(trimmed, `"`) + `"`
	if minimized {
		cmd += " --minimized"
	}
	return cmd
}

// IsEnabled checks whether NetPulse is registered in HKCU\...\Run for automatic startup.
func IsEnabled() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()

	val, _, err := key.GetStringValue(runValueName)
	return err == nil && strings.TrimSpace(val) != ""
}

// SetEnabled enables or disables current-user Windows autostart without requiring Admin rights.
func SetEnabled(enabled bool) error {
	if !enabled {
		key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
		if err != nil {
			if errors.Is(err, registry.ErrNotExist) {
				return nil
			}
			return err
		}
		defer key.Close()
		err = key.DeleteValue(runValueName)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := BuildRunCommand(exePath, true)
	if cmd == "" {
		return errors.New("could not resolve NetPulse executable path")
	}

	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	return key.SetStringValue(runValueName, cmd)
}
