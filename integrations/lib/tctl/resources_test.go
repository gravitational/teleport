/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package tctl

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestReadRoleResources(t *testing.T) {
	const valid = `kind: role
version: v9
metadata:
  name: test
spec:
  allow:
    app_labels:
      env: ["dev"]
    app_resources:
      - allow_all: true
`
	resources, err := readResourcesYAMLOrJSON(strings.NewReader(valid))
	require.NoError(t, err)
	require.Len(t, resources, 1)
	role, ok := resources[0].(types.Role)
	require.True(t, ok)
	require.Equal(t, []types.AppResource{{AllowAll: true}}, role.GetAppResources(types.Allow))

	const unknownField = `kind: role
version: v9
metadata:
  name: test
spec:
  allow:
    app_resources:
      - allow_all: true
        paths: ["/admin"]
`
	_, err = readResourcesYAMLOrJSON(strings.NewReader(unknownField))
	require.ErrorContains(t, err, `app_resources rule has unknown field "paths"`)

	const unknownFieldOutsideAppResources = `kind: role
version: v9
metadata:
  name: test
spec:
  allow:
    app_labels:
      env: ["dev"]
    app_resources:
      - allow_all: true
    made_up_field: true
`
	resources, err = readResourcesYAMLOrJSON(strings.NewReader(unknownFieldOutsideAppResources))
	require.NoError(t, err)
	require.Len(t, resources, 1)
}
