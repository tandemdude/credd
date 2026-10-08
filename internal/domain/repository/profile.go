package repository

import (
	"context"
	"errors"

	"github.com/tandemdude/credd/internal/domain/models"
)

var (
	// ErrNotFound is returned (possibly wrapped) when a profile or profile var
	// does not exist.
	ErrNotFound = errors.New("not found")
	// ErrAlreadyExists is returned (possibly wrapped) when creating a profile
	// whose name is already taken.
	ErrAlreadyExists = errors.New("already exists")
)

type ProfileRepository interface {
	CreateProfile(ctx context.Context, name, description string) error
	// ListProfiles returns every profile sorted by name, without their vars.
	ListProfiles(ctx context.Context) ([]models.Profile, error)
	// GetProfile returns the profile with its vars sorted by name.
	GetProfile(ctx context.Context, name string) (models.Profile, error)
	SetProfileDescription(ctx context.Context, name, description string) error
	// ImportProfile atomically creates profile p with all of its vars. If a
	// profile with the same name exists it is replaced when overwrite is true,
	// otherwise ErrAlreadyExists is returned and nothing is changed.
	ImportProfile(ctx context.Context, p models.Profile, overwrite bool) error
	// SetProfileVar creates or replaces the var v.Name within profile.
	SetProfileVar(ctx context.Context, profile string, v models.ProfileVar) error
	UnsetProfileVar(ctx context.Context, profile, name string) error
	DeleteProfile(ctx context.Context, name string) error
}
