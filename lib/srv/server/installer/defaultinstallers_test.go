/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package installer_test

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/installers"
	"github.com/gravitational/teleport/lib/srv/server/installer"
)

const defaultInstallerSnapshot = `#!/usr/bin/env sh
set -euo pipefail


INSTALL_SCRIPT_URL="https://teleport.example.com:443/scripts/install.sh"

echo "Offloading the installation part to the generic Teleport install script hosted at: $INSTALL_SCRIPT_URL"

TEMP_INSTALLER_SCRIPT="$(mktemp)"
curl -sSf "$INSTALL_SCRIPT_URL" -o "$TEMP_INSTALLER_SCRIPT"

chmod +x "$TEMP_INSTALLER_SCRIPT"

sudo "$TEMP_INSTALLER_SCRIPT" || (echo "The install script ($TEMP_INSTALLER_SCRIPT) returned a non-zero exit code" && exit 1)
rm "$TEMP_INSTALLER_SCRIPT"


echo "Configuring the Teleport agent"

set +x
sudo teleport install autodiscover-node --public-proxy-addr=teleport.example.com:443 --teleport-package=teleport-ent --repo-channel=stable/cloud --auto-upgrade=true --azure-client-id=`

// TestNewDefaultInstaller is a minimal
func TestNewDefaultInstaller(t *testing.T) {
	// Test setup.
	inputs := installers.Template{
		PublicProxyAddr:   "teleport.example.com:443",
		MajorVersion:      "v16",
		TeleportPackage:   "teleport-ent",
		RepoChannel:       "stable/cloud",
		AutomaticUpgrades: "true",
		AzureClientID:     "",
	}

	// Test execution: check that the template can be parsed.
	script := installer.NewDefaultInstaller.GetScript()
	installationTemplate, err := template.New("").Parse(script)
	require.NoError(t, err)

	// Test execution: render template.
	buf := &bytes.Buffer{}
	require.NoError(t, installationTemplate.Execute(buf, inputs))

	// Test validation: rendered template must look like the snapshot.
	require.Equal(t, defaultInstallerSnapshot, buf.String())
}
