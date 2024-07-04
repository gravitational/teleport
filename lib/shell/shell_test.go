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

package shell

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetShell(t *testing.T) {
	shell, err := GetLoginShell("root")
	require.NoError(t, err)
	require.True(t, shell == "/bin/bash" || shell == "/bin/sh")

	_, err = GetLoginShell("non-existent-user")
	require.ErrorContains(t, err, "unknown user")

	shell, err = GetLoginShell("nobody")
	require.NoError(t, err)
	require.Regexp(t, ".*(nologin|false)", shell)
}
