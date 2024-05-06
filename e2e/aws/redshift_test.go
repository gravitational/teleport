/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package e2e

import (
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
)

func testRedshiftServerless(t *testing.T) {
	t.Skip("skipped until we fix the spacelift stack")
	t.Parallel()
	accessRole := mustGetEnv(t, rssAccessRoleEnv)
	discoveryRole := mustGetEnv(t, rssDiscoveryRoleEnv)
	cluster := makeDBTestCluster(t, accessRole, discoveryRole, types.AWSMatcherRedshiftServerless)

	// wait for the database to be discovered
	rssDBName := mustGetEnv(t, rssNameEnv)
	rssEndpointName := mustGetEnv(t, rssEndpointNameEnv)
	waitForDatabases(t, cluster.Process, rssDBName, rssEndpointName)

	t.Run("connect as iam role", func(t *testing.T) {
		// test connections
		rssRoute := tlsca.RouteToDatabase{
			ServiceName: rssDBName,
			Protocol:    defaults.ProtocolPostgres,
			Username:    mustGetEnv(t, rssDBUserEnv),
			Database:    "postgres",
		}
		t.Run("via proxy", func(t *testing.T) {
			t.Parallel()
			postgresConnTest(t, cluster, hostUser, rssRoute, "select 1")
		})
		t.Run("via local proxy", func(t *testing.T) {
			t.Parallel()
			postgresLocalProxyConnTest(t, cluster, hostUser, rssRoute, "select 1")
		})
	})
}
