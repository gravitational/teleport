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

package client

import (
	"testing"

	"github.com/stretchr/testify/require"

	scopedapp "github.com/gravitational/teleport/lib/scopes/app"
)

// TestAppCredentialName verifies the on-disk credential naming for app certs:
// unscoped apps keep their plain name, while scoped apps are keyed by their
// scope-qualified subdomain so same-named apps in different scopes don't
// overwrite each other.
func TestAppCredentialName(t *testing.T) {
	t.Parallel()

	// Unscoped apps use the plain app name.
	require.Equal(t, "grafana", AppCredentialName("grafana", ""))

	// Scoped apps use the scope-qualified subdomain.
	staging := AppCredentialName("grafana", "/staging")
	require.Equal(t, scopedapp.ScopedSubdomain("/staging", "grafana"), staging)
	require.NotEqual(t, "grafana", staging)

	// Deterministic.
	require.Equal(t, staging, AppCredentialName("grafana", "/staging"))

	// Same app name in different scopes must not collide.
	east := AppCredentialName("grafana", "/staging/east")
	require.NotEqual(t, staging, east)

	// Different app names in the same scope must not collide.
	require.NotEqual(t, staging, AppCredentialName("prometheus", "/staging"))
}
