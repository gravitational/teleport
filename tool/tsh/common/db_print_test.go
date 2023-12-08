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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_printDatabaseTable(t *testing.T) {
	t.Parallel()

	rows := [][]string{
		{"proxy", "cluster1", "db1", "describe db1", "postgres", "self-hosted", "localhost:5432", "[*]", "Env=dev", "tsh db connect db1"},
		{"proxy", "cluster1", "db2", "describe db2", "mysql", "self-hosted", "localhost:3306", "[alice] (Auto-provisioned)", "Env=prod", ""},
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
			expect: `Name Description  Allowed Users       Labels   Connect             
---- ------------ ------------------- -------- ------------------- 
db1  describe db1 [*]                 Env=dev  tsh db connect d... 
db2  describe db2 [alice] (Auto-pr... Env=prod                     

`,
		},
		{
			name: "tsh db ls -E Description -E Labels -E Connect",
			cfg: printDatabaseTableConfig{
				rows:                rows,
				showProxyAndCluster: false,
				verbose:             false,
				excludeColumns:      []string{"Description", "Labels", "Connect"},
			},
			expect: `Name Allowed Users              
---- -------------------------- 
db1  [*]                        
db2  [alice] (Auto-provisioned) 

`,
		},
		{
			name: "tsh db ls -v",
			cfg: printDatabaseTableConfig{
				rows:                rows,
				showProxyAndCluster: false,
				verbose:             true,
			},
			expect: `Name Description  Protocol Type        URI            Allowed Users              Labels   Connect            
---- ------------ -------- ----------- -------------- -------------------------- -------- ------------------ 
db1  describe db1 postgres self-hosted localhost:5432 [*]                        Env=dev  tsh db connect db1 
db2  describe db2 mysql    self-hosted localhost:3306 [alice] (Auto-provisioned) Env=prod                    

`,
		},
		{
			name: "tsh db ls -v -A",
			cfg: printDatabaseTableConfig{
				rows:                rows,
				showProxyAndCluster: true,
				verbose:             true,
			},
			expect: `Proxy Cluster  Name Description  Protocol Type        URI            Allowed Users              Labels   Connect            
----- -------- ---- ------------ -------- ----------- -------------- -------------------------- -------- ------------------ 
proxy cluster1 db1  describe db1 postgres self-hosted localhost:5432 [*]                        Env=dev  tsh db connect db1 
proxy cluster1 db2  describe db2 mysql    self-hosted localhost:3306 [alice] (Auto-provisioned) Env=prod                    

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
