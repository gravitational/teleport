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

package scopes_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/scopes"
)

// TestFeatures verifies the expected behavior of scope feature parsing.
func TestFeatures(t *testing.T) {
	require.False(t, scopes.FeaturesFromEnv().Enabled)
	require.False(t, scopes.FeaturesFromEnv().AgentPinEnabled)
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")
	require.True(t, scopes.FeaturesFromEnv().Enabled)
	require.False(t, scopes.FeaturesFromEnv().AgentPinEnabled)
	require.NoError(t, scopes.Features{Enabled: true}.AssertEnabled())
	require.Error(t, scopes.Features{}.AssertEnabled())
	t.Setenv("TELEPORT_UNSTABLE_AGENT_SCOPE_PIN", "yes")
	require.True(t, scopes.FeaturesFromEnv().Enabled)
	require.True(t, scopes.FeaturesFromEnv().AgentPinEnabled)
}
