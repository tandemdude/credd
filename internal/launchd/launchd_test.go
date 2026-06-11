package launchd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeResult struct {
	out string
	err error
}

// fakeRunner records launchctl invocations and returns canned results. If
// results is non-empty it is consumed one entry per call; otherwise out/err
// are returned for every call.
type fakeRunner struct {
	calls   [][]string
	out     string
	err     error
	results []fakeResult
}

func (f *fakeRunner) run(args ...string) (string, error) {
	f.calls = append(f.calls, args)
	if len(f.results) > 0 {
		r := f.results[0]
		f.results = f.results[1:]
		return r.out, r.err
	}
	return f.out, f.err
}

func newManager(t *testing.T, fr *fakeRunner) *Manager {
	t.Helper()
	dir := t.TempDir()
	return &Manager{
		Run:       fr.run,
		UID:       501,
		PlistPath: filepath.Join(dir, "agent.plist"),
		LogPath:   filepath.Join(dir, "logs", "creddserver.log"),
	}
}

func TestManager_Install(t *testing.T) {
	fr := &fakeRunner{}
	m := newManager(t, fr)

	if err := m.Install("/usr/local/bin/creddserver", "/home/u/.credd/config.toml"); err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, err := os.ReadFile(m.PlistPath)
	if err != nil {
		t.Fatalf("read plist: %v", err)
	}
	for _, w := range []string{"/usr/local/bin/creddserver", "/home/u/.credd/config.toml", Label} {
		if !strings.Contains(string(data), w) {
			t.Errorf("plist missing %q", w)
		}
	}
	if fi, _ := os.Stat(m.PlistPath); fi.Mode().Perm() != 0o600 {
		t.Errorf("plist perm = %o, want 600", fi.Mode().Perm())
	}

	if len(fr.calls) != 2 {
		t.Fatalf("calls = %v, want bootout then bootstrap", fr.calls)
	}
	if fr.calls[0][0] != "bootout" || fr.calls[0][1] != "gui/501/"+Label {
		t.Errorf("call[0] = %v, want bootout gui/501/%s", fr.calls[0], Label)
	}
	if fr.calls[1][0] != "bootstrap" || fr.calls[1][1] != "gui/501" || fr.calls[1][2] != m.PlistPath {
		t.Errorf("call[1] = %v, want bootstrap gui/501 %s", fr.calls[1], m.PlistPath)
	}
}

func TestManager_InstallBootstrapError(t *testing.T) {
	fr := &fakeRunner{err: errors.New("boom")}
	m := newManager(t, fr)
	if err := m.Install("/bin/creddserver", "/c.toml"); err == nil {
		t.Fatal("expected error when bootstrap fails")
	}
}

func TestManager_Uninstall(t *testing.T) {
	fr := &fakeRunner{}
	m := newManager(t, fr)
	if err := os.MkdirAll(filepath.Dir(m.PlistPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m.PlistPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := m.Uninstall(); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(m.PlistPath); !errors.Is(err, os.ErrNotExist) {
		t.Error("plist should be removed")
	}
	if fr.calls[0][0] != "bootout" || fr.calls[0][1] != "gui/501/"+Label {
		t.Errorf("call = %v, want bootout gui/501/%s", fr.calls[0], Label)
	}
}

func TestManager_StartNotInstalled(t *testing.T) {
	fr := &fakeRunner{}
	m := newManager(t, fr)
	if err := m.Start(); err == nil {
		t.Fatal("expected error when plist is missing")
	}
	if len(fr.calls) != 0 {
		t.Errorf("no launchctl calls expected, got %v", fr.calls)
	}
}

func TestManager_Start(t *testing.T) {
	fr := &fakeRunner{}
	m := newManager(t, fr)
	if err := os.MkdirAll(filepath.Dir(m.PlistPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m.PlistPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if fr.calls[0][0] != "bootstrap" || fr.calls[0][1] != "gui/501" || fr.calls[0][2] != m.PlistPath {
		t.Errorf("call = %v, want bootstrap gui/501 %s", fr.calls[0], m.PlistPath)
	}
}

func TestManager_Stop(t *testing.T) {
	fr := &fakeRunner{}
	m := newManager(t, fr)
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if fr.calls[0][0] != "bootout" || fr.calls[0][1] != "gui/501/"+Label {
		t.Errorf("call = %v, want bootout gui/501/%s", fr.calls[0], Label)
	}
}

func TestManager_Restart(t *testing.T) {
	fr := &fakeRunner{}
	m := newManager(t, fr)
	if err := m.Restart(); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if fr.calls[0][0] != "kickstart" || fr.calls[0][1] != "-k" || fr.calls[0][2] != "gui/501/"+Label {
		t.Errorf("call = %v, want kickstart -k gui/501/%s", fr.calls[0], Label)
	}
}

func TestManager_Status_Loaded(t *testing.T) {
	fr := &fakeRunner{out: "{\n\t\"PID\" = 4242;\n\t\"LastExitStatus\" = 0;\n};\n"}
	m := newManager(t, fr)
	if err := os.MkdirAll(filepath.Dir(m.PlistPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m.PlistPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := m.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Installed || !st.Loaded || st.PID != 4242 || st.LastExit != 0 {
		t.Fatalf("status = %+v, want installed+loaded pid=4242 exit=0", st)
	}
}

func TestManager_Status_NotLoaded(t *testing.T) {
	fr := &fakeRunner{err: errors.New("Could not find service")}
	m := newManager(t, fr)
	st, err := m.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Installed || st.Loaded {
		t.Fatalf("status = %+v, want not installed, not loaded", st)
	}
}

func TestParseListOutput(t *testing.T) {
	pid, exit := parseListOutput("\t\"PID\" = 99;\n\t\"LastExitStatus\" = 2;\n")
	if pid != 99 || exit != 2 {
		t.Fatalf("got pid=%d exit=%d, want 99/2", pid, exit)
	}
}

func TestManager_StartFallbackKickstart(t *testing.T) {
	fr := &fakeRunner{results: []fakeResult{
		{err: errors.New("already loaded")}, // bootstrap fails
		{},                                  // kickstart succeeds
	}}
	m := newManager(t, fr)
	if err := os.MkdirAll(filepath.Dir(m.PlistPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m.PlistPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(fr.calls) != 2 || fr.calls[0][0] != "bootstrap" || fr.calls[1][0] != "kickstart" || fr.calls[1][1] != "gui/501/"+Label {
		t.Fatalf("calls = %v, want bootstrap then kickstart gui/501/%s", fr.calls, Label)
	}
}

func TestManager_RestartFallbackBootstrap(t *testing.T) {
	fr := &fakeRunner{results: []fakeResult{
		{err: errors.New("not loaded")}, // kickstart -k fails
		{},                              // bootstrap succeeds
	}}
	m := newManager(t, fr)
	if err := m.Restart(); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if len(fr.calls) != 2 || fr.calls[0][0] != "kickstart" || fr.calls[1][0] != "bootstrap" {
		t.Fatalf("calls = %v, want kickstart then bootstrap", fr.calls)
	}
}

func TestManager_RestartBothFailReturnsBootstrapError(t *testing.T) {
	bootErr := errors.New("bootstrap actionable error")
	fr := &fakeRunner{results: []fakeResult{
		{err: errors.New("kickstart error")}, // kickstart -k fails
		{err: bootErr},                       // bootstrap also fails
	}}
	m := newManager(t, fr)
	err := m.Restart()
	if err != bootErr {
		t.Fatalf("Restart err = %v, want the bootstrap error %v", err, bootErr)
	}
}

func TestManager_InstallAbsolutizesConfigPath(t *testing.T) {
	fr := &fakeRunner{}
	m := newManager(t, fr)
	if err := m.Install("/usr/local/bin/creddserver", "rel/config.toml"); err != nil {
		t.Fatalf("Install: %v", err)
	}
	want, err := filepath.Abs("rel/config.toml")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(m.PlistPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "<string>"+want+"</string>") {
		t.Fatalf("plist should contain absolute config path %q\n%s", want, data)
	}
	if strings.Contains(string(data), "<string>rel/config.toml</string>") {
		t.Fatal("plist still contains the relative path")
	}
}
