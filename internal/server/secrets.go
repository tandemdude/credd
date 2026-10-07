package server

import (
	"context"
	"errors"
	"log/slog"

	"github.com/tandemdude/credd/internal/secretstore"

	secretsv1 "github.com/tandemdude/credd/gen/go/secrets/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SecretSource looks up secrets by reference. It is satisfied by
// *secretstore.Registry.
type SecretSource interface {
	Exists(ctx context.Context, ref string) (bool, error)
	Get(ctx context.Context, ref string) (string, error)
}

type SecretsServer struct {
	secretsv1.UnimplementedSecretsServer

	secrets SecretSource
}

func NewSecretsServer(secrets SecretSource) *SecretsServer {
	return &SecretsServer{secrets: secrets}
}

// toStatus maps secretstore errors onto gRPC status codes; anything else is
// passed through unchanged.
func toStatus(err error) error {
	switch {
	case errors.Is(err, secretstore.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, secretstore.ErrInvalidReference), errors.Is(err, secretstore.ErrUnsupportedReference):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return err
	}
}

func (s *SecretsServer) GetSecret(ctx context.Context, req *secretsv1.GetSecretRequest) (*secretsv1.GetSecretResponse, error) {
	slog.InfoContext(ctx, "SecretsServer.GetSecret", "reference", req.GetReference())

	secret, err := s.secrets.Get(ctx, req.GetReference())
	if err != nil {
		return nil, toStatus(err)
	}
	return &secretsv1.GetSecretResponse{Secret: secret}, nil
}

func (s *SecretsServer) SecretExists(ctx context.Context, req *secretsv1.SecretExistsRequest) (*secretsv1.SecretExistsResponse, error) {
	slog.InfoContext(ctx, "SecretsServer.SecretExists", "reference", req.GetReference())

	exists, err := s.secrets.Exists(ctx, req.GetReference())
	if err != nil {
		return nil, toStatus(err)
	}
	return &secretsv1.SecretExistsResponse{Exists: exists}, nil
}
