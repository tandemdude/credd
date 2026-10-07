package secretstore

import (
	"context"
	"errors"
	"testing"
)

type fakeStore struct {
	scheme  string
	secrets map[string]string
}

func (f *fakeStore) Scheme() string { return f.scheme }

func (f *fakeStore) Exists(_ context.Context, ref string) (bool, error) {
	_, ok := f.secrets[ref]
	return ok, nil
}

func (f *fakeStore) Get(_ context.Context, ref string) (string, error) {
	v, ok := f.secrets[ref]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func TestScheme(t *testing.T) {
	cases := []struct {
		ref    string
		want   string
		wantOk bool
	}{
		{"op://Vault/item/field", "op", true},
		{"bw://item", "bw", true},
		{"plainname", "", false},
		{"://nothing", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		got, ok := Scheme(tc.ref)
		if got != tc.want || ok != tc.wantOk {
			t.Errorf("Scheme(%q) = (%q, %v), want (%q, %v)", tc.ref, got, ok, tc.want, tc.wantOk)
		}
	}
}

func TestRegistryDispatchesByScheme(t *testing.T) {
	r := NewRegistry(
		&fakeStore{scheme: "op", secrets: map[string]string{"op://V/i/f": "from-op"}},
		&fakeStore{scheme: "bw", secrets: map[string]string{"bw://i": "from-bw"}},
	)
	ctx := context.Background()

	if v, err := r.Get(ctx, "op://V/i/f"); err != nil || v != "from-op" {
		t.Fatalf("Get op = (%q, %v)", v, err)
	}
	if v, err := r.Get(ctx, "bw://i"); err != nil || v != "from-bw" {
		t.Fatalf("Get bw = (%q, %v)", v, err)
	}
	if ok, err := r.Exists(ctx, "op://V/missing/f"); err != nil || ok {
		t.Fatalf("Exists missing = (%v, %v), want (false, nil)", ok, err)
	}
	if _, err := r.Get(ctx, "op://V/missing/f"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get missing err = %v, want ErrNotFound", err)
	}
}

func TestRegistryRejectsUnsupportedReferences(t *testing.T) {
	r := NewRegistry(&fakeStore{scheme: "op"})
	ctx := context.Background()

	for _, ref := range []string{"plainname", "lp://item"} {
		if _, err := r.Get(ctx, ref); !errors.Is(err, ErrUnsupportedReference) {
			t.Errorf("Get(%q) err = %v, want ErrUnsupportedReference", ref, err)
		}
		if _, err := r.Exists(ctx, ref); !errors.Is(err, ErrUnsupportedReference) {
			t.Errorf("Exists(%q) err = %v, want ErrUnsupportedReference", ref, err)
		}
	}
}

func TestNewRegistryPanicsOnDuplicateScheme(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for duplicate scheme")
		}
	}()
	NewRegistry(&fakeStore{scheme: "op"}, &fakeStore{scheme: "op"})
}
