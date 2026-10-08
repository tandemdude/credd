package sqlite

import (
	"database/sql"

	"github.com/tandemdude/credd/internal/domain/repository"
)

// Store bundles the sqlite-backed repositories, all sharing a single database
// connection. Add new repositories here as they are introduced so callers wire
// one Store instead of threading each repository individually.
type Store struct {
	Profiles repository.ProfileRepository
}

// NewStore constructs the repository bundle from a database connection.
func NewStore(conn *sql.DB) *Store {
	return &Store{
		Profiles: NewProfileRepository(conn),
	}
}
