//go:build !windows

package save

import (
	"os"
	"path/filepath"
)

// dataDir is $XDG_DATA_HOME, else ~/.local/share ("" with no home).
func dataDir() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share")
}
