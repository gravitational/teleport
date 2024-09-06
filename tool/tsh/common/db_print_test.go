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

package common

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func Test_printDatabaseTable(t *testing.T) {
	t.Parallel()

	rows := []databaseTableRow{
		{
			Proxy:        "proxy",
			Cluster:      "cluster1",
			DisplayName:  "db1",
			Description:  "describe db1",
			Protocol:     "postgres",
			Type:         "self-hosted",
			URI:          "localhost:5432",
			AllowedUsers: "[*]",
			Labels:       "Env=dev",
			Connect:      "tsh db connect db1",
		},
		{
			Proxy:         "proxy",
			Cluster:       "cluster1",
			DisplayName:   "db2",
			Description:   "describe db2",
			Protocol:      "mysql",
			Type:          "self-hosted",
			URI:           "localhost:3306",
			AllowedUsers:  "[alice]",
			DatabaseRoles: "[readonly]",
			Labels:        "Env=prod",
		},
	}

	tests := []struct {
		name   string
		cfg    printDatabaseTableConfig
		expect string
	}{
		{
			name: "tsh db ls",
			cfg: printDatabaseTableConfig{
				rows:                rows,
				showProxyAndCluster: false,
				verbose:             false,
			},
			// os.Stdin.Fd() fails during go test, so width is defaulted to 80 for truncated table.
			expect: `Name Description  Allowed Users Labels   Connect             
---- ------------ ------------- -------- ------------------- 
db1  describe db1 [*]           Env=dev  tsh db connect d... 
db2  describe db2 [alice]       Env=prod                     

`,
		},
		{
			name: "tsh db ls --verbose",
			cfg: printDatabaseTableConfig{
				rows:                rows,
				showProxyAndCluster: false,
				verbose:             true,
			},
			expect: `Name Description  Protocol Type        URI            Allowed Users Database Roles Labels   Connect            
---- ------------ -------- ----------- -------------- ------------- -------------- -------- ------------------ 
db1  describe db1 postgres self-hosted localhost:5432 [*]                          Env=dev  tsh db connect db1 
db2  describe db2 mysql    self-hosted localhost:3306 [alice]       [readonly]     Env=prod                    

`,
		},
		{
			name: "tsh db ls --verbose --all",
			cfg: printDatabaseTableConfig{
				rows:                rows,
				showProxyAndCluster: true,
				verbose:             true,
			},
			expect: `Proxy Cluster  Name Description  Protocol Type        URI            Allowed Users Database Roles Labels   Connect            
----- -------- ---- ------------ -------- ----------- -------------- ------------- -------------- -------- ------------------ 
proxy cluster1 db1  describe db1 postgres self-hosted localhost:5432 [*]                          Env=dev  tsh db connect db1 
proxy cluster1 db2  describe db2 mysql    self-hosted localhost:3306 [alice]       [readonly]     Env=prod                    

`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var sb strings.Builder

			cfg := test.cfg
			cfg.writer = &sb

			printDatabaseTable(cfg)
			require.Equal(t, test.expect, sb.String())
		})
	}
}

func Test_formatDatabaseRolesForDB(t *testing.T) {
	t.Parallel()

	db, err := types.NewDatabaseV3(types.Metadata{
		Name: "db",
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "localhost:5432",
	})
	require.NoError(t, err)

	dbWithAutoUser, err := types.NewDatabaseV3(types.Metadata{
		Name:   "dbWithAutoUser",
		Labels: map[string]string{"env": "prod"},
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "localhost:5432",
		AdminUser: &types.DatabaseAdminUser{
			Name: "teleport-admin",
		},
	})
	require.NoError(t, err)

	roleAutoUser := &types.RoleV6{
		Metadata: types.Metadata{Name: "auto-user", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				CreateDatabaseUserMode: types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			},
			Allow: types.RoleConditions{
				Namespaces:     []string{apidefaults.Namespace},
				DatabaseLabels: types.Labels{"env": []string{"prod"}},
				DatabaseRoles:  []string{"roleA", "roleB"},
				DatabaseNames:  []string{"*"},
				DatabaseUsers:  []string{types.Wildcard},
			},
		},
	}

	tests := []struct {
		name          string
		database      types.Database
		accessChecker services.AccessChecker
		expect        string
	}{
		{
			name:     "nil accessChecker",
			database: dbWithAutoUser,
			expect:   "(unknown)",
		},
		{
			name:     "roles",
			database: dbWithAutoUser,
			accessChecker: services.NewAccessCheckerWithRoleSet(&services.AccessInfo{
				Username: "alice",
			}, "clustername", services.RoleSet{roleAutoUser}),
			expect: "[roleA roleB]",
		},
		{
			name:     "db without admin user",
			database: db,
			accessChecker: services.NewAccessCheckerWithRoleSet(&services.AccessInfo{
				Username: "alice",
			}, "clustername", services.RoleSet{roleAutoUser}),
			expect: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expect, formatDatabaseRolesForDB(test.database, test.accessChecker))
		})
	}
}

func Test_maybeShowListDatabaseHint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cf       *CLIConf
		numRows  int
		wantHint bool
	}{
		{
			name: "show hint when number is big",
			cf: &CLIConf{
				command: "db ls",
			},
			numRows:  25,
			wantHint: true,
		},
		{
			name: "no hint for tsh db connect",
			cf: &CLIConf{
				command: "db connect",
			},
			numRows:  25,
			wantHint: false,
		},
		{
			name: "no hint when number is small",
			cf: &CLIConf{
				command: "db ls",
			},
			numRows:  15,
			wantHint: false,
		},
		{
			name: "no hint when search flag exists",
			cf: &CLIConf{
				command:        "db ls",
				SearchKeywords: "foo",
			},
			numRows:  25,
			wantHint: false,
		},
		{
			name: "no hint when query flag exists",
			cf: &CLIConf{
				command:             "db ls",
				PredicateExpression: "labels[\"key\"] == \"value\"",
			},
			numRows:  25,
			wantHint: false,
		},
		{
			name: "no hint when labels exist",
			cf: &CLIConf{
				command: "db ls",
				Labels:  "key=value",
			},
			numRows:  25,
			wantHint: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var buf bytes.Buffer

			maybeShowListDatabasesHint(test.cf, &buf, test.numRows)

			if test.wantHint {
				require.Contains(t, buf.String(), "hint")
			} else {
				require.Empty(t, buf.String())
			}
		})
	}
}
