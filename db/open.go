package db

import (
	"database/sql"
	"fmt"
)

// Open opens the sqlite database at path with foreign key enforcement enabled
// (required for ON DELETE CASCADE) and applies any pending migrations.
func Open(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err = Migrate(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	return conn, nil
}
