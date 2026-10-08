package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/tandemdude/credd/db"
	"github.com/tandemdude/credd/internal/domain/models"
	"github.com/tandemdude/credd/internal/domain/repository"
)

type profileRepository struct {
	q *db.Queries
}

// NewProfileRepository returns a sqlite-backed implementation of the profile repository.
func NewProfileRepository(q *db.Queries) repository.ProfileRepository {
	return &profileRepository{q: q}
}

func (r *profileRepository) profileID(ctx context.Context, name string) (int64, error) {
	id, err := r.q.GetProfileID(ctx, name)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("profile %q: %w", name, repository.ErrNotFound)
	}
	return id, err
}

func (r *profileRepository) CreateProfile(ctx context.Context, name string) error {
	rows, err := r.q.CreateProfile(ctx, name)
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("profile %q: %w", name, repository.ErrAlreadyExists)
	}
	return nil
}

func (r *profileRepository) ListProfiles(ctx context.Context) ([]string, error) {
	return r.q.ListProfileNames(ctx)
}

func (r *profileRepository) GetProfile(ctx context.Context, name string) (models.Profile, error) {
	id, err := r.profileID(ctx, name)
	if err != nil {
		return models.Profile{}, err
	}

	rows, err := r.q.ListProfileVars(ctx, id)
	if err != nil {
		return models.Profile{}, err
	}

	vars := make([]models.ProfileVar, len(rows))
	for i, row := range rows {
		vars[i] = models.ProfileVar{Name: row.Name, Value: row.Value, Kind: models.VarKind(row.Kind)}
	}
	return models.Profile{Name: name, Vars: vars}, nil
}

func (r *profileRepository) SetProfileVar(ctx context.Context, profile string, v models.ProfileVar) error {
	id, err := r.profileID(ctx, profile)
	if err != nil {
		return err
	}

	return r.q.UpsertProfileVar(ctx, db.UpsertProfileVarParams{
		ProfileID: id,
		Name:      v.Name,
		Value:     v.Value,
		Kind:      string(v.Kind),
	})
}

func (r *profileRepository) UnsetProfileVar(ctx context.Context, profile, name string) error {
	id, err := r.profileID(ctx, profile)
	if err != nil {
		return err
	}

	rows, err := r.q.DeleteProfileVar(ctx, db.DeleteProfileVarParams{ProfileID: id, Name: name})
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("var %q in profile %q: %w", name, profile, repository.ErrNotFound)
	}
	return nil
}

func (r *profileRepository) DeleteProfile(ctx context.Context, name string) error {
	rows, err := r.q.DeleteProfile(ctx, name)
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("profile %q: %w", name, repository.ErrNotFound)
	}
	return nil
}
