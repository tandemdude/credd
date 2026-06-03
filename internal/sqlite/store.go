package sqlite

import (
	"github.com/tandemdude/credd/db"
	"github.com/tandemdude/credd/internal/domain/repository"
)

// Store bundles the sqlite-backed repositories, all sharing a single
// *db.Queries handle. Add new repositories here as they are introduced so
// callers wire one Store instead of threading each repository individually.
type Store struct {
	Secrets repository.SecretRepository
}

// NewStore constructs the repository bundle from a sqlc Queries handle.
func NewStore(q *db.Queries) *Store {
	return &Store{
		Secrets: NewSecretRepository(q),
	}
}
