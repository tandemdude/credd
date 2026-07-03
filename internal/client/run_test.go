package client

import (
	"context"
	"os"
	"reflect"
	"testing"
)

func TestParseTemplate(t *testing.T) {
	t.Run("no braces is a single whole-value ref", func(t *testing.T) {
		got, err := parseTemplate("bar")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []templatePart{{ref: "bar", isRef: true}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("op reference with no braces is a single ref", func(t *testing.T) {
		got, err := parseTemplate("op://Vault/item/field")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []templatePart{{ref: "op://Vault/item/field", isRef: true}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("literal text around a placeholder", func(t *testing.T) {
		got, err := parseTemplate("postgres://u:{op://V/S/P}@h/db")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []templatePart{
			{literal: "postgres://u:"},
			{ref: "op://V/S/P", isRef: true},
			{literal: "@h/db"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("adjacent placeholders", func(t *testing.T) {
		got, err := parseTemplate("{a}{b}")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []templatePart{
			{ref: "a", isRef: true},
			{ref: "b", isRef: true},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("whitespace inside a placeholder is trimmed", func(t *testing.T) {
		got, err := parseTemplate("{ op://V/S/P }")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []templatePart{{ref: "op://V/S/P", isRef: true}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("escaped braces become literal single braces", func(t *testing.T) {
		got, err := parseTemplate("a{{b}}c{x}")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []templatePart{
			{literal: "a{b}c"},
			{ref: "x", isRef: true},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("escaped braces only, no placeholder", func(t *testing.T) {
		got, err := parseTemplate("a{{b}}c")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []templatePart{{literal: "a{b}c"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("unmatched open brace is an error", func(t *testing.T) {
		if _, err := parseTemplate("a{b"); err == nil {
			t.Fatalf("expected error for unmatched '{'")
		}
	})

	t.Run("lone close brace is an error", func(t *testing.T) {
		if _, err := parseTemplate("a}b"); err == nil {
			t.Fatalf("expected error for lone '}'")
		}
	})

	t.Run("empty placeholder is an error", func(t *testing.T) {
		if _, err := parseTemplate("a{}b"); err == nil {
			t.Fatalf("expected error for empty placeholder")
		}
	})

	t.Run("whitespace-only placeholder is an error", func(t *testing.T) {
		if _, err := parseTemplate("a{   }b"); err == nil {
			t.Fatalf("expected error for whitespace-only placeholder")
		}
	})
}

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
