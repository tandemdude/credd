package repository

import (
	"context"

	"github.com/tandemdude/credd/internal/domain/models"
)

type SecretRepository interface {
	CreateSecret(ctx context.Context, secret models.Secret) error
	ListSecrets(ctx context.Context) ([]models.Secret, error)
	GetSecret(ctx context.Context, name string) (models.Secret, error)
	DeleteSecret(ctx context.Context, name string) (bool, error)
	DeleteAllSecrets(ctx context.Context) error
}
