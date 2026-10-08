package client

import (
	"context"

	profilesv1 "github.com/tandemdude/credd/gen/go/profiles/v1"
	"github.com/tandemdude/credd/internal/envtemplate"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CreateProfile creates a new, empty profile.
func (c *Client) CreateProfile(ctx context.Context, name, description string) error {
	_, err := c.profiles.CreateProfile(ctx, &profilesv1.CreateProfileRequest{Name: name, Description: description})
	return err
}

// ListProfiles returns the name and description of every profile, sorted by name.
func (c *Client) ListProfiles(ctx context.Context) ([]*profilesv1.ProfileSummary, error) {
	resp, err := c.profiles.ListProfiles(ctx, &profilesv1.ListProfilesRequest{})
	if err != nil {
		return nil, err
	}
	return resp.GetProfiles(), nil
}

// SetProfileDescription replaces a profile's description; an empty
// description clears it.
func (c *Client) SetProfileDescription(ctx context.Context, name, description string) error {
	_, err := c.profiles.SetProfileDescription(ctx, &profilesv1.SetProfileDescriptionRequest{Name: name, Description: description})
	return err
}

// GetProfile returns a profile and its vars, sorted by name.
func (c *Client) GetProfile(ctx context.Context, name string) (*profilesv1.Profile, error) {
	resp, err := c.profiles.GetProfile(ctx, &profilesv1.GetProfileRequest{Name: name})
	if err != nil {
		return nil, err
	}
	return resp.GetProfile(), nil
}

// SetProfileVar creates or replaces a var in a profile, returning it with the
// kind (plain or secret) the server detected for its value.
func (c *Client) SetProfileVar(ctx context.Context, profile, name, value string) (*profilesv1.ProfileVar, error) {
	resp, err := c.profiles.SetProfileVar(ctx, &profilesv1.SetProfileVarRequest{Profile: profile, Name: name, Value: value})
	if err != nil {
		return nil, err
	}
	return resp.GetVar(), nil
}

// UnsetProfileVar removes a var from a profile.
func (c *Client) UnsetProfileVar(ctx context.Context, profile, name string) error {
	_, err := c.profiles.UnsetProfileVar(ctx, &profilesv1.UnsetProfileVarRequest{Profile: profile, Name: name})
	return err
}

// DeleteProfile deletes a profile and all of its vars.
func (c *Client) DeleteProfile(ctx context.Context, name string) error {
	_, err := c.profiles.DeleteProfile(ctx, &profilesv1.DeleteProfileRequest{Name: name})
	return err
}

// Problem describes a secret reference in a profile that cannot be resolved.
type Problem struct {
	Var    string
	Ref    string
	Reason string
}

// ValidateProfile checks that every secret reference in a profile exists,
// without fetching any secret values. Unresolvable references are returned
// as problems; an error is returned only if validation itself could not run
// (e.g. the server or secret manager is unreachable).
func (c *Client) ValidateProfile(ctx context.Context, name string) ([]Problem, error) {
	p, err := c.GetProfile(ctx, name)
	if err != nil {
		return nil, err
	}

	var problems []Problem
	for _, v := range p.GetVars() {
		if v.GetKind() != profilesv1.VarKind_VAR_KIND_SECRET {
			continue
		}

		parts, err := envtemplate.Parse(v.GetValue())
		if err != nil {
			problems = append(problems, Problem{Var: v.GetName(), Reason: err.Error()})
			continue
		}

		for _, ref := range envtemplate.Refs(parts) {
			exists, err := c.Exists(ctx, ref)
			switch {
			case err == nil && exists:
			case err == nil:
				problems = append(problems, Problem{Var: v.GetName(), Ref: ref, Reason: "secret does not exist"})
			case status.Code(err) == codes.InvalidArgument:
				problems = append(problems, Problem{Var: v.GetName(), Ref: ref, Reason: status.Convert(err).Message()})
			default:
				return nil, err
			}
		}
	}
	return problems, nil
}
