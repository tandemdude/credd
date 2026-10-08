package server

import (
	"context"
	"errors"
	"log/slog"
	"regexp"

	"github.com/tandemdude/credd/internal/domain/models"
	"github.com/tandemdude/credd/internal/domain/repository"
	"github.com/tandemdude/credd/internal/envtemplate"

	profilesv1 "github.com/tandemdude/credd/gen/go/profiles/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ProfilesServer struct {
	profilesv1.UnimplementedProfilesServer

	profileRepo repository.ProfileRepository
	// isSecretRef reports whether a reference belongs to a configured secret
	// store; satisfied by (*secretstore.Registry).Supports.
	isSecretRef func(ref string) bool
}

func NewProfilesServer(profileRepo repository.ProfileRepository, isSecretRef func(ref string) bool) *ProfilesServer {
	return &ProfilesServer{
		profileRepo: profileRepo,
		isSecretRef: isSecretRef,
	}
}

var (
	profileNameRe = regexp.MustCompile(`^[0-9a-zA-Z_-]+$`)
	varNameRe     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

func validateProfileName(name string) error {
	if !profileNameRe.MatchString(name) {
		return status.Errorf(codes.InvalidArgument, "invalid profile name %q: must match [0-9a-zA-Z_-]+", name)
	}
	return nil
}

func validateVarName(name string) error {
	if !varNameRe.MatchString(name) {
		return status.Errorf(codes.InvalidArgument, "invalid env var name %q: must match [A-Za-z_][A-Za-z0-9_]*", name)
	}
	return nil
}

// detectKind classifies a var value. It is a secret if it is a supported
// secret reference (e.g. op://vault/item/field) or a template containing at
// least one; anything else, including text that merely contains braces, is a
// plain value used verbatim.
func detectKind(value string, isSecretRef func(ref string) bool) models.VarKind {
	parts, err := envtemplate.Parse(value)
	if err != nil {
		return models.VarKindPlain
	}
	for _, ref := range envtemplate.Refs(parts) {
		if isSecretRef(ref) {
			return models.VarKindSecret
		}
	}
	return models.VarKindPlain
}

// repoErrToStatus maps repository errors onto gRPC status codes; anything else
// is passed through unchanged.
func repoErrToStatus(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, repository.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		return err
	}
}

func toProtoVar(v models.ProfileVar) *profilesv1.ProfileVar {
	kind := profilesv1.VarKind_VAR_KIND_PLAIN
	if v.Kind == models.VarKindSecret {
		kind = profilesv1.VarKind_VAR_KIND_SECRET
	}
	return &profilesv1.ProfileVar{Name: v.Name, Value: v.Value, Kind: kind}
}

func (s *ProfilesServer) CreateProfile(ctx context.Context, req *profilesv1.CreateProfileRequest) (*profilesv1.CreateProfileResponse, error) {
	slog.InfoContext(ctx, "ProfilesServer.CreateProfile", "profile", req.GetName())
	if err := validateProfileName(req.GetName()); err != nil {
		return nil, err
	}
	if err := s.profileRepo.CreateProfile(ctx, req.GetName()); err != nil {
		return nil, repoErrToStatus(err)
	}
	return &profilesv1.CreateProfileResponse{}, nil
}

func (s *ProfilesServer) ListProfiles(ctx context.Context, _ *profilesv1.ListProfilesRequest) (*profilesv1.ListProfilesResponse, error) {
	slog.InfoContext(ctx, "ProfilesServer.ListProfiles")
	names, err := s.profileRepo.ListProfiles(ctx)
	if err != nil {
		return nil, err
	}
	return &profilesv1.ListProfilesResponse{Names: names}, nil
}

func (s *ProfilesServer) GetProfile(ctx context.Context, req *profilesv1.GetProfileRequest) (*profilesv1.GetProfileResponse, error) {
	slog.InfoContext(ctx, "ProfilesServer.GetProfile", "profile", req.GetName())
	p, err := s.profileRepo.GetProfile(ctx, req.GetName())
	if err != nil {
		return nil, repoErrToStatus(err)
	}

	vars := make([]*profilesv1.ProfileVar, len(p.Vars))
	for i, v := range p.Vars {
		vars[i] = toProtoVar(v)
	}
	return &profilesv1.GetProfileResponse{Profile: &profilesv1.Profile{Name: p.Name, Vars: vars}}, nil
}

func (s *ProfilesServer) SetProfileVar(ctx context.Context, req *profilesv1.SetProfileVarRequest) (*profilesv1.SetProfileVarResponse, error) {
	if err := validateVarName(req.GetName()); err != nil {
		return nil, err
	}

	v := models.ProfileVar{
		Name:  req.GetName(),
		Value: req.GetValue(),
		Kind:  detectKind(req.GetValue(), s.isSecretRef),
	}
	// Plain values are logged by kind only; they are not secret, but there is
	// no need to put them in the logs either.
	slog.InfoContext(ctx, "ProfilesServer.SetProfileVar", "profile", req.GetProfile(), "name", v.Name, "kind", v.Kind)

	if err := s.profileRepo.SetProfileVar(ctx, req.GetProfile(), v); err != nil {
		return nil, repoErrToStatus(err)
	}
	return &profilesv1.SetProfileVarResponse{Var: toProtoVar(v)}, nil
}

func (s *ProfilesServer) UnsetProfileVar(ctx context.Context, req *profilesv1.UnsetProfileVarRequest) (*profilesv1.UnsetProfileVarResponse, error) {
	slog.InfoContext(ctx, "ProfilesServer.UnsetProfileVar", "profile", req.GetProfile(), "name", req.GetName())
	if err := s.profileRepo.UnsetProfileVar(ctx, req.GetProfile(), req.GetName()); err != nil {
		return nil, repoErrToStatus(err)
	}
	return &profilesv1.UnsetProfileVarResponse{}, nil
}

func (s *ProfilesServer) DeleteProfile(ctx context.Context, req *profilesv1.DeleteProfileRequest) (*profilesv1.DeleteProfileResponse, error) {
	slog.InfoContext(ctx, "ProfilesServer.DeleteProfile", "profile", req.GetName())
	if err := s.profileRepo.DeleteProfile(ctx, req.GetName()); err != nil {
		return nil, repoErrToStatus(err)
	}
	return &profilesv1.DeleteProfileResponse{}, nil
}
