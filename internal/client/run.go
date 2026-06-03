package client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
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

// Run resolves each env spec via the server, then executes target with the
// resolved variables appended to the inherited environment (resolved values
// override inherited ones on conflict). This relies on POSIX last-definition-wins
// semantics for duplicate env keys. It returns the child's exit code.
// Resolution failures are fail-fast: the target is never executed.
func (c *Client) Run(ctx context.Context, envSpecs, target []string) (int, error) {
	if len(target) == 0 {
		return 1, errors.New("no command provided after '--'")
	}

	vars, err := parseEnvSpec(envSpecs)
	if err != nil {
		return 1, err
	}

	env := os.Environ()
	for _, v := range vars {
		value, err := c.Show(ctx, v.Ref)
		if err != nil {
			return 1, fmt.Errorf("failed to resolve secret %q for %s: %w", v.Ref, v.Name, err)
		}
		env = append(env, v.Name+"="+value)
	}

	return runProcess(ctx, env, target)
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
