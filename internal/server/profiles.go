package server

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

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

// maxDescriptionLen is the maximum length, in characters, of a profile description.
const maxDescriptionLen = 200

// validateDescription requires descriptions to be short and fit on one line,
// so they display cleanly in `credd profile list`.
func validateDescription(description string) error {
	if utf8.RuneCountInString(description) > maxDescriptionLen {
		return status.Errorf(codes.InvalidArgument, "invalid description: must be at most %d characters", maxDescriptionLen)
	}
	if strings.ContainsFunc(description, unicode.IsControl) {
		return status.Error(codes.InvalidArgument, "invalid description: must be a single line without control characters")
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

func toProtoProfile(p models.Profile) *profilesv1.Profile {
	vars := make([]*profilesv1.ProfileVar, len(p.Vars))
	for i, v := range p.Vars {
		vars[i] = toProtoVar(v)
	}
	return &profilesv1.Profile{Name: p.Name, Description: p.Description, Vars: vars}
}

func (s *ProfilesServer) CreateProfile(ctx context.Context, req *profilesv1.CreateProfileRequest) (*profilesv1.CreateProfileResponse, error) {
	slog.InfoContext(ctx, "ProfilesServer.CreateProfile", "profile", req.GetName())
	if err := validateProfileName(req.GetName()); err != nil {
		return nil, err
	}
	if err := validateDescription(req.GetDescription()); err != nil {
		return nil, err
	}
	if err := s.profileRepo.CreateProfile(ctx, req.GetName(), req.GetDescription()); err != nil {
		return nil, repoErrToStatus(err)
	}
	return &profilesv1.CreateProfileResponse{}, nil
}

func (s *ProfilesServer) ListProfiles(ctx context.Context, _ *profilesv1.ListProfilesRequest) (*profilesv1.ListProfilesResponse, error) {
	slog.InfoContext(ctx, "ProfilesServer.ListProfiles")
	profiles, err := s.profileRepo.ListProfiles(ctx)
	if err != nil {
		return nil, err
	}

	summaries := make([]*profilesv1.ProfileSummary, len(profiles))
	for i, p := range profiles {
		summaries[i] = &profilesv1.ProfileSummary{Name: p.Name, Description: p.Description}
	}
	return &profilesv1.ListProfilesResponse{Profiles: summaries}, nil
}

func (s *ProfilesServer) GetProfile(ctx context.Context, req *profilesv1.GetProfileRequest) (*profilesv1.GetProfileResponse, error) {
	slog.InfoContext(ctx, "ProfilesServer.GetProfile", "profile", req.GetName())
	p, err := s.profileRepo.GetProfile(ctx, req.GetName())
	if err != nil {
		return nil, repoErrToStatus(err)
	}

	return &profilesv1.GetProfileResponse{Profile: toProtoProfile(p)}, nil
}

func (s *ProfilesServer) SetProfileDescription(ctx context.Context, req *profilesv1.SetProfileDescriptionRequest) (*profilesv1.SetProfileDescriptionResponse, error) {
	slog.InfoContext(ctx, "ProfilesServer.SetProfileDescription", "profile", req.GetName())
	if err := validateDescription(req.GetDescription()); err != nil {
		return nil, err
	}
	if err := s.profileRepo.SetProfileDescription(ctx, req.GetName(), req.GetDescription()); err != nil {
		return nil, repoErrToStatus(err)
	}
	return &profilesv1.SetProfileDescriptionResponse{}, nil
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

func (s *ProfilesServer) ImportProfile(ctx context.Context, req *profilesv1.ImportProfileRequest) (*profilesv1.ImportProfileResponse, error) {
	in := req.GetProfile()
	slog.InfoContext(ctx, "ProfilesServer.ImportProfile", "profile", in.GetName(), "vars", len(in.GetVars()), "overwrite", req.GetOverwrite())
	if err := validateProfileName(in.GetName()); err != nil {
		return nil, err
	}
	if err := validateDescription(in.GetDescription()); err != nil {
		return nil, err
	}

	p := models.Profile{Name: in.GetName(), Description: in.GetDescription(), Vars: make([]models.ProfileVar, len(in.GetVars()))}
	seen := make(map[string]bool, len(in.GetVars()))
	for i, v := range in.GetVars() {
		if err := validateVarName(v.GetName()); err != nil {
			return nil, err
		}
		if seen[v.GetName()] {
			return nil, status.Errorf(codes.InvalidArgument, "duplicate env var %q", v.GetName())
		}
		seen[v.GetName()] = true
		// Kinds are never trusted from the request: an imported file may come
		// from someone else, so classify values exactly as SetProfileVar does.
		p.Vars[i] = models.ProfileVar{Name: v.GetName(), Value: v.GetValue(), Kind: detectKind(v.GetValue(), s.isSecretRef)}
	}
	slices.SortFunc(p.Vars, func(a, b models.ProfileVar) int { return strings.Compare(a.Name, b.Name) })

	if err := s.profileRepo.ImportProfile(ctx, p, req.GetOverwrite()); err != nil {
		return nil, repoErrToStatus(err)
	}
	return &profilesv1.ImportProfileResponse{Profile: toProtoProfile(p)}, nil
}
