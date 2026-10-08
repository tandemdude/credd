package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	profilesv1 "github.com/tandemdude/credd/gen/go/profiles/v1"
)

// profileFileVersion is the current version of the profile export format.
const profileFileVersion = 1

// ProfileFile is the shareable JSON representation of a profile, produced by
// ExportProfile and consumed by ImportProfile. It holds secret references, never
// secret values, so it is safe to share with anyone who can resolve them.
type ProfileFile struct {
	Version     int               `json:"version"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Vars        map[string]string `json:"vars"`
}

// ExportProfile returns the shareable representation of a profile.
func (c *Client) ExportProfile(ctx context.Context, name string) (ProfileFile, error) {
	p, err := c.GetProfile(ctx, name)
	if err != nil {
		return ProfileFile{}, err
	}

	vars := make(map[string]string, len(p.GetVars()))
	for _, v := range p.GetVars() {
		vars[v.GetName()] = v.GetValue()
	}
	return ProfileFile{Version: profileFileVersion, Name: p.GetName(), Description: p.GetDescription(), Vars: vars}, nil
}

// ImportProfile atomically creates a profile from f, returning it with the var
// kinds the server detected. If overwrite is true an existing profile with the
// same name is replaced; otherwise the import fails if the name is taken.
func (c *Client) ImportProfile(ctx context.Context, f ProfileFile, overwrite bool) (*profilesv1.Profile, error) {
	vars := make([]*profilesv1.ProfileVar, 0, len(f.Vars))
	for name, value := range f.Vars {
		vars = append(vars, &profilesv1.ProfileVar{Name: name, Value: value})
	}

	resp, err := c.profiles.ImportProfile(ctx, &profilesv1.ImportProfileRequest{
		Profile:   &profilesv1.Profile{Name: f.Name, Description: f.Description, Vars: vars},
		Overwrite: overwrite,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetProfile(), nil
}

// WriteProfileFile writes f as indented JSON. Vars are written sorted by name.
func WriteProfileFile(w io.Writer, f ProfileFile) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(f)
}

// ReadProfileFile parses a profile file, rejecting unknown fields and
// unsupported versions.
func ReadProfileFile(r io.Reader) (ProfileFile, error) {
	var f ProfileFile
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return ProfileFile{}, fmt.Errorf("invalid profile file: %w", err)
	}
	if f.Version != profileFileVersion {
		return ProfileFile{}, fmt.Errorf("unsupported profile file version %d (expected %d)", f.Version, profileFileVersion)
	}
	return f, nil
}
