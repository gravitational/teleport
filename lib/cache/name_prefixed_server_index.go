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
	"github.com/gravitational/trace"
	"rsc.io/ordered"

	"github.com/gravitational/teleport/lib/backend"
)

// namePrefixedServerIndexKey constructs an index key prefixed by resourceName
// and positioned by host ID and server name.
func namePrefixedServerIndexKey(resourceName, hostID, serverName string) string {
	return string(ordered.Encode(resourceName, hostID, serverName))
}

// namePrefixedServerIndexRange returns the start and end index keys for a
// given resource name.
func namePrefixedServerIndexRange(resourceName string) (start, end string) {
	return string(ordered.Encode(resourceName)), string(ordered.Encode(resourceName, ordered.Inf))
}

// namePrefixedServerIndexKeyToListResourcesKey converts an index key from the
// name-prefixed server index into a pagination key for ListResources.
func namePrefixedServerIndexKeyToListResourcesKey(key, expectedResourceName string) (string, error) {
	var actualResourceName string
	rest, err := ordered.DecodePrefix([]byte(key), &actualResourceName)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Verify that the token's resource name matches the expected resource name.
	// This ensures that if the token is malformed or belongs to a different
	// resource, we don't return incorrect results.
	if actualResourceName != expectedResourceName {
		return "", trace.BadParameter("pagination token does not match the expected name")
	}

	if len(rest) == 0 {
		return "", nil
	}

	var hostID, serverName string
	if err := ordered.Decode(rest, &hostID, &serverName); err != nil {
		return "", trace.Wrap(err)
	}
	return backend.HostIDPaginationKey(hostID, serverName), nil
}

// listResourcesKeyToNamePrefixedServerIndexKey converts a ListResources
// pagination key into an index key for the name-prefixed server index.
func listResourcesKeyToNamePrefixedServerIndexKey(key, resourceName string) (string, error) {
	if key == "" {
		return "", nil
	}

	hostID, serverName, err := backend.ParseHostIDPaginationKey(key)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return namePrefixedServerIndexKey(resourceName, hostID, serverName), nil
}
