/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package oneoff

import (
	"bytes"
	_ "embed"
	"text/template"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
)

const (
	// teleportCDNLocation is the Teleport's CDN URL
	// This is used to download the Teleport Binary
	teleportCDNLocation = "https://cdn.teleport.dev"

	// binUname is the default binary name for inspecting the host's OS.
	binUname = "uname"

	// binMktemp is the default binary name for creating temporary directories.
	binMktemp = "mktemp"
)

var (
	//go:embed oneoff.sh
	oneoffScript string

	// oneOffBashScript is a template that can generate oneoff scripts using the `oneoff.sh` shell script.
	oneOffBashScript = template.Must(template.New("oneoff").Parse(oneoffScript))
)

// OneOffScriptParams contains the required params to create a script that downloads and executes teleport binary.
type OneOffScriptParams struct {
	// TeleportArgs is the arguments to pass to the teleport binary.
	// Eg, 'version'
	TeleportArgs string

	// BinUname is the binary used to get OS name and Architecture of the host.
	// Defaults to `uname`.
	BinUname string
	// BinUname is the binary used to create a temporary directory, used to download the files.
	// Defaults to `mktemp`.
	BinMktemp string
	// CDNBaseURL is the URL used to download the teleport tarball.
	// Defaults to `https://cdn.teleport.dev`
	CDNBaseURL string
	// TeleportVersion is the teleport version to download.
	// Defaults to v<currentTeleportVersion>
	// Eg, v13.1.0
	TeleportVersion string
}

// CheckAndSetDefaults checks if the required params ara present.
func (p *OneOffScriptParams) CheckAndSetDefaults() error {
	if p.TeleportArgs == "" {
		return trace.BadParameter("missing teleport args")
	}

	if p.BinUname == "" {
		p.BinUname = binUname
	}

	if p.BinMktemp == "" {
		p.BinMktemp = binMktemp
	}

	if p.CDNBaseURL == "" {
		p.CDNBaseURL = teleportCDNLocation
	}

	if p.TeleportVersion == "" {
		p.TeleportVersion = "v" + teleport.Version
	}

	return nil
}

// BuildScript creates a Bash script that:
// - downloads and extracts teleport binary
// - runs `teleport ` with the args defined in the  query param
func BuildScript(p OneOffScriptParams) (string, error) {
	if err := p.CheckAndSetDefaults(); err != nil {
		return "", trace.Wrap(err)
	}

	out := &bytes.Buffer{}
	if err := oneOffBashScript.Execute(out, p); err != nil {
		return "", trace.Wrap(err)
	}

	return out.String(), nil
}
