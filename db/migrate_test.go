package db

import (
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpenEnablesForeignKeys(t *testing.T) {
	conn, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	var enabled int
	if err := conn.QueryRow(`PRAGMA foreign_keys`).Scan(&enabled); err != nil {
		t.Fatal(err)
	}
	if enabled != 1 {
		t.Fatal("expected foreign keys to be enabled")
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	for range 2 {
		conn, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		conn.Close()
	}
}
