// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"testing"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestNamePrefixedServerIndexKey(t *testing.T) {
	t.Parallel()

	const (
		resourceName    = "resource-name"
		hostID          = "host-id"
		serverName      = "server-name"
		expectedListKey = hostID + backend.SeparatorString + serverName
	)

	start, end := namePrefixedServerIndexRange(resourceName)
	indexKey := namePrefixedServerIndexKey(resourceName, hostID, serverName)
	require.Less(t, start, indexKey)
	require.Less(t, indexKey, end)

	listKey, err := namePrefixedServerIndexKeyToListResourcesKey(start, resourceName)
	require.NoError(t, err)
	require.Empty(t, listKey, "expected no pagination key for the starting index key")

	listKey, err = namePrefixedServerIndexKeyToListResourcesKey(indexKey, resourceName)
	require.NoError(t, err)
	require.Equal(t, expectedListKey, listKey, "expected to get the list key back from the index key")

	namePrefixedKey, err := listResourcesKeyToNamePrefixedServerIndexKey(listKey, resourceName)
	require.NoError(t, err)
	require.Equal(t, indexKey, namePrefixedKey, "expected to get the same index key back from the pagination key")

	namePrefixedKey, err = listResourcesKeyToNamePrefixedServerIndexKey("", resourceName)
	require.NoError(t, err)
	require.Empty(t, namePrefixedKey)

	_, err = namePrefixedServerIndexKeyToListResourcesKey(indexKey, "other-name")
	require.ErrorAs(t, err, new(*trace.BadParameterError))
}
