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
	conn *sql.DB
	q    *db.Queries
}

// NewProfileRepository returns a sqlite-backed implementation of the profile repository.
func NewProfileRepository(conn *sql.DB) repository.ProfileRepository {
	return &profileRepository{conn: conn, q: db.New(conn)}
}

func profileID(ctx context.Context, q *db.Queries, name string) (int64, error) {
	id, err := q.GetProfileID(ctx, name)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("profile %q: %w", name, repository.ErrNotFound)
	}
	return id, err
}

func createProfile(ctx context.Context, q *db.Queries, name, description string) error {
	rows, err := q.CreateProfile(ctx, db.CreateProfileParams{Name: name, Description: description})
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("profile %q: %w", name, repository.ErrAlreadyExists)
	}
	return nil
}

func (r *profileRepository) CreateProfile(ctx context.Context, name, description string) error {
	return createProfile(ctx, r.q, name, description)
}

func (r *profileRepository) ListProfiles(ctx context.Context) ([]models.Profile, error) {
	rows, err := r.q.ListProfiles(ctx)
	if err != nil {
		return nil, err
	}

	profiles := make([]models.Profile, len(rows))
	for i, row := range rows {
		profiles[i] = models.Profile{Name: row.Name, Description: row.Description}
	}
	return profiles, nil
}

func (r *profileRepository) GetProfile(ctx context.Context, name string) (models.Profile, error) {
	p, err := r.q.GetProfile(ctx, name)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Profile{}, fmt.Errorf("profile %q: %w", name, repository.ErrNotFound)
	}
	if err != nil {
		return models.Profile{}, err
	}

	rows, err := r.q.ListProfileVars(ctx, p.ID)
	if err != nil {
		return models.Profile{}, err
	}

	vars := make([]models.ProfileVar, len(rows))
	for i, row := range rows {
		vars[i] = models.ProfileVar{Name: row.Name, Value: row.Value, Kind: models.VarKind(row.Kind)}
	}
	return models.Profile{Name: name, Description: p.Description, Vars: vars}, nil
}

func (r *profileRepository) SetProfileDescription(ctx context.Context, name, description string) error {
	rows, err := r.q.UpdateProfileDescription(ctx, db.UpdateProfileDescriptionParams{Description: description, Name: name})
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("profile %q: %w", name, repository.ErrNotFound)
	}
	return nil
}

func (r *profileRepository) ImportProfile(ctx context.Context, p models.Profile, overwrite bool) error {
	tx, err := r.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := r.q.WithTx(tx)

	if overwrite {
		// Deleting cascades to the old vars, so the import fully replaces them.
		if _, err := q.DeleteProfile(ctx, p.Name); err != nil {
			return err
		}
	}
	if err := createProfile(ctx, q, p.Name, p.Description); err != nil {
		return err
	}
	id, err := profileID(ctx, q, p.Name)
	if err != nil {
		return err
	}
	for _, v := range p.Vars {
		if err := q.UpsertProfileVar(ctx, db.UpsertProfileVarParams{
			ProfileID: id,
			Name:      v.Name,
			Value:     v.Value,
			Kind:      string(v.Kind),
		}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *profileRepository) SetProfileVar(ctx context.Context, profile string, v models.ProfileVar) error {
	id, err := profileID(ctx, r.q, profile)
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
	id, err := profileID(ctx, r.q, profile)
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
