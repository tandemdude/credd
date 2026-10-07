package server

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	secretsv1 "github.com/tandemdude/credd/gen/go/secrets/v1"
	"github.com/tandemdude/credd/internal/secretstore"
)

// fakeStore implements secretstore.Store for tests.
type fakeStore struct {
	secrets map[string]string
	err     error // returned from every call when non-nil
}

func (f *fakeStore) Scheme() string { return "fake" }

func (f *fakeStore) Exists(_ context.Context, ref string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	_, ok := f.secrets[ref]
	return ok, nil
}

func (f *fakeStore) Get(_ context.Context, ref string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	v, ok := f.secrets[ref]
	if !ok {
		return "", fmt.Errorf("%w: %q", secretstore.ErrNotFound, ref)
	}
	return v, nil
}

func newTestServer(store *fakeStore) *SecretsServer {
	return NewSecretsServer(secretstore.NewRegistry(store))
}

func TestGetSecretReturnsValueOnSuccess(t *testing.T) {
	srv := newTestServer(&fakeStore{secrets: map[string]string{"fake://item": "s3cret"}})

	resp, err := srv.GetSecret(context.Background(), &secretsv1.GetSecretRequest{Reference: "fake://item"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetSecret() != "s3cret" {
		t.Fatalf("got %q, want s3cret", resp.GetSecret())
	}
}

func TestGetSecretErrorCodes(t *testing.T) {
	otherErr := errors.New("boom")
	cases := []struct {
		name     string
		store    *fakeStore
		ref      string
		wantCode codes.Code
	}{
		{"missing secret", &fakeStore{}, "fake://missing", codes.NotFound},
		{"bare name", &fakeStore{}, "mykey", codes.InvalidArgument},
		{"unknown scheme", &fakeStore{}, "bw://item", codes.InvalidArgument},
		{"invalid reference", &fakeStore{err: fmt.Errorf("%w: bad", secretstore.ErrInvalidReference)}, "fake://x", codes.InvalidArgument},
		{"other error", &fakeStore{err: otherErr}, "fake://x", codes.Unknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newTestServer(tc.store).GetSecret(context.Background(), &secretsv1.GetSecretRequest{Reference: tc.ref})
			if err == nil {
				t.Fatal("expected error")
			}
			if got := status.Code(err); got != tc.wantCode {
				t.Fatalf("code = %v, want %v (err: %v)", got, tc.wantCode, err)
			}
		})
	}
}

func TestSecretExists(t *testing.T) {
	srv := newTestServer(&fakeStore{secrets: map[string]string{"fake://item": "s3cret"}})

	for ref, want := range map[string]bool{"fake://item": true, "fake://missing": false} {
		resp, err := srv.SecretExists(context.Background(), &secretsv1.SecretExistsRequest{Reference: ref})
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", ref, err)
		}
		if resp.GetExists() != want {
			t.Fatalf("%s: exists = %v, want %v", ref, resp.GetExists(), want)
		}
	}
}

func TestSecretExistsRejectsUnsupportedReference(t *testing.T) {
	_, err := newTestServer(&fakeStore{}).SecretExists(context.Background(), &secretsv1.SecretExistsRequest{Reference: "mykey"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
	}
}
