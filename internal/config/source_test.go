package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTOML(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTOMLSource_FindsKey(t *testing.T) {
	path := writeTOML(t, "[server]\naddr = \"1.2.3.4:9000\"\n")
	src := TOMLSource("server.addr", &path)
	got, ok := src.Lookup()
	if !ok || got != "1.2.3.4:9000" {
		t.Fatalf("Lookup() = (%q, %v), want (\"1.2.3.4:9000\", true)", got, ok)
	}
}

func TestTOMLSource_SectionWithDigitName(t *testing.T) {
	path := writeTOML(t, "[1password]\naccount = \"Acme\"\n")
	src := TOMLSource("1password.account", &path)
	got, ok := src.Lookup()
	if !ok || got != "Acme" {
		t.Fatalf("Lookup() = (%q, %v), want (\"Acme\", true)", got, ok)
	}
}

func TestTOMLSource_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.toml")
	src := TOMLSource("server.addr", &path)
	if got, ok := src.Lookup(); ok {
		t.Fatalf("Lookup() = (%q, true), want (\"\", false) for missing file", got)
	}
}

func TestTOMLSource_MissingKey(t *testing.T) {
	path := writeTOML(t, "[server]\naddr = \"x\"\n")
	src := TOMLSource("1password.account", &path)
	if _, ok := src.Lookup(); ok {
		t.Fatal("Lookup() ok = true, want false for absent key")
	}
}

func TestTOMLSource_NilOrEmptyPath(t *testing.T) {
	empty := ""
	if _, ok := TOMLSource("server.addr", &empty).Lookup(); ok {
		t.Fatal("empty path should not resolve")
	}
	if _, ok := TOMLSource("server.addr", nil).Lookup(); ok {
		t.Fatal("nil path should not resolve")
	}
}

func TestTOMLSource_NonStringValue(t *testing.T) {
	path := writeTOML(t, "[server]\nport = 9000\n")
	src := TOMLSource("server.port", &path)
	if got, ok := src.Lookup(); ok {
		t.Fatalf("Lookup() = (%q, true), want (\"\", false) for non-string value", got)
	}
}
