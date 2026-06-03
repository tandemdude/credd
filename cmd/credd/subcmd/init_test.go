package subcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tandemdude/credd/internal/config"
)

func TestRunInit_Defaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	var out bytes.Buffer
	// Two blank lines: accept default addr, skip account.
	if err := runInit(strings.NewReader("\n\n"), &out, path); err != nil {
		t.Fatalf("runInit: %v", err)
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
	if err := runInit(strings.NewReader("10.0.0.1:9000\nAcme Inc\n"), &out, path); err != nil {
		t.Fatalf("runInit: %v", err)
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
	// "n" declines the overwrite prompt.
	if err := runInit(strings.NewReader("n\n"), &out, path); err != nil {
		t.Fatalf("runInit: %v", err)
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
	original := []byte("[server]\naddr = \"old:1\"\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	// "y" confirms overwrite, then new addr, then account.
	if err := runInit(strings.NewReader("y\n10.0.0.1:9000\nAcme Inc\n"), &out, path); err != nil {
		t.Fatalf("runInit: %v", err)
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
