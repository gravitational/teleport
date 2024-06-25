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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// TestVersionUnmarshal verifies a version resource can be unmarshalled.
func TestVersionUnmarshal(t *testing.T) {
	t.Parallel()

	expected, err := types.NewVersion("1.2.3")
	require.NoError(t, err)
	data, err := utils.ToJSON([]byte(versionYAML))
	require.NoError(t, err)
	actual, err := UnmarshalVersion(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

// TestVersionMarshal verifies a marshaled version resource can be unmarshalled back.
func TestVersionMarshal(t *testing.T) {
	t.Parallel()

	expected, err := types.NewVersion("1.2.3")
	require.NoError(t, err)
	data, err := MarshalVersion(expected)
	require.NoError(t, err)
	actual, err := UnmarshalVersion(data)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

var versionYAML = `---
kind: version
version: v1
metadata:
  name: teleport_version
spec:
  version: 1.2.3
`
