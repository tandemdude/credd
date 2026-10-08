// Package envtemplate parses env var values that may embed secret references,
// e.g. "postgresql://user:{op://Vault/DB/password}@host/db".
package envtemplate

import (
	"fmt"
	"strings"
)

// Part is one segment of a parsed --env value: either literal text or
// a secret reference to be resolved and substituted in place.
type Part struct {
	Literal string
	Ref     string
	IsRef   bool
}

// Parse interprets a raw --env value. A value with no braces is treated
// as a single secret reference (e.g. op://vault/item/field) spanning the whole
// value. A value containing braces is a template
// of literal text with {ref} placeholders; each placeholder's content is trimmed
// and resolved as a reference, while {{ and }} are literal single braces.
// Unmatched '{', a lone '}', and empty placeholders are errors.
func Parse(value string) ([]Part, error) {
	if !strings.ContainsAny(value, "{}") {
		return []Part{{Ref: value, IsRef: true}}, nil
	}

	var parts []Part
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			parts = append(parts, Part{Literal: lit.String()})
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
			parts = append(parts, Part{Ref: ref, IsRef: true})
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

// Refs returns the secret references in parts, in order.
func Refs(parts []Part) []string {
	var refs []string
	for _, p := range parts {
		if p.IsRef {
			refs = append(refs, p.Ref)
		}
	}
	return refs
}
