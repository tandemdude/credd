package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultDir_CreddHomeWins(t *testing.T) {
	t.Setenv("CREDD_HOME", "/tmp/credd-home")
	got, err := DefaultDir()
	if err != nil {
		t.Fatalf("DefaultDir: %v", err)
	}
	if got != "/tmp/credd-home" {
		t.Fatalf("got %q, want /tmp/credd-home", got)
	}
}

func TestDefaultDir_FallsBackToHome(t *testing.T) {
	t.Setenv("CREDD_HOME", "")
	t.Setenv("HOME", "/tmp/fake-home")
	got, err := DefaultDir()
	if err != nil {
		t.Fatalf("DefaultDir: %v", err)
	}
	want := filepath.Join("/tmp/fake-home", ".credd")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDefaultConfigPath(t *testing.T) {
	t.Setenv("CREDD_HOME", "/tmp/credd-home")
	got, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath: %v", err)
	}
	want := filepath.Join("/tmp/credd-home", "config.toml")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDataDBPath(t *testing.T) {
	got := DataDBPath("/tmp/credd-home/config.toml")
	want := "/tmp/credd-home/data.db"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriteLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.toml") // sub dir must be created
	in := Config{
		Server:      ServerConfig{Addr: "127.0.0.1:50051"},
		OnePassword: OnePasswordConfig{Account: "Acme Inc"},
	}
	if err := Write(path, in); err != nil {
		t.Fatalf("Write: %v", err)
	}

	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if out != in {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", out, in)
	}
}

func TestWritePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nest", "config.toml")
	if err := Write(path, Config{}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("file perm = %o, want 600", perm)
	}
	di, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := di.Mode().Perm(); perm != 0o700 {
		t.Fatalf("dir perm = %o, want 700", perm)
	}
}

func TestValidate_MissingFileOK(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.toml")
	if err := Validate(path); err != nil {
		t.Fatalf("Validate(missing) = %v, want nil", err)
	}
}

func TestValidate_MalformedErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.toml")
	if err := os.WriteFile(path, []byte("this is = = not toml"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Validate(path); err == nil {
		t.Fatal("Validate(malformed) = nil, want error")
	}
}
