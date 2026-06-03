package server

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"regexp"
	"sync"

	"github.com/tandemdude/credd/internal/domain/models"
	"github.com/tandemdude/credd/internal/domain/repository"
	"github.com/tandemdude/credd/internal/opwd"

	secretsv1 "github.com/tandemdude/credd/gen/go/secrets/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type opFactoryFn = func(ctx context.Context) (*opwd.Client, error)

type SecretsServer struct {
	secretsv1.UnimplementedSecretsServer

	secretRepo repository.SecretRepository

	opFn opFactoryFn
	opMu sync.Mutex
	op   *opwd.Client
}

func NewSecretsServer(secretRepo repository.SecretRepository, opFn opFactoryFn) *SecretsServer {
	return &SecretsServer{
		secretRepo: secretRepo,
		opFn:       opFn,
	}
}

func (s *SecretsServer) getOp(ctx context.Context) (*opwd.Client, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	if s.op == nil {
		op, err := s.opFn(ctx)
		if err != nil {
			return nil, err
		}
		s.op = op
	}
	return s.op, nil
}

var secretNameRe = regexp.MustCompile(`^[0-9a-zA-Z_-]+$`)

func validateCreateSecret(name, value string) error {
	if !secretNameRe.MatchString(name) {
		return status.Errorf(codes.InvalidArgument, "invalid secret name %q: must match [0-9a-zA-Z_-]+", name)
	}
	if opwd.IsOpwdRef(value) {
		return status.Error(codes.InvalidArgument, "secret value must not be a 1Password reference (op://...)")
	}
	return nil
}

func (s *SecretsServer) CreateSecret(ctx context.Context, req *secretsv1.CreateSecretRequest) (*secretsv1.CreateSecretResponse, error) {
	if err := validateCreateSecret(req.GetName(), req.GetValue()); err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "SecretsServer.CreateSecret", "reference", req.GetName())
	return &secretsv1.CreateSecretResponse{}, s.secretRepo.CreateSecret(ctx, models.Secret{
		Name:  req.Name,
		Value: req.Value,
	})
}

func (s *SecretsServer) ListSecrets(ctx context.Context, _ *secretsv1.ListSecretsRequest) (*secretsv1.ListSecretsResponse, error) {
	slog.InfoContext(ctx, "SecretsServer.ListSecrets")
	secrets, err := s.secretRepo.ListSecrets(ctx)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(secrets))
	for i, secret := range secrets {
		names[i] = secret.Name
	}

	return &secretsv1.ListSecretsResponse{Name: names}, nil
}

func (s *SecretsServer) GetSecret(ctx context.Context, req *secretsv1.GetSecretRequest) (*secretsv1.GetSecretResponse, error) {
	var secret string
	var err error

	if opwd.IsOpwdRef(req.GetReference()) {
		slog.InfoContext(ctx, "SecretsServer.GetSecret", "reference", req.GetReference(), "source", "OnePassword")
		var op *opwd.Client

		op, err = s.getOp(ctx)
		if err != nil {
			return nil, err
		}

		secret, err = op.ResolveSecret(ctx, req.GetReference())
	} else {
		slog.InfoContext(ctx, "SecretsServer.GetSecret", "reference", req.GetReference(), "source", "DB")
		var rawSecret models.Secret

		rawSecret, err = s.secretRepo.GetSecret(ctx, req.GetReference())
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "secret not found")
		}

		secret = rawSecret.Value
	}

	if err != nil {
		return nil, err
	}
	return &secretsv1.GetSecretResponse{Secret: secret}, nil
}

func (s *SecretsServer) DeleteSecret(ctx context.Context, req *secretsv1.DeleteSecretRequest) (*secretsv1.DeleteSecretResponse, error) {
	slog.InfoContext(ctx, "SecretsServer.DeleteSecret", "name", req.GetName())

	existed, err := s.secretRepo.DeleteSecret(ctx, req.GetName())
	if err != nil {
		return nil, err
	}
	if !existed {
		return nil, status.Error(codes.NotFound, "secret not found")
	}

	return &secretsv1.DeleteSecretResponse{}, nil
}

func (s *SecretsServer) DeleteAllSecrets(ctx context.Context, _ *secretsv1.DeleteAllSecretsRequest) (*secretsv1.DeleteAllSecretsResponse, error) {
	slog.InfoContext(ctx, "SecretsServer.DeleteAllSecrets")
	return &secretsv1.DeleteAllSecretsResponse{}, s.secretRepo.DeleteAllSecrets(ctx)
}
