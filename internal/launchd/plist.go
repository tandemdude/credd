package launchd

import (
	"bytes"
	"encoding/xml"
	"text/template"
)

// PlistOptions are the substitution values for the LaunchAgent plist.
type PlistOptions struct {
	Label      string
	ServerPath string
	ConfigPath string
	LogPath    string
}

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{.Label | xml}}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.ServerPath | xml}}</string>
		<string>--config</string>
		<string>{{.ConfigPath | xml}}</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardOutPath</key>
	<string>{{.LogPath | xml}}</string>
	<key>StandardErrorPath</key>
	<string>{{.LogPath | xml}}</string>
</dict>
</plist>
`

var plistTmpl = template.Must(template.New("plist").Funcs(template.FuncMap{
	"xml": func(s string) (string, error) {
		var b bytes.Buffer
		if err := xml.EscapeText(&b, []byte(s)); err != nil {
			return "", err
		}
		return b.String(), nil
	},
}).Parse(plistTemplate))

// renderPlist produces the LaunchAgent plist XML for opts.
func renderPlist(opts PlistOptions) ([]byte, error) {
	var b bytes.Buffer
	if err := plistTmpl.Execute(&b, opts); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
