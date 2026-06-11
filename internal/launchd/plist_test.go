package launchd

import (
	"strings"
	"testing"
)

func TestRenderPlist(t *testing.T) {
	data, err := renderPlist(PlistOptions{
		Label:      Label,
		ServerPath: "/opt/homebrew/bin/creddserver",
		ConfigPath: "/Users/me/.credd/config.toml",
		LogPath:    "/Users/me/Library/Logs/creddserver.log",
	})
	if err != nil {
		t.Fatalf("renderPlist: %v", err)
	}
	s := string(data)

	wants := []string{
		"<string>io.github.tandemdude.creddserver</string>",
		"<string>/opt/homebrew/bin/creddserver</string>",
		"<string>--config</string>",
		"<string>/Users/me/.credd/config.toml</string>",
		"<string>/Users/me/Library/Logs/creddserver.log</string>",
		"<key>RunAtLoad</key>",
		"<key>KeepAlive</key>",
		"<true/>",
		"<key>StandardOutPath</key>",
		"<key>StandardErrorPath</key>",
	}
	for _, w := range wants {
		if !strings.Contains(s, w) {
			t.Errorf("rendered plist missing %q\n---\n%s", w, s)
		}
	}
}

func TestRenderPlist_XMLEscapesValues(t *testing.T) {
	data, err := renderPlist(PlistOptions{
		Label:      Label,
		ServerPath: "/bin/creddserver",
		ConfigPath: "/Users/a&b/.credd/config.toml",
		LogPath:    "/Users/a&b/Library/Logs/creddserver.log",
	})
	if err != nil {
		t.Fatalf("renderPlist: %v", err)
	}
	if strings.Contains(string(data), "a&b") {
		t.Fatal("ampersand was not XML-escaped")
	}
	if !strings.Contains(string(data), "a&amp;b") {
		t.Fatal("expected escaped a&amp;b")
	}
}
