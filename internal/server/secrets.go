package server

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"sync"

	"github.com/tandemdude/credd/internal/domain/models"
	"github.com/tandemdude/credd/internal/domain/repository"
	"github.com/tandemdude/credd/internal/opwd"

	secretsv1 "github.com/tandemdude/credd/gen/go/secrets/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SecretResolver resolves a 1Password reference to its secret value. It is
// satisfied by *opwd.Client and exists so the server can be tested without the
// 1Password SDK.
type SecretResolver interface {
	ResolveSecret(ctx context.Context, reference string) (string, error)
}

type opFactoryFn = func(ctx context.Context) (SecretResolver, error)

type SecretsServer struct {
	secretsv1.UnimplementedSecretsServer

	secretRepo repository.SecretRepository

	opFn  opFactoryFn
	opMu  sync.Mutex
	op    SecretResolver
	opGen uint64 // bumps each time a new client is built; guards concurrent invalidation
}

func NewSecretsServer(secretRepo repository.SecretRepository, opFn opFactoryFn) *SecretsServer {
	return &SecretsServer{
		secretRepo: secretRepo,
		opFn:       opFn,
	}
}

func (s *SecretsServer) getOp(ctx context.Context) (SecretResolver, uint64, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	if s.op == nil {
		op, err := s.opFn(ctx)
		if err != nil {
			return nil, 0, err
		}
		s.op = op
		s.opGen++
	}
	return s.op, s.opGen, nil
}

// invalidateOp drops the cached client so the next getOp rebuilds it, but only
// if no one has already replaced the generation the caller observed — otherwise
// a concurrent rebuild would be thrown away.
func (s *SecretsServer) invalidateOp(gen uint64) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	if gen == s.opGen {
		s.op = nil
	}
}

// isStaleClient reports whether err indicates the 1Password desktop app has
// invalidated our client (e.g. after an app restart, update, or sleep/wake).
// The SDK surfaces this as an untyped error, so a string match is the only
// signal available; see internal/opwd and the onepassword-sdk-go errors.go
// default case.
func isStaleClient(err error) bool {
	return err != nil && strings.Contains(err.Error(), "invalid client id")
}

// resolveSecret resolves a 1Password reference, transparently rebuilding the
// client once if the desktop app has invalidated it.
func (s *SecretsServer) resolveSecret(ctx context.Context, reference string) (string, error) {
	op, gen, err := s.getOp(ctx)
	if err != nil {
		return "", err
	}

	secret, err := op.ResolveSecret(ctx, reference)
	if err == nil || !isStaleClient(err) {
		return secret, err
	}

	slog.WarnContext(ctx, "1Password client invalidated, recreating", "err", err)
	s.invalidateOp(gen)

	op, _, err = s.getOp(ctx)
	if err != nil {
		return "", err
	}
	return op.ResolveSecret(ctx, reference)
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
		secret, err = s.resolveSecret(ctx, req.GetReference())
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
