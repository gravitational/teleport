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
	"slices"
	"strings"
	"text/template"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils/teleportassets"
)

const (
	// binUname is the default binary name for inspecting the host's OS.
	binUname = "uname"

	// binMktemp is the default binary name for creating temporary directories.
	binMktemp = "mktemp"

	// PrefixSUDO is a Teleport Command Prefix that executes with higher privileges
	// Use with caution.
	PrefixSUDO = "sudo"
)

var allowedCommandPrefix = []string{PrefixSUDO}

var (
	//go:embed oneoff.sh
	oneoffScript string

	// oneOffBashScript is a template that can generate oneoff scripts using the `oneoff.sh` shell script.
	oneOffBashScript = template.Must(template.New("oneoff").Parse(oneoffScript))
)

// OneOffScriptParams contains the required params to create a script that downloads and executes teleport binary.
type OneOffScriptParams struct {
	// TeleportCommandPrefix is a prefix command to use when calling teleport command.
	// Acceptable values are: "sudo"
	TeleportCommandPrefix string
	// binSudo contains the location for the sudo binary.
	// Used for testing.
	binSudo string

	// Entrypoint is the name of the binary from the teleport package. Defaults to "teleport", but can be set to
	// other binaries such as "teleport-update" or "tbot".
	Entrypoint string
	// EntrypointArgs is the arguments to pass to the Entrypoint binary.
	// Eg, 'version'
	EntrypointArgs string

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

	// TeleportFlavor is the teleport flavor to download.
	// Only OSS or Enterprise versions are allowed.
	// Possible values:
	// - teleport
	// - teleport-ent
	TeleportFlavor string

	// TeleportFIPS represents if the script should install a FIPS build of Teleport.
	TeleportFIPS bool

	// SuccessMessage is a message shown to the user after the one off is completed.
	SuccessMessage string
}

var validPackageNames = []string{types.PackageNameOSS, types.PackageNameEnt}

// CheckAndSetDefaults checks if the required params ara present.
func (p *OneOffScriptParams) CheckAndSetDefaults() error {
	if p.EntrypointArgs == "" {
		return trace.BadParameter("missing teleport args")
	}

	if p.Entrypoint == "" {
		p.Entrypoint = "teleport"
	}

	if p.BinUname == "" {
		p.BinUname = binUname
	}

	if p.BinMktemp == "" {
		p.BinMktemp = binMktemp
	}

	if p.binSudo == "" {
		p.binSudo = PrefixSUDO
	}

	if p.TeleportVersion == "" {
		p.TeleportVersion = "v" + api.Version
	}

	if p.CDNBaseURL == "" {
		p.CDNBaseURL = teleportassets.CDNBaseURL()
	}
	p.CDNBaseURL = strings.TrimRight(p.CDNBaseURL, "/")

	if p.TeleportFlavor == "" {
		p.TeleportFlavor = types.PackageNameOSS
		if modules.GetModules().BuildType() == modules.BuildEnterprise {
			p.TeleportFlavor = types.PackageNameEnt
		}
	}
	if !slices.Contains(validPackageNames, p.TeleportFlavor) {
		return trace.BadParameter("invalid teleport flavor, only %v are supported", validPackageNames)
	}

	if p.SuccessMessage == "" {
		p.SuccessMessage = "Completed successfully."
	}

	switch p.TeleportCommandPrefix {
	case PrefixSUDO:
		p.TeleportCommandPrefix = p.binSudo
	case "":
	default:
		return trace.BadParameter("invalid command prefix %q, only %v are supported", p.TeleportCommandPrefix, allowedCommandPrefix)
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
