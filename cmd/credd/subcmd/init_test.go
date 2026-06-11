package subcmd

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tandemdude/credd/internal/config"
)

func scannerFor(s string) *bufio.Scanner { return bufio.NewScanner(strings.NewReader(s)) }

func TestRunInit_Defaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	var out bytes.Buffer
	wrote, err := runInit(scannerFor("\n\n"), &out, path)
	if err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if !wrote {
		t.Fatal("wrote = false, want true")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Addr != "127.0.0.1:50051" {
		t.Fatalf("addr = %q, want default", cfg.Server.Addr)
	}
	if cfg.OnePassword.Account != "" {
		t.Fatalf("account = %q, want empty", cfg.OnePassword.Account)
	}
}

func TestRunInit_CustomValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	var out bytes.Buffer
	wrote, err := runInit(scannerFor("10.0.0.1:9000\nAcme Inc\n"), &out, path)
	if err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if !wrote {
		t.Fatal("wrote = false, want true")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Addr != "10.0.0.1:9000" {
		t.Fatalf("addr = %q, want 10.0.0.1:9000", cfg.Server.Addr)
	}
	if cfg.OnePassword.Account != "Acme Inc" {
		t.Fatalf("account = %q, want Acme Inc", cfg.OnePassword.Account)
	}
}

func TestRunInit_OverwriteDeclinedLeavesFileUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := []byte("[server]\naddr = \"keep:me\"\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	wrote, err := runInit(scannerFor("n\n"), &out, path)
	if err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if wrote {
		t.Fatal("wrote = true, want false on declined overwrite")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("file was modified: %q", got)
	}
}

func TestRunInit_OverwriteConfirmedRewritesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[server]\naddr = \"old:1\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	wrote, err := runInit(scannerFor("y\n10.0.0.1:9000\nAcme Inc\n"), &out, path)
	if err != nil {
		t.Fatalf("runInit: %v", err)
	}
	if !wrote {
		t.Fatal("wrote = false, want true")
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Addr != "10.0.0.1:9000" {
		t.Fatalf("addr = %q, want 10.0.0.1:9000", cfg.Server.Addr)
	}
}

func TestPromptInstallService_Declined(t *testing.T) {
	var out bytes.Buffer
	called := false
	install := func(string) error { called = true; return nil }
	if err := promptInstallService(scannerFor("n\n"), &out, "/c.toml", install); err != nil {
		t.Fatalf("promptInstallService: %v", err)
	}
	if called {
		t.Fatal("install should not be called when declined")
	}
}

func TestPromptInstallService_Accepted(t *testing.T) {
	var out bytes.Buffer
	var gotConfig string
	install := func(cfg string) error { gotConfig = cfg; return nil }
	if err := promptInstallService(scannerFor("y\n"), &out, "/c.toml", install); err != nil {
		t.Fatalf("promptInstallService: %v", err)
	}
	if gotConfig != "/c.toml" {
		t.Fatalf("install called with %q, want /c.toml", gotConfig)
	}
}

func TestPromptInstallService_InstallErrorPropagates(t *testing.T) {
	var out bytes.Buffer
	sentinel := errors.New("install failed")
	install := func(string) error { return sentinel }
	if err := promptInstallService(scannerFor("y\n"), &out, "/c.toml", install); err != sentinel {
		t.Fatalf("promptInstallService err = %v, want %v", err, sentinel)
	}
}
