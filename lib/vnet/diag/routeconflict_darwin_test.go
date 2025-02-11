//go:build darwin
// +build darwin

// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package diag

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDarwinRouting is just a smoke test to verify that DarwinRouting does not blow up when ran on
// an actual macOS machine.
func TestDarwinRouting(t *testing.T) {
	dr := DarwinRouting{}

	rds, err := dr.GetRouteDestinations()
	require.NoError(t, err)
	require.NotEmpty(t, rds)

	hasNonDefaultRoute := slices.ContainsFunc(rds, func(rd RouteDest) bool {
		return !rd.IsDefault()
	})
	// Check that the code that casts route.Inet4Addr to RouteDest isn't bugged and doesn't just cast
	// everything to 0.0.0.0.
	require.True(t, hasNonDefaultRoute, "Expected routes to include at least one route with a non-default destination, got: %+v", rds)
}
