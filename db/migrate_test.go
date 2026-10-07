package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"

	_ "modernc.org/sqlite"
)

// TestMigrateDropsSecretTable checks that an existing database which stored
// secrets locally is upgraded to the external-secret-manager-only schema.
func TestMigrateDropsSecretTable(t *testing.T) {
	conn, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Simulate a pre-existing install at the initial schema with a stored secret.
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("sqlite"); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpTo(conn, "src/migrations", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO secret (name, encrypted_value) VALUES ('foo', 'bar')`); err != nil {
		t.Fatal(err)
	}

	if err := Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var n int
	if err := conn.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'secret'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("expected secret table to be dropped")
	}
}
