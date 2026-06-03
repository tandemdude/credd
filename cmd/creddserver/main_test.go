package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/urfave/cli/v3"
)

// resolveServerAddr builds the real server flags and runs the command with the
// given args, returning the resolved value of --server.addr.
func resolveServerAddr(t *testing.T, args []string) string {
	t.Helper()
	var configPath string
	var got string
	cmd := &cli.Command{
		Name:  "creddserver",
		Flags: serverFlags("/nonexistent/default/config.toml", &configPath),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			got = cmd.String("server.addr")
			return nil
		},
	}
	if err := cmd.Run(context.Background(), args); err != nil {
		t.Fatalf("cmd.Run: %v", err)
	}
	return got
}

func writeServerConfig(t *testing.T, addr string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	body := "[server]\naddr = \"" + addr + "\"\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestServerAddr_DefaultWhenNothingSet(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "none.toml")
	got := resolveServerAddr(t, []string{"creddserver", "--config", missing})
	if got != "127.0.0.1:50051" {
		t.Fatalf("got %q, want default 127.0.0.1:50051", got)
	}
}

func TestServerAddr_FileOverridesDefault(t *testing.T) {
	cfg := writeServerConfig(t, "file:1111")
	got := resolveServerAddr(t, []string{"creddserver", "--config", cfg})
	if got != "file:1111" {
		t.Fatalf("got %q, want file:1111", got)
	}
}

func TestServerAddr_EnvOverridesFile(t *testing.T) {
	cfg := writeServerConfig(t, "file:1111")
	t.Setenv("CREDD_SERVER_ADDR", "env:2222")
	got := resolveServerAddr(t, []string{"creddserver", "--config", cfg})
	if got != "env:2222" {
		t.Fatalf("got %q, want env:2222", got)
	}
}

func TestServerAddr_FlagOverridesEnvAndFile(t *testing.T) {
	cfg := writeServerConfig(t, "file:1111")
	t.Setenv("CREDD_SERVER_ADDR", "env:2222")
	got := resolveServerAddr(t, []string{"creddserver", "--config", cfg, "--server.addr", "flag:3333"})
	if got != "flag:3333" {
		t.Fatalf("got %q, want flag:3333", got)
	}
}
