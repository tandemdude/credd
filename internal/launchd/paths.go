// Package launchd manages creddserver as a per-user macOS LaunchAgent.
package launchd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Label is the LaunchAgent label (reverse-DNS of the module's repo).
const Label = "io.github.tandemdude.creddserver"

// PlistPath returns the per-user LaunchAgent plist path.
func PlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", Label+".plist"), nil
}

// LogPath returns the file the agent's stdout/stderr are captured to.
func LogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, "Library", "Logs", "creddserver.log"), nil
}

// ResolveServerPath finds the creddserver binary: first as a sibling of selfExe
// (the running credd binary), then on PATH.
func ResolveServerPath(selfExe string) (string, error) {
	sibling := filepath.Join(filepath.Dir(selfExe), "creddserver")
	if fi, err := os.Stat(sibling); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
		return sibling, nil
	}
	if p, err := exec.LookPath("creddserver"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("creddserver binary not found next to %s or on PATH", selfExe)
}
