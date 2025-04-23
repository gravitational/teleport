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

package config

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestWriteSystemdUnitFile(t *testing.T) {
	flags := SystemdFlags{
		EnvironmentFile:          "/custom/env/dir/teleport",
		PIDFile:                  "/custom/pid/dir/teleport.pid",
		FileDescriptorLimit:      16384,
		TeleportInstallationFile: "/custom/install/dir/teleport",
	}

	stdout := new(bytes.Buffer)
	err := WriteSystemdUnitFile(flags, stdout)
	require.NoError(t, err)
	data := stdout.Bytes()
	if golden.ShouldSet() {
		golden.Set(t, data)
	}
	require.Equal(t, string(golden.Get(t)), stdout.String())
}
