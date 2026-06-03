package server

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	secretsv1 "github.com/tandemdude/credd/gen/go/secrets/v1"
	"github.com/tandemdude/credd/internal/domain/models"
	"github.com/tandemdude/credd/internal/domain/repository"
)

// fakeSecretRepo implements repository.SecretRepository for tests.
// Unimplemented methods are provided by the embedded interface (they panic if called).
type fakeSecretRepo struct {
	repository.SecretRepository
	getSecret func(ctx context.Context, name string) (models.Secret, error)
}

func (f *fakeSecretRepo) GetSecret(ctx context.Context, name string) (models.Secret, error) {
	return f.getSecret(ctx, name)
}

func TestGetSecretPropagatesRepoError(t *testing.T) {
	wantErr := errors.New("not found")
	srv := NewSecretsServer(&fakeSecretRepo{
		getSecret: func(ctx context.Context, name string) (models.Secret, error) {
			return models.Secret{}, wantErr
		},
	}, nil)

	_, err := srv.GetSecret(context.Background(), &secretsv1.GetSecretRequest{Reference: "mykey"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error to propagate, got %v", err)
	}
}

func TestGetSecretReturnsValueOnSuccess(t *testing.T) {
	srv := NewSecretsServer(&fakeSecretRepo{
		getSecret: func(ctx context.Context, name string) (models.Secret, error) {
			return models.Secret{Name: name, Value: "s3cret"}, nil
		},
	}, nil)

	resp, err := srv.GetSecret(context.Background(), &secretsv1.GetSecretRequest{Reference: "mykey"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetSecret() != "s3cret" {
		t.Fatalf("got %q, want s3cret", resp.GetSecret())
	}
}

func TestValidateCreateSecret(t *testing.T) {
	cases := []struct {
		name       string
		secretName string
		value      string
		wantErr    bool
	}{
		{"valid simple", "FOO", "bar", false},
		{"valid all classes", "Foo_Bar-123", "anything goes here", false},
		{"valid single char", "A", "bar", false},
		{"empty name", "", "bar", true},
		{"space in name", "foo bar", "bar", true},
		{"slash in name", "foo/bar", "bar", true},
		{"dot in name", "foo.bar", "bar", true},
		{"value is op ref", "FOO", "op://Vault/item/field", true},
		{"value merely contains op", "FOO", "not-op://x", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCreateSecret(tc.secretName, tc.value)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tc.wantErr {
				if status.Code(err) != codes.InvalidArgument {
					t.Fatalf("expected InvalidArgument, got %v", status.Code(err))
				}
				if status.Convert(err).Message() == "" {
					t.Fatalf("expected non-empty error message")
				}
			}
		})
	}
}
