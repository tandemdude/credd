package envtemplate

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	t.Run("no braces is a single whole-value ref", func(t *testing.T) {
		got, err := Parse("bar")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []Part{{Ref: "bar", IsRef: true}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("op reference with no braces is a single ref", func(t *testing.T) {
		got, err := Parse("op://Vault/item/field")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []Part{{Ref: "op://Vault/item/field", IsRef: true}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("literal text around a placeholder", func(t *testing.T) {
		got, err := Parse("postgres://u:{op://V/S/P}@h/db")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []Part{
			{Literal: "postgres://u:"},
			{Ref: "op://V/S/P", IsRef: true},
			{Literal: "@h/db"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("adjacent placeholders", func(t *testing.T) {
		got, err := Parse("{a}{b}")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []Part{
			{Ref: "a", IsRef: true},
			{Ref: "b", IsRef: true},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("whitespace inside a placeholder is trimmed", func(t *testing.T) {
		got, err := Parse("{ op://V/S/P }")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []Part{{Ref: "op://V/S/P", IsRef: true}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("escaped braces become literal single braces", func(t *testing.T) {
		got, err := Parse("a{{b}}c{x}")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []Part{
			{Literal: "a{b}c"},
			{Ref: "x", IsRef: true},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("escaped braces only, no placeholder", func(t *testing.T) {
		got, err := Parse("a{{b}}c")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []Part{{Literal: "a{b}c"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("unmatched open brace is an error", func(t *testing.T) {
		if _, err := Parse("a{b"); err == nil {
			t.Fatalf("expected error for unmatched '{'")
		}
	})

	t.Run("lone close brace is an error", func(t *testing.T) {
		if _, err := Parse("a}b"); err == nil {
			t.Fatalf("expected error for lone '}'")
		}
	})

	t.Run("empty placeholder is an error", func(t *testing.T) {
		if _, err := Parse("a{}b"); err == nil {
			t.Fatalf("expected error for empty placeholder")
		}
	})

	t.Run("whitespace-only placeholder is an error", func(t *testing.T) {
		if _, err := Parse("a{   }b"); err == nil {
			t.Fatalf("expected error for whitespace-only placeholder")
		}
	})
}

func TestRefs(t *testing.T) {
	parts, err := Parse("{op://V/u/f}:{{x}}:{op://V/p/f}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"op://V/u/f", "op://V/p/f"}
	if got := Refs(parts); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
