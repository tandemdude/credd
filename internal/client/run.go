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
		value, err := c.resolveValue(ctx, v)
		if err != nil {
			return 1, err
		}
		env = append(env, v.Name+"="+value)
	}

	return runProcess(ctx, env, target)
}

// resolveValue interprets a var's raw value as a template and resolves every
// secret reference within it, returning the fully substituted string.
func (c *Client) resolveValue(ctx context.Context, v EnvVar) (string, error) {
	parts, err := parseTemplate(v.Ref)
	if err != nil {
		return "", fmt.Errorf("invalid --env value for %s: %w", v.Name, err)
	}

	var b strings.Builder
	for _, p := range parts {
		if !p.isRef {
			b.WriteString(p.literal)
			continue
		}
		value, err := c.Show(ctx, p.ref)
		if err != nil {
			return "", fmt.Errorf("failed to resolve secret %q for %s: %w", p.ref, v.Name, err)
		}
		b.WriteString(value)
	}
	return b.String(), nil
}

// templatePart is one segment of a parsed --env value: either literal text or
// a secret reference to be resolved and substituted in place.
type templatePart struct {
	literal string
	ref     string
	isRef   bool
}

// parseTemplate interprets a raw --env value. A value with no braces is treated
// as a single secret reference spanning the whole value (the original behaviour:
// a secret name or an op:// reference). A value containing braces is a template
// of literal text with {ref} placeholders; each placeholder's content is trimmed
// and resolved as a reference, while {{ and }} are literal single braces.
// Unmatched '{', a lone '}', and empty placeholders are errors.
func parseTemplate(value string) ([]templatePart, error) {
	if !strings.ContainsAny(value, "{}") {
		return []templatePart{{ref: value, isRef: true}}, nil
	}

	var parts []templatePart
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			parts = append(parts, templatePart{literal: lit.String()})
			lit.Reset()
		}
	}

	for i := 0; i < len(value); {
		switch c := value[i]; c {
		case '{':
			if i+1 < len(value) && value[i+1] == '{' {
				lit.WriteByte('{')
				i += 2
				continue
			}
			end := strings.IndexByte(value[i+1:], '}')
			if end < 0 {
				return nil, fmt.Errorf("unmatched '{' in %q", value)
			}
			ref := strings.TrimSpace(value[i+1 : i+1+end])
			if ref == "" {
				return nil, fmt.Errorf("empty placeholder in %q", value)
			}
			flush()
			parts = append(parts, templatePart{ref: ref, isRef: true})
			i += end + 2
		case '}':
			if i+1 < len(value) && value[i+1] == '}' {
				lit.WriteByte('}')
				i += 2
				continue
			}
			return nil, fmt.Errorf("lone '}' in %q", value)
		default:
			lit.WriteByte(c)
			i++
		}
	}
	flush()
	return parts, nil
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
