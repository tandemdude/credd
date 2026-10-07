package opwd

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/tandemdude/credd/internal/secretstore"
)

// fakeResolver implements Resolver for tests.
type fakeResolver struct {
	resolve func(ctx context.Context, reference string) (string, error)
}

func (f *fakeResolver) ResolveSecret(ctx context.Context, reference string) (string, error) {
	return f.resolve(ctx, reference)
}

func TestGetRecreatesClientOnStaleClientError(t *testing.T) {
	var factoryCalls int
	s := NewStoreWithFactory(func(ctx context.Context) (Resolver, error) {
		factoryCalls++
		attempt := factoryCalls
		return &fakeResolver{
			resolve: func(ctx context.Context, reference string) (string, error) {
				if attempt == 1 {
					return "", errors.New("invalid client id")
				}
				return "resolved-secret", nil
			},
		}, nil
	})

	got, err := s.Get(context.Background(), "op://Vault/item/field")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "resolved-secret" {
		t.Fatalf("got %q, want resolved-secret", got)
	}
	if factoryCalls != 2 {
		t.Fatalf("expected client to be recreated (factory called twice), got %d calls", factoryCalls)
	}
}

func TestGetDoesNotRecreateClientOnOtherErrors(t *testing.T) {
	var factoryCalls int
	s := NewStoreWithFactory(func(ctx context.Context) (Resolver, error) {
		factoryCalls++
		return &fakeResolver{
			resolve: func(ctx context.Context, reference string) (string, error) {
				return "", errors.New("something went wrong")
			},
		}, nil
	})

	if _, err := s.Get(context.Background(), "op://Vault/item/field"); err == nil {
		t.Fatal("expected error to propagate")
	}
	if factoryCalls != 1 {
		t.Fatalf("expected no client recreation for non-stale errors, got %d factory calls", factoryCalls)
	}
}

func TestGetReturnsErrorWhenClientStaysStale(t *testing.T) {
	var factoryCalls int
	s := NewStoreWithFactory(func(ctx context.Context) (Resolver, error) {
		factoryCalls++
		return &fakeResolver{
			resolve: func(ctx context.Context, reference string) (string, error) {
				return "", errors.New("invalid client id")
			},
		}, nil
	})

	if _, err := s.Get(context.Background(), "op://Vault/item/field"); err == nil {
		t.Fatal("expected error when client stays stale")
	}
	// One rebuild attempt only: initial client + one recreation.
	if factoryCalls != 2 {
		t.Fatalf("expected exactly one recreation attempt, got %d factory calls", factoryCalls)
	}
}

func TestExists(t *testing.T) {
	otherErr := errors.New("desktop app locked")
	cases := []struct {
		name      string
		resolve   error
		want      bool
		wantErrIs error
	}{
		{"found", nil, true, nil},
		{"not found", fmt.Errorf("%w: item", secretstore.ErrNotFound), false, nil},
		{"missing vault", ErrVaultNotExist, false, nil},
		{"other error", otherErr, false, otherErr},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewStoreWithFactory(func(ctx context.Context) (Resolver, error) {
				return &fakeResolver{
					resolve: func(ctx context.Context, reference string) (string, error) {
						return "value", tc.resolve
					},
				}, nil
			})

			got, err := s.Exists(context.Background(), "op://Vault/item/field")
			if !errors.Is(err, tc.wantErrIs) || (tc.wantErrIs == nil && err != nil) {
				t.Fatalf("err = %v, want %v", err, tc.wantErrIs)
			}
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
