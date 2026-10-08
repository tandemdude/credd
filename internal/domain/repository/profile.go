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
	CreateProfile(ctx context.Context, name string) error
	ListProfiles(ctx context.Context) ([]string, error)
	// GetProfile returns the profile with its vars sorted by name.
	GetProfile(ctx context.Context, name string) (models.Profile, error)
	// SetProfileVar creates or replaces the var v.Name within profile.
	SetProfileVar(ctx context.Context, profile string, v models.ProfileVar) error
	UnsetProfileVar(ctx context.Context, profile, name string) error
	DeleteProfile(ctx context.Context, name string) error
}
