package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	profilesv1 "github.com/tandemdude/credd/gen/go/profiles/v1"
	"github.com/tandemdude/credd/internal/envtemplate"
)

// EnvVar is a single --env NAME=ref pairing for `credd run`.
type EnvVar struct {
	Name string
	Ref  string
}

// runProcess executes target (target[0] is the command, the rest are args)
// with the given environment, inheriting the parent's stdio. It returns the
// child's exit code. A non-zero child exit is NOT an error; only a failure to
// start/await the process is.
func runProcess(ctx context.Context, env, target []string) (int, error) {
	if len(target) == 0 {
		return 1, errors.New("runProcess: target must not be empty")
	}
	cmd := exec.CommandContext(ctx, target[0], target[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			// Non-zero child exit (incl. -1 when killed by signal/context
			// cancellation) is reported as the exit code, not an error.
			return exitErr.ExitCode(), nil
		}
		return 1, fmt.Errorf("failed to run %q: %w", target[0], err)
	}
	return 0, nil
}

// Run resolves the given profiles (in order) and env specs via the server, then
// executes target with the resolved variables appended to the inherited
// environment. Later profiles override earlier ones, and --env specs override
// all profiles; resolved values override inherited ones on conflict. This
// relies on POSIX last-definition-wins semantics for duplicate env keys. It
// returns the child's exit code. Resolution failures are fail-fast: the target
// is never executed.
func (c *Client) Run(ctx context.Context, profileNames, envSpecs, target []string) (int, error) {
	if len(target) == 0 {
		return 1, errors.New("no command provided after '--'")
	}

	envVars, err := parseEnvSpec(envSpecs)
	if err != nil {
		return 1, err
	}

	profiles := make([]*profilesv1.Profile, len(profileNames))
	for i, name := range profileNames {
		profiles[i], err = c.GetProfile(ctx, name)
		if err != nil {
			return 1, fmt.Errorf("failed to load profile %q: %w", name, err)
		}
	}

	env := os.Environ()
	for _, v := range mergeVars(profiles, envVars) {
		value, err := c.resolveValue(ctx, v)
		if err != nil {
			return 1, err
		}
		env = append(env, v.name+"="+value)
	}

	return runProcess(ctx, env, target)
}

// pendingVar is an env var whose final value has been chosen but not yet
// resolved.
type pendingVar struct {
	name   string
	value  string
	secret bool   // resolve value as a template of secret references
	source string // where the value came from, for error messages
}

// mergeVars layers the vars of each profile in order, then the --env vars, so
// that later definitions replace earlier ones. Merging happens before any
// resolution so overridden secrets are never fetched. Vars keep the position
// of their first definition, which keeps the result deterministic.
func mergeVars(profiles []*profilesv1.Profile, envVars []EnvVar) []pendingVar {
	var merged []pendingVar
	index := make(map[string]int)
	set := func(v pendingVar) {
		if i, ok := index[v.name]; ok {
			merged[i] = v
			return
		}
		index[v.name] = len(merged)
		merged = append(merged, v)
	}

	for _, p := range profiles {
		for _, v := range p.GetVars() {
			set(pendingVar{
				name:   v.GetName(),
				value:  v.GetValue(),
				secret: v.GetKind() == profilesv1.VarKind_VAR_KIND_SECRET,
				source: fmt.Sprintf("profile %q", p.GetName()),
			})
		}
	}
	// --env values are always secret references or templates of them.
	for _, v := range envVars {
		set(pendingVar{name: v.Name, value: v.Ref, secret: true, source: "--env"})
	}
	return merged
}

// resolveValue returns a var's final value: plain values are used verbatim,
// secret values are parsed as a template and every reference within is
// resolved and substituted.
func (c *Client) resolveValue(ctx context.Context, v pendingVar) (string, error) {
	if !v.secret {
		return v.value, nil
	}

	parts, err := envtemplate.Parse(v.value)
	if err != nil {
		return "", fmt.Errorf("invalid value for %s (from %s): %w", v.name, v.source, err)
	}

	var b strings.Builder
	for _, p := range parts {
		if !p.IsRef {
			b.WriteString(p.Literal)
			continue
		}
		value, err := c.Get(ctx, p.Ref)
		if err != nil {
			return "", fmt.Errorf("failed to resolve secret %q for %s (from %s): %w", p.Ref, v.name, v.source, err)
		}
		b.WriteString(value)
	}
	return b.String(), nil
}

// parseEnvSpec splits each "NAME=ref" entry on the first '='. The ref may
// itself contain '=' characters. A missing '=' or an empty name is an error.
func parseEnvSpec(entries []string) ([]EnvVar, error) {
	vars := make([]EnvVar, 0, len(entries))
	for _, e := range entries {
		name, ref, found := strings.Cut(e, "=")
		if !found {
			return nil, fmt.Errorf("invalid --env %q: expected NAME=ref", e)
		}
		if name == "" {
			return nil, fmt.Errorf("invalid --env %q: empty name", e)
		}
		vars = append(vars, EnvVar{Name: name, Ref: ref})
	}
	return vars, nil
}
