// Package secretstore defines the abstraction over external secret managers
// (1Password today; potentially Bitwarden, LastPass, ... later). credd never
// stores secret values itself: every secret is addressed by a reference of the
// form "<scheme>://<path>", and the scheme selects which Store resolves it.
package secretstore

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrNotFound is returned (possibly wrapped) when a well-formed reference
	// does not point at an existing secret.
	ErrNotFound = errors.New("secret not found")
	// ErrInvalidReference is returned (possibly wrapped) when a reference is
	// malformed for the store that owns its scheme.
	ErrInvalidReference = errors.New("invalid secret reference")
	// ErrUnsupportedReference is returned when no registered store handles a
	// reference's scheme.
	ErrUnsupportedReference = errors.New("unsupported secret reference")
)

// Store is a read-only view onto an external secret manager.
type Store interface {
	// Scheme is the reference scheme this store handles, without the "://"
	// suffix, e.g. "op" for "op://vault/item/field".
	Scheme() string
	// Exists reports whether ref points at an existing secret. A missing
	// secret is (false, nil), not an error.
	Exists(ctx context.Context, ref string) (bool, error)
	// Get returns the value of the secret ref points at, or an error wrapping
	// ErrNotFound if there is no such secret.
	Get(ctx context.Context, ref string) (string, error)
}

// Scheme returns the scheme of ref ("op" for "op://..."), or false if ref has
// no "<scheme>://" prefix.
func Scheme(ref string) (string, bool) {
	scheme, _, found := strings.Cut(ref, "://")
	if !found || scheme == "" {
		return "", false
	}
	return scheme, true
}

// Registry dispatches references to the Store registered for their scheme.
type Registry struct {
	stores map[string]Store
}

// NewRegistry builds a registry from stores. It panics if two stores claim
// the same scheme, since that is a wiring bug.
func NewRegistry(stores ...Store) *Registry {
	r := &Registry{stores: make(map[string]Store, len(stores))}
	for _, s := range stores {
		if _, dup := r.stores[s.Scheme()]; dup {
			panic(fmt.Sprintf("secretstore: duplicate store for scheme %q", s.Scheme()))
		}
		r.stores[s.Scheme()] = s
	}
	return r
}

// Supports reports whether ref has the scheme of a registered store.
func (r *Registry) Supports(ref string) bool {
	_, err := r.storeFor(ref)
	return err == nil
}

func (r *Registry) storeFor(ref string) (Store, error) {
	scheme, ok := Scheme(ref)
	if !ok {
		return nil, fmt.Errorf("%w %q: expected <scheme>://...", ErrUnsupportedReference, ref)
	}
	s, ok := r.stores[scheme]
	if !ok {
		return nil, fmt.Errorf("%w %q: no secret store configured for %s://", ErrUnsupportedReference, ref, scheme)
	}
	return s, nil
}

// Exists reports whether ref points at an existing secret in its store.
func (r *Registry) Exists(ctx context.Context, ref string) (bool, error) {
	s, err := r.storeFor(ref)
	if err != nil {
		return false, err
	}
	return s.Exists(ctx, ref)
}

// Get returns the value of the secret ref points at.
func (r *Registry) Get(ctx context.Context, ref string) (string, error) {
	s, err := r.storeFor(ref)
	if err != nil {
		return "", err
	}
	return s.Get(ctx, ref)
}
