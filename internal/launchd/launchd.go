package launchd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// Runner executes a launchctl invocation and returns its combined output.
type Runner func(args ...string) (string, error)

// Manager installs and controls the creddserver LaunchAgent.
type Manager struct {
	Run       Runner
	UID       int
	PlistPath string
	LogPath   string
}

// Status describes the agent's current state.
type Status struct {
	Installed bool // plist present on disk
	Loaded    bool // bootstrapped into launchd
	PID       int
	LastExit  int
}

// NewManager builds a Manager wired to the real launchctl. It errors on
// non-macOS platforms.
func NewManager() (*Manager, error) {
	if runtime.GOOS != "darwin" {
		return nil, errors.New("service management is only supported on macOS")
	}
	plistPath, err := PlistPath()
	if err != nil {
		return nil, err
	}
	logPath, err := LogPath()
	if err != nil {
		return nil, err
	}
	return &Manager{
		Run:       launchctlRun,
		UID:       os.Getuid(),
		PlistPath: plistPath,
		LogPath:   logPath,
	}, nil
}

func (m *Manager) domainTarget() string  { return fmt.Sprintf("gui/%d", m.UID) }
func (m *Manager) serviceTarget() string { return fmt.Sprintf("gui/%d/%s", m.UID, Label) }

// Install renders the plist (baking serverPath and configPath), writes it, and
// (re)loads the agent. RunAtLoad starts it immediately.
func (m *Manager) Install(serverPath, configPath string) error {
	configPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}
	data, err := renderPlist(PlistOptions{
		Label:      Label,
		ServerPath: serverPath,
		ConfigPath: configPath,
		LogPath:    m.LogPath,
	})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.LogPath), 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(m.PlistPath), 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}
	if err := os.WriteFile(m.PlistPath, data, 0o600); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}
	// Best-effort unload of any prior instance, then load.
	_, _ = m.Run("bootout", m.serviceTarget())
	if _, err := m.Run("bootstrap", m.domainTarget(), m.PlistPath); err != nil {
		return err
	}
	return nil
}

// Uninstall unloads the agent and removes its plist.
func (m *Manager) Uninstall() error {
	_, _ = m.Run("bootout", m.serviceTarget())
	if err := os.Remove(m.PlistPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove plist: %w", err)
	}
	return nil
}

// Start loads the agent (no-op-ish if already loaded).
func (m *Manager) Start() error {
	if _, err := os.Stat(m.PlistPath); errors.Is(err, fs.ErrNotExist) {
		return errors.New("not installed: run 'credd service install' first")
	}
	if _, err := m.Run("bootstrap", m.domainTarget(), m.PlistPath); err != nil {
		// Likely already loaded — try to (re)start it.
		if _, kerr := m.Run("kickstart", m.serviceTarget()); kerr != nil {
			return err
		}
	}
	return nil
}

// Stop unloads the agent. bootout is the only reliable stop given KeepAlive.
// Its error is intentionally ignored: the common case is "not loaded" (already
// stopped), and there is no reliable structured way to distinguish that from a
// genuine failure via launchctl's exit text.
func (m *Manager) Stop() error {
	_, _ = m.Run("bootout", m.serviceTarget())
	return nil
}

// Restart kills and restarts the running agent, loading it first if needed.
func (m *Manager) Restart() error {
	if _, err := m.Run("kickstart", "-k", m.serviceTarget()); err != nil {
		if _, berr := m.Run("bootstrap", m.domainTarget(), m.PlistPath); berr != nil {
			return berr
		}
	}
	return nil
}

// Status reports whether the agent is installed and loaded, with its PID and
// last exit code when loaded.
func (m *Manager) Status() (Status, error) {
	var s Status
	if _, err := os.Stat(m.PlistPath); err == nil {
		s.Installed = true
	}
	out, err := m.Run("list", Label)
	if err != nil {
		return s, nil // not loaded
	}
	s.Loaded = true
	s.PID, s.LastExit = parseListOutput(out)
	return s, nil
}

var (
	pidRe      = regexp.MustCompile(`"PID"\s*=\s*(\d+)`)
	lastExitRe = regexp.MustCompile(`"LastExitStatus"\s*=\s*(\d+)`)
)

func parseListOutput(s string) (pid int, lastExit int) {
	if m := pidRe.FindStringSubmatch(s); m != nil {
		pid, _ = strconv.Atoi(m[1])
	}
	if m := lastExitRe.FindStringSubmatch(s); m != nil {
		lastExit, _ = strconv.Atoi(m[1])
	}
	return pid, lastExit
}

func launchctlRun(args ...string) (string, error) {
	out, err := exec.Command("launchctl", args...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("launchctl %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
