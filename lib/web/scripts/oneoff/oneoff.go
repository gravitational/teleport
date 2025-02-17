/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package oneoff

import (
	"bytes"
	_ "embed"
	"slices"
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
		p.binSudo = "sudo"
	}

	if p.TeleportVersion == "" {
		p.TeleportVersion = "v" + api.Version
	}

	if p.CDNBaseURL == "" {
		p.CDNBaseURL = teleportassets.CDNBaseURL()
	}

	if p.TeleportFlavor == "" {
		p.TeleportFlavor = types.PackageNameOSS
		if modules.GetModules().BuildType() == modules.BuildEnterprise {
			p.TeleportFlavor = types.PackageNameEnt
		}
	}
	if !slices.Contains(types.PackageNameKinds, p.TeleportFlavor) {
		return trace.BadParameter("invalid teleport flavor, only %v are supported", types.PackageNameKinds)
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
