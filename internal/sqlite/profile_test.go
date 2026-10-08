package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tandemdude/credd/db"
	"github.com/tandemdude/credd/internal/domain/models"
	"github.com/tandemdude/credd/internal/domain/repository"

	_ "modernc.org/sqlite"
)

func newTestRepo(t *testing.T) repository.ProfileRepository {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return NewProfileRepository(db.New(conn))
}

func TestProfileLifecycle(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)

	// "dev" is created last so that, once deleted, recreating it reuses the
	// same rowid; that makes the cascade check below meaningful.
	if err := r.CreateProfile(ctx, "base"); err != nil {
		t.Fatal(err)
	}
	if err := r.CreateProfile(ctx, "dev"); err != nil {
		t.Fatal(err)
	}
	if err := r.CreateProfile(ctx, "dev"); !errors.Is(err, repository.ErrAlreadyExists) {
		t.Fatalf("duplicate create err = %v, want ErrAlreadyExists", err)
	}

	names, err := r.ListProfiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(names, []string{"base", "dev"}) {
		t.Fatalf("ListProfiles = %v", names)
	}

	set := func(v models.ProfileVar) {
		t.Helper()
		if err := r.SetProfileVar(ctx, "dev", v); err != nil {
			t.Fatal(err)
		}
	}
	set(models.ProfileVar{Name: "TOKEN", Value: "op://V/i/f", Kind: models.VarKindSecret})
	set(models.ProfileVar{Name: "ENV", Value: "dev", Kind: models.VarKindPlain})
	set(models.ProfileVar{Name: "ENV", Value: "staging", Kind: models.VarKindPlain}) // overwrite

	p, err := r.GetProfile(ctx, "dev")
	if err != nil {
		t.Fatal(err)
	}
	want := models.Profile{Name: "dev", Vars: []models.ProfileVar{
		{Name: "ENV", Value: "staging", Kind: models.VarKindPlain},
		{Name: "TOKEN", Value: "op://V/i/f", Kind: models.VarKindSecret},
	}}
	if !reflect.DeepEqual(p, want) {
		t.Fatalf("GetProfile = %+v, want %+v", p, want)
	}

	if err := r.UnsetProfileVar(ctx, "dev", "ENV"); err != nil {
		t.Fatal(err)
	}
	if err := r.UnsetProfileVar(ctx, "dev", "ENV"); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("second unset err = %v, want ErrNotFound", err)
	}

	if err := r.DeleteProfile(ctx, "dev"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetProfile(ctx, "dev"); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("GetProfile after delete err = %v, want ErrNotFound", err)
	}

	// Recreating the profile must not resurrect its old vars (ON DELETE CASCADE).
	if err := r.CreateProfile(ctx, "dev"); err != nil {
		t.Fatal(err)
	}
	p, err = r.GetProfile(ctx, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Vars) != 0 {
		t.Fatalf("expected no vars on recreated profile, got %+v", p.Vars)
	}
}

func TestMissingProfileIsNotFound(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)

	if err := r.SetProfileVar(ctx, "nope", models.ProfileVar{Name: "A", Value: "b", Kind: models.VarKindPlain}); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("SetProfileVar err = %v, want ErrNotFound", err)
	}
	if err := r.UnsetProfileVar(ctx, "nope", "A"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("UnsetProfileVar err = %v, want ErrNotFound", err)
	}
	if err := r.DeleteProfile(ctx, "nope"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("DeleteProfile err = %v, want ErrNotFound", err)
	}
	if _, err := r.GetProfile(ctx, "nope"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("GetProfile err = %v, want ErrNotFound", err)
	}
}
