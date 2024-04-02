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
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTarballAndChecksumURL(t *testing.T) {
	artifact := "teleport-ent-v15.1.10-linux-arm64-bin.tar.gz"
	expectedTarballURL, err := url.Parse("https://cdn.teleport.dev/teleport-ent-v15.1.10-linux-arm64-bin.tar.gz")
	require.NoError(t, err)
	expectedChecksumURL, err := url.Parse("https://get.gravitational.com/teleport-ent-v15.1.10-linux-arm64-bin.tar.gz.sha256")
	require.NoError(t, err)

	teleportInstaller, err := NewTeleportInstaller(Config{
		TeleportBinDir: "/tmp/teleport/bin",
	})
	require.NoError(t, err)

	tarballURL, err := teleportInstaller.tarballURL(artifact)
	require.NoError(t, err)
	require.Equal(t, expectedTarballURL.String(), tarballURL.String())

	checksumURL, err := teleportInstaller.checksumURL(artifact)
	require.NoError(t, err)
	require.Equal(t, expectedChecksumURL.String(), checksumURL.String())
}

func TestSelectArtifact(t *testing.T) {
	ossArtifact := selectArtifact(Request{
		Version: "v15.1.10",
		Arch:    "arm64",
		OS:      "linux",
		Flavor:  "teleport",
	})
	require.Equal(t, "teleport-v15.1.10-linux-arm64-bin.tar.gz", ossArtifact)

	entArtifact := selectArtifact(Request{
		Version: "v15.1.10",
		Arch:    "arm64",
		OS:      "linux",
		Flavor:  "teleport-ent",
	})
	require.Equal(t, "teleport-ent-v15.1.10-linux-arm64-bin.tar.gz", entArtifact)
}
