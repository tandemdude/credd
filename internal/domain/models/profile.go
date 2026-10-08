package models

// VarKind says how a profile var's value is turned into an env var value.
type VarKind string

const (
	// VarKindPlain values are used verbatim.
	VarKindPlain VarKind = "plain"
	// VarKindSecret values are a secret reference, or a template containing
	// secret references, resolved from an external secret manager at run time.
	VarKindSecret VarKind = "secret"
)

// ProfileVar is a single NAME=value pairing within a profile.
type ProfileVar struct {
	Name  string
	Value string
	Kind  VarKind
}

// Profile is a named collection of env vars.
type Profile struct {
	Name        string
	Description string
	Vars        []ProfileVar
}
