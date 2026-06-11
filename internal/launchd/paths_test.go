package launchd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlistPath(t *testing.T) {
	t.Setenv("HOME", "/tmp/h")
	got, err := PlistPath()
	if err != nil {
		t.Fatalf("PlistPath: %v", err)
	}
	want := "/tmp/h/Library/LaunchAgents/io.github.tandemdude.creddserver.plist"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLogPath(t *testing.T) {
	t.Setenv("HOME", "/tmp/h")
	got, err := LogPath()
	if err != nil {
		t.Fatalf("LogPath: %v", err)
	}
	want := "/tmp/h/Library/Logs/creddserver.log"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveServerPath_Sibling(t *testing.T) {
	dir := t.TempDir()
	server := filepath.Join(dir, "creddserver")
	if err := os.WriteFile(server, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveServerPath(filepath.Join(dir, "credd"))
	if err != nil {
		t.Fatalf("ResolveServerPath: %v", err)
	}
	if got != server {
		t.Fatalf("got %q, want %q", got, server)
	}
}

func TestResolveServerPath_NotFound(t *testing.T) {
	t.Setenv("PATH", "")
	dir := t.TempDir()
	if _, err := ResolveServerPath(filepath.Join(dir, "credd")); err == nil {
		t.Fatal("expected error when creddserver is absent")
	}
}

func TestResolveServerPath_OnPATH(t *testing.T) {
	pathDir := t.TempDir()
	onPath := filepath.Join(pathDir, "creddserver")
	if err := os.WriteFile(onPath, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", pathDir)

	// selfExe is in a different dir with no sibling creddserver.
	got, err := ResolveServerPath(filepath.Join(t.TempDir(), "credd"))
	if err != nil {
		t.Fatalf("ResolveServerPath: %v", err)
	}
	if got != onPath {
		t.Fatalf("got %q, want %q", got, onPath)
	}
}
