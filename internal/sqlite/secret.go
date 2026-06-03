package sqlite

import (
	"context"

	"github.com/tandemdude/credd/db"
	"github.com/tandemdude/credd/internal/domain/models"
	"github.com/tandemdude/credd/internal/domain/repository"
)

type secretRepository struct {
	q *db.Queries
}

// NewSecretRepository returns a sqlite-backed implementation of the secret repository.
func NewSecretRepository(q *db.Queries) repository.SecretRepository {
	return &secretRepository{q: q}
}

func (r *secretRepository) CreateSecret(ctx context.Context, secret models.Secret) error {
	// TODO - encrypt
	return r.q.CreateSecret(ctx, db.CreateSecretParams{
		Name:           secret.Name,
		EncryptedValue: secret.Value,
	})
}

func (r *secretRepository) ListSecrets(ctx context.Context) ([]models.Secret, error) {
	secrets, err := r.q.ListSecrets(ctx)
	if err != nil {
		return nil, err
	}

	// TODO - decrypt
	mappedSecrets := make([]models.Secret, len(secrets))
	for i, secret := range secrets {
		mappedSecrets[i] = models.Secret{Name: secret.Name, Value: secret.EncryptedValue}
	}
	return mappedSecrets, nil
}

func (r *secretRepository) GetSecret(ctx context.Context, name string) (models.Secret, error) {
	secret, err := r.q.GetSecret(ctx, name)
	if err != nil {
		return models.Secret{}, err
	}

	// TODO - decrypt
	return models.Secret{
		Name:  secret.Name,
		Value: secret.EncryptedValue,
	}, nil
}

func (r *secretRepository) DeleteSecret(ctx context.Context, name string) (bool, error) {
	rows, err := r.q.DeleteSecret(ctx, name)
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *secretRepository) DeleteAllSecrets(ctx context.Context) error {
	return r.q.DeleteAllSecrets(ctx)
}
