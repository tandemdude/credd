package server

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	profilesv1 "github.com/tandemdude/credd/gen/go/profiles/v1"
	"github.com/tandemdude/credd/internal/domain/models"
	"github.com/tandemdude/credd/internal/domain/repository"
)

func isOpRef(ref string) bool { return strings.HasPrefix(ref, "op://") }

func TestDetectKind(t *testing.T) {
	cases := []struct {
		value string
		want  models.VarKind
	}{
		{"dev", models.VarKindPlain},
		{"", models.VarKindPlain},
		{"op://Vault/item/field", models.VarKindSecret},
		{"https://example.com", models.VarKindPlain}, // a scheme, but no store for it
		{"bw://item", models.VarKindPlain},
		{"postgres://u:{op://V/DB/pw}@h/db", models.VarKindSecret},
		{"{bw://item}", models.VarKindPlain},
		{`{"json": true}`, models.VarKindPlain}, // braces, but no supported refs
		{"a{b", models.VarKindPlain},            // unparseable template
		{"{{literal}}", models.VarKindPlain},
	}
	for _, tc := range cases {
		if got := detectKind(tc.value, isOpRef); got != tc.want {
			t.Errorf("detectKind(%q) = %q, want %q", tc.value, got, tc.want)
		}
	}
}

// fakeProfileRepo implements repository.ProfileRepository for tests.
// Unimplemented methods are provided by the embedded interface (they panic if called).
type fakeProfileRepo struct {
	repository.ProfileRepository
	setVar        func(ctx context.Context, profile string, v models.ProfileVar) error
	importProfile func(ctx context.Context, p models.Profile, overwrite bool) error
}

func (f *fakeProfileRepo) ImportProfile(ctx context.Context, p models.Profile, overwrite bool) error {
	return f.importProfile(ctx, p, overwrite)
}

func (f *fakeProfileRepo) SetProfileVar(ctx context.Context, profile string, v models.ProfileVar) error {
	return f.setVar(ctx, profile, v)
}

func TestSetProfileVarStoresDetectedKind(t *testing.T) {
	var stored models.ProfileVar
	srv := NewProfilesServer(&fakeProfileRepo{
		setVar: func(_ context.Context, _ string, v models.ProfileVar) error {
			stored = v
			return nil
		},
	}, isOpRef)

	resp, err := srv.SetProfileVar(context.Background(), &profilesv1.SetProfileVarRequest{
		Profile: "dev", Name: "TOKEN", Value: "op://V/i/f",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stored.Kind != models.VarKindSecret {
		t.Fatalf("stored kind = %q, want secret", stored.Kind)
	}
	if resp.GetVar().GetKind() != profilesv1.VarKind_VAR_KIND_SECRET {
		t.Fatalf("response kind = %v, want secret", resp.GetVar().GetKind())
	}
}

func TestSetProfileVarRejectsInvalidName(t *testing.T) {
	srv := NewProfilesServer(&fakeProfileRepo{}, isOpRef)

	for _, name := range []string{"", "1FOO", "FOO-BAR", "FOO=BAR", "FOO BAR"} {
		_, err := srv.SetProfileVar(context.Background(), &profilesv1.SetProfileVarRequest{
			Profile: "dev", Name: name, Value: "x",
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("name %q: code = %v, want InvalidArgument", name, status.Code(err))
		}
	}
}

func TestSetProfileVarMissingProfileIsNotFound(t *testing.T) {
	srv := NewProfilesServer(&fakeProfileRepo{
		setVar: func(context.Context, string, models.ProfileVar) error {
			return fmt.Errorf("profile %q: %w", "nope", repository.ErrNotFound)
		},
	}, isOpRef)

	_, err := srv.SetProfileVar(context.Background(), &profilesv1.SetProfileVarRequest{
		Profile: "nope", Name: "FOO", Value: "x",
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("code = %v, want NotFound", status.Code(err))
	}
}

func TestCreateProfileRejectsInvalidName(t *testing.T) {
	srv := NewProfilesServer(&fakeProfileRepo{}, isOpRef)

	for _, name := range []string{"", "a b", "a/b", "a.b"} {
		_, err := srv.CreateProfile(context.Background(), &profilesv1.CreateProfileRequest{Name: name})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("name %q: code = %v, want InvalidArgument", name, status.Code(err))
		}
	}
}

func TestCreateProfileRejectsInvalidDescription(t *testing.T) {
	srv := NewProfilesServer(&fakeProfileRepo{}, isOpRef)

	for _, d := range []string{"two\nlines", "tab\there", strings.Repeat("x", maxDescriptionLen+1)} {
		_, err := srv.CreateProfile(context.Background(), &profilesv1.CreateProfileRequest{Name: "dev", Description: d})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("description %q: code = %v, want InvalidArgument", d, status.Code(err))
		}
	}
}

func TestImportProfileDetectsKindsAndSorts(t *testing.T) {
	var stored models.Profile
	var storedOverwrite bool
	srv := NewProfilesServer(&fakeProfileRepo{
		importProfile: func(_ context.Context, p models.Profile, overwrite bool) error {
			stored, storedOverwrite = p, overwrite
			return nil
		},
	}, isOpRef)

	resp, err := srv.ImportProfile(context.Background(), &profilesv1.ImportProfileRequest{
		Profile: &profilesv1.Profile{Name: "dev", Description: "d", Vars: []*profilesv1.ProfileVar{
			// The client-supplied kind is wrong on purpose; it must be ignored.
			{Name: "TOKEN", Value: "op://V/i/f", Kind: profilesv1.VarKind_VAR_KIND_PLAIN},
			{Name: "ENV", Value: "dev", Kind: profilesv1.VarKind_VAR_KIND_SECRET},
		}},
		Overwrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := models.Profile{Name: "dev", Description: "d", Vars: []models.ProfileVar{
		{Name: "ENV", Value: "dev", Kind: models.VarKindPlain},
		{Name: "TOKEN", Value: "op://V/i/f", Kind: models.VarKindSecret},
	}}
	if !reflect.DeepEqual(stored, want) {
		t.Fatalf("stored = %+v, want %+v", stored, want)
	}
	if !storedOverwrite {
		t.Fatal("overwrite was not passed through")
	}
	if got := resp.GetProfile().GetVars()[1].GetKind(); got != profilesv1.VarKind_VAR_KIND_SECRET {
		t.Fatalf("response TOKEN kind = %v, want secret", got)
	}
}

func TestImportProfileRejectsInvalidInput(t *testing.T) {
	srv := NewProfilesServer(&fakeProfileRepo{}, isOpRef)

	for _, p := range []*profilesv1.Profile{
		{Name: "bad name"},
		{Name: "dev", Description: "a\nb"},
		{Name: "dev", Vars: []*profilesv1.ProfileVar{{Name: "1BAD"}}},
		{Name: "dev", Vars: []*profilesv1.ProfileVar{{Name: "A"}, {Name: "A"}}},
	} {
		_, err := srv.ImportProfile(context.Background(), &profilesv1.ImportProfileRequest{Profile: p})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("profile %v: code = %v, want InvalidArgument", p, status.Code(err))
		}
	}
}
