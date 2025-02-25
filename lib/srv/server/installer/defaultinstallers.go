/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package installer

import (
	"strings"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/installers"
	"github.com/gravitational/teleport/lib/web/scripts/oneoff"
)

// DefaultInstaller represents the default installer script provided by teleport.
var DefaultInstaller = oneoffScriptToDefaultInstaller()

func oneoffScriptToDefaultInstaller() *types.InstallerV1 {
	argsList := []string{
		"install", "autodiscover-node",
		"--public-proxy-addr={{.PublicProxyAddr}}",
		"--teleport-package={{.TeleportPackage}}",
		"--repo-channel={{.RepoChannel}}",
		"--auto-upgrade={{.AutomaticUpgrades}}",
		"--azure-client-id={{.AzureClientID}}",
	}

	script, err := oneoff.BuildScript(oneoff.OneOffScriptParams{
		EntrypointArgs:        strings.Join(argsList, " "),
		SuccessMessage:        "Teleport is installed and running.",
		TeleportCommandPrefix: oneoff.PrefixSUDO,
	})
	if err != nil {
		panic(err)
	}

	return types.MustNewInstallerV1(installers.InstallerScriptName, script)
}
