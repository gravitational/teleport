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

package gcp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsGCPZoneInLocation(t *testing.T) {
	t.Parallel()
	passingCases := []struct {
		name     string
		location string
		zone     string
	}{
		{
			name:     "matching zone",
			location: "us-west1-b",
			zone:     "us-west1-b",
		},
		{
			name:     "matching region",
			location: "us-west1",
			zone:     "us-west1-b",
		},
	}
	for _, tc := range passingCases {
		t.Run("accept "+tc.name, func(t *testing.T) {
			require.True(t, isGCPZoneInLocation(tc.location, tc.zone))
		})
	}

	failingCases := []struct {
		name     string
		location string
		zone     string
	}{
		{
			name:     "non-matching zone",
			location: "europe-southwest1-b",
			zone:     "us-west1-b",
		},
		{
			name:     "non-matching region",
			location: "europe-southwest1",
			zone:     "us-west1-b",
		},
		{
			name:     "malformed location",
			location: "us",
			zone:     "us-west1-b",
		},
		{
			name:     "similar but non-matching region",
			location: "europe-west1",
			zone:     "europe-west12-a",
		},
		{
			name:     "empty zone",
			location: "us-west1",
			zone:     "",
		},
		{
			name:     "empty location",
			location: "",
			zone:     "us-west1-b",
		},
		{
			name:     "invalid zone",
			location: "us-west1",
			zone:     "us-west1",
		},
	}
	for _, tc := range failingCases {
		t.Run("reject "+tc.name, func(t *testing.T) {
			require.False(t, isGCPZoneInLocation(tc.location, tc.zone))
		})
	}
}
