package client

import (
	"context"
	"os"
	"testing"
)

func TestParseEnvSpec(t *testing.T) {
	t.Run("valid pairs", func(t *testing.T) {
		got, err := parseEnvSpec([]string{"FOO=bar", "BAZ=op://Vault/item/field"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("got %d vars, want 2", len(got))
		}
		if got[0] != (EnvVar{Name: "FOO", Ref: "bar"}) {
			t.Fatalf("got[0]=%+v", got[0])
		}
		if got[1] != (EnvVar{Name: "BAZ", Ref: "op://Vault/item/field"}) {
			t.Fatalf("got[1]=%+v", got[1])
		}
	})

	t.Run("ref containing equals sign", func(t *testing.T) {
		got, err := parseEnvSpec([]string{"TOKEN=a=b=c"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got[0] != (EnvVar{Name: "TOKEN", Ref: "a=b=c"}) {
			t.Fatalf("got[0]=%+v", got[0])
		}
	})

	t.Run("missing equals", func(t *testing.T) {
		if _, err := parseEnvSpec([]string{"FOO"}); err == nil {
			t.Fatalf("expected error for missing '='")
		}
	})

	t.Run("empty name", func(t *testing.T) {
		if _, err := parseEnvSpec([]string{"=bar"}); err == nil {
			t.Fatalf("expected error for empty name")
		}
	})
}

func TestRunProcess(t *testing.T) {
	ctx := context.Background()

	t.Run("success exit 0", func(t *testing.T) {
		code, err := runProcess(ctx, os.Environ(), []string{"sh", "-c", "exit 0"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if code != 0 {
			t.Fatalf("got exit code %d, want 0", code)
		}
	})

	t.Run("non-zero exit code propagates", func(t *testing.T) {
		code, err := runProcess(ctx, os.Environ(), []string{"sh", "-c", "exit 3"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if code != 3 {
			t.Fatalf("got exit code %d, want 3", code)
		}
	})

	t.Run("injected env is visible to child", func(t *testing.T) {
		env := append(os.Environ(), "credd_TEST_VAR=hello")
		code, err := runProcess(ctx, env, []string{"sh", "-c", `test "$credd_TEST_VAR" = hello`})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if code != 0 {
			t.Fatalf("env not injected: child exited %d", code)
		}
	})

	t.Run("command not found is an error", func(t *testing.T) {
		if _, err := runProcess(ctx, os.Environ(), []string{"credd-nonexistent-cmd-xyz"}); err == nil {
			t.Fatalf("expected error for missing command")
		}
	})

	t.Run("later env value overrides earlier", func(t *testing.T) {
		env := append(os.Environ(), "credd_OVERRIDE=original", "credd_OVERRIDE=replaced")
		code, err := runProcess(ctx, env, []string{"sh", "-c", `test "$credd_OVERRIDE" = replaced`})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if code != 0 {
			t.Fatalf("override not last-wins: child exited %d", code)
		}
	})
}
