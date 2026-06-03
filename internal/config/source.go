package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/urfave/cli/v3"
)

// tomlSource is a cli.ValueSource that reads a dotted key (e.g. "server.addr")
// from the TOML file at *path. A missing file, unparseable file, or absent key
// all resolve to (no value); malformed files are caught separately by Validate.
type tomlSource struct {
	key  string
	path *string
}

// TOMLSource returns a cli.ValueSource for key, reading the file pointed to by
// path at lookup time (so it honors a --config value resolved during parsing).
func TOMLSource(key string, path *string) cli.ValueSource {
	return tomlSource{key: key, path: path}
}

func (s tomlSource) Lookup() (string, bool) {
	if s.path == nil || *s.path == "" {
		return "", false
	}
	return lookupKey(*s.path, s.key)
}

func (s tomlSource) String() string   { return fmt.Sprintf("TOML key %q", s.key) }
func (s tomlSource) GoString() string { return fmt.Sprintf("config.tomlSource{key:%q}", s.key) }

// lookupKey reads a dotted key from the TOML file at path.
func lookupKey(path, key string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var raw map[string]any
	if err := toml.Unmarshal(b, &raw); err != nil {
		return "", false
	}

	var cur any = raw
	for _, part := range strings.Split(key, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		cur, ok = m[part]
		if !ok {
			return "", false
		}
	}
	s, ok := cur.(string)
	if !ok {
		return "", false
	}
	return s, true
}
