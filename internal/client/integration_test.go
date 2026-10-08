package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/tandemdude/credd/db"
	profilesv1 "github.com/tandemdude/credd/gen/go/profiles/v1"
	secretsv1 "github.com/tandemdude/credd/gen/go/secrets/v1"
	"github.com/tandemdude/credd/internal/secretstore"
	"github.com/tandemdude/credd/internal/server"
	"github.com/tandemdude/credd/internal/sqlite"

	_ "modernc.org/sqlite"
)

// fakeStore is an in-memory secretstore.Store handling the "op" scheme, and
// records every reference it is asked to resolve.
type fakeStore struct {
	secrets  map[string]string
	resolved []string
}

func (f *fakeStore) Scheme() string { return "op" }

func (f *fakeStore) Exists(_ context.Context, ref string) (bool, error) {
	_, ok := f.secrets[ref]
	return ok, nil
}

func (f *fakeStore) Get(_ context.Context, ref string) (string, error) {
	f.resolved = append(f.resolved, ref)
	v, ok := f.secrets[ref]
	if !ok {
		return "", fmt.Errorf("%w: %q", secretstore.ErrNotFound, ref)
	}
	return v, nil
}

// newIntegrationClient wires a real gRPC server (real sqlite profile storage,
// fake secret store) to a Client over an in-memory connection.
func newIntegrationClient(t *testing.T, store *fakeStore) *Client {
	t.Helper()

	conn, err := db.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	registry := secretstore.NewRegistry(store)
	srv := grpc.NewServer()
	secretsv1.RegisterSecretsServer(srv, server.NewSecretsServer(registry))
	profilesv1.RegisterProfilesServer(srv, server.NewProfilesServer(sqlite.NewStore(conn).Profiles, registry.Supports))

	lis := bufconn.Listen(1 << 20)
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)

	cc, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	c := newFromConn(cc)
	t.Cleanup(func() { c.Close() })
	return c
}

func mustSet(t *testing.T, c *Client, profile, name, value string) {
	t.Helper()
	if _, err := c.SetProfileVar(context.Background(), profile, name, value); err != nil {
		t.Fatal(err)
	}
}

// runAndCaptureEnv runs `credd run` with a shell command that writes the
// named vars to a file, and returns their values.
func runAndCaptureEnv(t *testing.T, c *Client, profiles, envSpecs []string, names ...string) map[string]string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "env")
	var script strings.Builder
	for _, n := range names {
		fmt.Fprintf(&script, `printf '%%s=%%s\n' %s "$%s" >> %s;`, n, n, out)
	}

	code, err := c.Run(context.Background(), profiles, envSpecs, []string{"sh", "-c", script.String()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if code != 0 {
		t.Fatalf("Run exit code %d", code)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		k, v, _ := strings.Cut(line, "=")
		got[k] = v
	}
	return got
}

func TestRunWithProfiles(t *testing.T) {
	ctx := context.Background()
	store := &fakeStore{secrets: map[string]string{
		"op://V/base/token": "base-token",
		"op://V/dev/token":  "dev-token",
		"op://V/db/pw":      "hunter2",
		"op://V/extra":      "extra-secret",
	}}
	c := newIntegrationClient(t, store)

	for _, p := range []string{"base", "dev"} {
		if err := c.CreateProfile(ctx, p, ""); err != nil {
			t.Fatal(err)
		}
	}
	mustSet(t, c, "base", "ENV", "base")
	mustSet(t, c, "base", "REGION", "eu")
	mustSet(t, c, "base", "JSON", `{"a":1}`)
	mustSet(t, c, "base", "TOKEN", "op://V/base/token")
	mustSet(t, c, "dev", "ENV", "dev")
	mustSet(t, c, "dev", "TOKEN", "op://V/dev/token")
	mustSet(t, c, "dev", "DB", "postgres://u:{op://V/db/pw}@h/db")

	got := runAndCaptureEnv(
		t, c,
		[]string{"base", "dev"},
		[]string{"REGION=op://V/extra"},
		"ENV", "REGION", "JSON", "TOKEN", "DB",
	)
	want := map[string]string{
		"ENV":    "dev",          // later profile overrides
		"REGION": "extra-secret", // --env overrides all profiles
		"JSON":   `{"a":1}`,      // plain value with braces is verbatim
		"TOKEN":  "dev-token",
		"DB":     "postgres://u:hunter2@h/db",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}

	// The overridden base TOKEN must never have been fetched.
	for _, ref := range store.resolved {
		if ref == "op://V/base/token" {
			t.Fatalf("overridden secret was resolved; resolved refs: %v", store.resolved)
		}
	}
}

func TestRunFailsFastOnMissingProfile(t *testing.T) {
	c := newIntegrationClient(t, &fakeStore{})

	marker := filepath.Join(t.TempDir(), "ran")
	_, err := c.Run(context.Background(), []string{"nope"}, nil, []string{"touch", marker})
	if err == nil {
		t.Fatal("expected error for missing profile")
	}
	if _, statErr := os.Stat(marker); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("target must not run when a profile is missing")
	}
}

func TestValidateProfile(t *testing.T) {
	ctx := context.Background()
	c := newIntegrationClient(t, &fakeStore{secrets: map[string]string{"op://V/ok": "x"}})

	if err := c.CreateProfile(ctx, "dev", ""); err != nil {
		t.Fatal(err)
	}
	mustSet(t, c, "dev", "OK", "op://V/ok")
	mustSet(t, c, "dev", "PLAIN", "dev")
	mustSet(t, c, "dev", "MISSING", "op://V/missing")
	mustSet(t, c, "dev", "MIXED", "{op://V/ok}:{bw://item}")

	problems, err := c.ValidateProfile(ctx, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 2 {
		t.Fatalf("expected 2 problems, got %+v", problems)
	}
	// Vars come back sorted by name: MISSING, then MIXED.
	if problems[0].Var != "MISSING" || problems[0].Ref != "op://V/missing" {
		t.Errorf("unexpected first problem: %+v", problems[0])
	}
	if problems[1].Var != "MIXED" || problems[1].Ref != "bw://item" {
		t.Errorf("unexpected second problem: %+v", problems[1])
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	ctx := context.Background()
	c := newIntegrationClient(t, &fakeStore{})

	if err := c.CreateProfile(ctx, "dev", "local development"); err != nil {
		t.Fatal(err)
	}
	mustSet(t, c, "dev", "ENV", "dev")
	mustSet(t, c, "dev", "TOKEN", "op://V/dev/token")

	exported, err := c.ExportProfile(ctx, "dev")
	if err != nil {
		t.Fatal(err)
	}
	var buf strings.Builder
	if err := WriteProfileFile(&buf, exported); err != nil {
		t.Fatal(err)
	}
	f, err := ReadProfileFile(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatal(err)
	}

	// The original name is taken, so importing without a rename or overwrite fails.
	if _, err := c.ImportProfile(ctx, f, false); err == nil {
		t.Fatal("expected import over an existing profile to fail")
	}

	f.Name = "dev-copy"
	if _, err := c.ImportProfile(ctx, f, false); err != nil {
		t.Fatal(err)
	}
	got, err := c.GetProfile(ctx, "dev-copy")
	if err != nil {
		t.Fatal(err)
	}
	want, err := c.GetProfile(ctx, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if got.GetDescription() != "local development" {
		t.Errorf("description = %q, want %q", got.GetDescription(), "local development")
	}
	// Kinds are re-detected by the server on import, so they must match the original.
	if len(got.GetVars()) != len(want.GetVars()) {
		t.Fatalf("vars = %v, want %v", got.GetVars(), want.GetVars())
	}
	for i := range want.GetVars() {
		g, w := got.GetVars()[i], want.GetVars()[i]
		if g.GetName() != w.GetName() || g.GetValue() != w.GetValue() || g.GetKind() != w.GetKind() {
			t.Errorf("var %d = %v, want %v", i, g, w)
		}
	}
}

func TestReadProfileFileRejectsBadInput(t *testing.T) {
	for _, in := range []string{
		`{"version": 2, "name": "dev", "vars": {}}`,
		`{"version": 1, "name": "dev", "vars": {}, "extra": true}`,
		`not json`,
	} {
		if _, err := ReadProfileFile(strings.NewReader(in)); err == nil {
			t.Errorf("ReadProfileFile(%q): expected error", in)
		}
	}
}
