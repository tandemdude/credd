package opwd

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"

	"github.com/tandemdude/credd/internal/secretstore"
)

// scheme is the reference scheme handled by 1Password ("op://...").
const scheme = "op"

// Resolver resolves a 1Password reference to its secret value. It is
// satisfied by *Client and exists so Store can be tested without the
// 1Password SDK.
type Resolver interface {
	ResolveSecret(ctx context.Context, reference string) (string, error)
}

// ResolverFactory builds a new Resolver; it is called lazily on first use and
// again whenever the desktop app invalidates the current client.
type ResolverFactory = func(ctx context.Context) (Resolver, error)

// Store is the 1Password implementation of secretstore.Store.
type Store struct {
	newResolver ResolverFactory

	mu  sync.Mutex
	r   Resolver
	gen uint64 // bumps each time a new client is built; guards concurrent invalidation
}

var _ secretstore.Store = (*Store)(nil)

// NewStore returns a Store that connects to the 1Password desktop app for
// accountName on first use.
func NewStore(accountName string) *Store {
	return NewStoreWithFactory(func(ctx context.Context) (Resolver, error) {
		return NewClient(ctx, accountName)
	})
}

// NewStoreWithFactory returns a Store that builds its client with factory.
func NewStoreWithFactory(factory ResolverFactory) *Store {
	return &Store{newResolver: factory}
}

func (s *Store) Scheme() string { return scheme }

func (s *Store) Get(ctx context.Context, ref string) (string, error) {
	return s.resolve(ctx, ref)
}

func (s *Store) Exists(ctx context.Context, ref string) (bool, error) {
	_, err := s.resolve(ctx, ref)
	if errors.Is(err, secretstore.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) get(ctx context.Context) (Resolver, uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.r == nil {
		r, err := s.newResolver(ctx)
		if err != nil {
			return nil, 0, err
		}
		s.r = r
		s.gen++
	}
	return s.r, s.gen, nil
}

// invalidate drops the cached client so the next get rebuilds it, but only
// if no one has already replaced the generation the caller observed — otherwise
// a concurrent rebuild would be thrown away.
func (s *Store) invalidate(gen uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if gen == s.gen {
		s.r = nil
	}
}

// isStaleClient reports whether err indicates the 1Password desktop app has
// invalidated our client (e.g. after an app restart, update, or sleep/wake).
// The SDK surfaces this as an untyped error, so a string match is the only
// signal available; see the onepassword-sdk-go errors.go default case.
func isStaleClient(err error) bool {
	return err != nil && strings.Contains(err.Error(), "invalid client id")
}

// resolve resolves a 1Password reference, transparently rebuilding the
// client once if the desktop app has invalidated it.
func (s *Store) resolve(ctx context.Context, ref string) (string, error) {
	r, gen, err := s.get(ctx)
	if err != nil {
		return "", err
	}

	secret, err := r.ResolveSecret(ctx, ref)
	if err == nil || !isStaleClient(err) {
		return secret, err
	}

	slog.WarnContext(ctx, "1Password client invalidated, recreating", "err", err)
	s.invalidate(gen)

	r, _, err = s.get(ctx)
	if err != nil {
		return "", err
	}
	return r.ResolveSecret(ctx, ref)
}
