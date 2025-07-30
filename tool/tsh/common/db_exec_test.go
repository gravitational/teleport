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

package common

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/common"
)

func Test_checkDatabaseExecInputFlags(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name       string
		cf         *CLIConf
		checkError require.ErrorAssertionFunc
	}{
		{
			name: "with database services",
			cf: &CLIConf{
				ParallelJobs:     1,
				DatabaseServices: "db1,db2",
			},
			checkError: require.NoError,
		},
		{
			name: "with search",
			cf: &CLIConf{
				ParallelJobs:   1,
				SearchKeywords: "dev",
			},
			checkError: require.NoError,
		},
		{
			name: "with labels",
			cf: &CLIConf{
				ParallelJobs: 1,
				Labels:       "env=dev",
			},
			checkError: require.NoError,
		},
		{
			name: "invalid max connections",
			cf: &CLIConf{
				ParallelJobs: 15,
				Labels:       "env=dev",
			},
			checkError: require.Error,
		},
		{
			name: "missing selection",
			cf: &CLIConf{
				ParallelJobs: 1,
			},
			checkError: require.Error,
		},
		{
			name: "too many selection options",
			cf: &CLIConf{
				ParallelJobs:     1,
				Labels:           "env=dev",
				DatabaseServices: "db1,db2",
			},
			checkError: require.Error,
		},
		{
			name: "missing output dir",
			cf: &CLIConf{
				ParallelJobs: 5,
				Labels:       "env=dev",
			},
			checkError: require.Error,
		},
		{
			name: "output dir exists",
			cf: &CLIConf{
				ParallelJobs: 5,
				OutputDir:    dir,
				Labels:       "env=dev",
			},
			checkError: require.Error,
		},
		{
			name: "max connections and output dir",
			cf: &CLIConf{
				ParallelJobs: 5,
				OutputDir:    filepath.Join(dir, "output"),
				Labels:       "env=dev",
			},
			checkError: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkError(t, checkDatabaseExecInputFlags(tt.cf))
		})
	}
}

func TestDatabaseExec(t *testing.T) {
	// Populate fake client.
	cert, err := tls.X509KeyPair([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)

	fakeClient := &fakeDatabaseExecClient{
		cert: cert,
		allDatabaseServers: []types.DatabaseServer{
			mustMakeDatabaseServerForEnv(t, "pg1", types.DatabaseProtocolPostgreSQL, "dev"),
			mustMakeDatabaseServerForEnv(t, "pg2", types.DatabaseProtocolPostgreSQL, "dev"),
			mustMakeDatabaseServerForEnv(t, "pg3", types.DatabaseProtocolPostgreSQL, "prod"),
			mustMakeDatabaseServerForEnv(t, "mysql", types.DatabaseProtocolMySQL, "prod"),
			mustMakeDatabaseServerForEnv(t, "mongo", types.DatabaseProtocolMongoDB, "staging"),
		},
	}

	// Commands are not actually being run but passed to cf.RunCommand.
	// Here just passing the query through the command for verification.
	dbQuery := "db-query"
	makeCommand := func(_ context.Context, dbInfo *databaseInfo, _ string, dbQuery string) (*exec.Cmd, error) {
		return exec.Command(dbQuery), nil
	}
	verifyDBQuery := func(cmd *exec.Cmd) error {
		if !slices.Equal(cmd.Args, []string{dbQuery}) {
			return trace.CompareFailed("expect %q but got %q", dbQuery, cmd.Args)
		}
		fmt.Fprintln(cmd.Stdout, dbQuery, "executed")
		return nil
	}

	tests := []struct {
		name                 string
		setup                func(*testing.T, *databaseExecCommand)
		wantError            string
		expectOutputContains []string
		verifyDir            func(t *testing.T, dir string)
	}{
		{
			name: "no databases found by search",
			setup: func(_ *testing.T, cmd *databaseExecCommand) {
				cmd.cf.SearchKeywords = "not-found"
			},
			wantError: "no databases found",
		},
		{
			name: "no databases found by names",
			setup: func(_ *testing.T, cmd *databaseExecCommand) {
				cmd.cf.DatabaseServices = "not-found"
			},
			wantError: "not found",
		},
		{
			name: "by names",
			setup: func(_ *testing.T, cmd *databaseExecCommand) {
				cmd.cf.DatabaseServices = "pg1,pg2,pg3"
			},
			expectOutputContains: []string{
				"Fetching databases by name",
				"Executing command for \"pg1\".",
				"db-query executed",
				"Executing command for \"pg2\".",
				"Executing command for \"pg3\".",
				"Summary:",
			},
		},
		{
			name: "by keyword",
			setup: func(_ *testing.T, cmd *databaseExecCommand) {
				cmd.cf.SearchKeywords = "mysql"
			},
			expectOutputContains: []string{
				"Found 1 database(s)",
				"Name  Description Protocol Labels",
				"----- ----------- -------- --------",
				"mysql             mysql    env=prod",
				"Executing command for \"mysql\".",
				"db-query executed",
			},
		},
		{
			name: "by env",
			setup: func(_ *testing.T, cmd *databaseExecCommand) {
				cmd.cf.Labels = "env=dev"
			},
			expectOutputContains: []string{
				"Found 2 database(s)",
				"Name Description Protocol Labels",
				"---- ----------- -------- -------",
				"pg1              postgres env=dev",
				"pg2              postgres env=dev",
				"Executing command for \"pg1\".",
				"db-query executed",
				"Executing command for \"pg2\".",
				"Summary:",
			},
		},
		{
			name: "output dir",
			setup: func(_ *testing.T, cmd *databaseExecCommand) {
				cmd.cf.DatabaseServices = "pg3,mysql"
				cmd.cf.OutputDir = filepath.Join(cmd.cf.HomePath, "test-output")
			},
			expectOutputContains: []string{
				"Fetching databases by name",
				"Executing command for \"pg3\". Output will be saved at",
				"Executing command for \"mysql\". Output will be saved at",
				"Summary:",
				"Summary is saved",
			},
			verifyDir: func(t *testing.T, dir string) {
				t.Helper()
				read, err := utils.ReadPath(filepath.Join(dir, "test-output", "pg3.output"))
				require.NoError(t, err)
				require.Equal(t, "db-query executed", strings.TrimSpace(string(read)))
				read, err = utils.ReadPath(filepath.Join(dir, "test-output", "mysql.output"))
				require.NoError(t, err)
				require.Equal(t, "db-query executed", strings.TrimSpace(string(read)))
				require.True(t, utils.FileExists(filepath.Join(dir, "test-output", "summary.json")))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			// Prep CLIConf.
			var capture bytes.Buffer
			writer := utils.NewSyncWriter(&capture)
			cf := &CLIConf{
				Proxy:           "proxy:3080",
				Context:         context.Background(),
				HomePath:        dir,
				ParallelJobs:    1,
				DatabaseUser:    "db-user",
				DatabaseName:    "db-name",
				DatabaseCommand: dbQuery,
				cmdRunner:       verifyDBQuery,
				OverrideStdout:  writer,
				overrideStderr:  writer,
				Confirm:         false,
			}

			// Prep command and sanity check.
			c := &databaseExecCommand{
				cf:          cf,
				client:      fakeClient,
				makeCommand: makeCommand,
			}
			tt.setup(t, c)
			mustCreateEmptyProfile(t, cf)
			c.tc, err = makeClient(cf)
			require.NoError(t, err)
			require.NoError(t, checkDatabaseExecInputFlags(c.cf))

			runError := c.run()
			if tt.wantError != "" {
				require.Error(t, runError)
				require.Contains(t, runError.Error(), tt.wantError)
				return
			}

			output := capture.String()
			for _, expect := range tt.expectOutputContains {
				require.Contains(t, output, expect)
			}

			if tt.verifyDir != nil {
				tt.verifyDir(t, dir)
			}
		})
	}

}

type fakeDatabaseExecClient struct {
	cert               tls.Certificate
	allDatabaseServers []types.DatabaseServer
}

func (c *fakeDatabaseExecClient) close() error {
	return nil
}
func (c *fakeDatabaseExecClient) getProfileStatus() *client.ProfileStatus {
	return &client.ProfileStatus{}
}
func (c *fakeDatabaseExecClient) getAccessChecker() services.AccessChecker {
	return services.NewAccessCheckerWithRoleSet(&services.AccessInfo{}, "clustername", services.NewRoleSet())
}
func (c *fakeDatabaseExecClient) issueCert(context.Context, *databaseInfo) (tls.Certificate, error) {
	return c.cert, nil
}
func (c *fakeDatabaseExecClient) listDatabasesWithFilter(ctx context.Context, req *proto.ListResourcesRequest) ([]types.Database, error) {
	filtered, err := matchResources(req, c.allDatabaseServers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.DatabaseServers(filtered).ToDatabases(), nil
}

func matchResources[R types.ResourceWithLabels](req *proto.ListResourcesRequest, s []R) ([]R, error) {
	filter := services.MatchResourceFilter{
		ResourceKind:   req.ResourceType,
		Labels:         req.Labels,
		SearchKeywords: req.SearchKeywords,
	}
	if req.PredicateExpression != "" {
		expression, err := services.NewResourceExpression(req.PredicateExpression)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		filter.PredicateExpression = expression
	}

	var filtered []R
	for _, r := range s {
		match, err := services.MatchResourceByFilters(r, filter, nil)
		if err != nil {
			return nil, trace.Wrap(err)
		} else if match {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
}

func mustMakeDatabaseServer(t *testing.T, db types.Database) types.DatabaseServer {
	t.Helper()

	dbV3, ok := db.(*types.DatabaseV3)
	require.True(t, ok)

	server, err := types.NewDatabaseServerV3(types.Metadata{
		Name: db.GetName(),
	}, types.DatabaseServerSpecV3{
		Version:  teleport.Version,
		Hostname: "hostname",
		HostID:   "host-id",
		Database: dbV3,
		ProxyIDs: []string{"proxy"},
	})
	require.NoError(t, err)
	return server
}

func mustMakeDatabaseForEnv(t *testing.T, name, protocol, env string) types.Database {
	t.Helper()
	db, err := types.NewDatabaseV3(
		types.Metadata{
			Name:   name,
			Labels: map[string]string{"env": env},
		},
		types.DatabaseSpecV3{
			Protocol: protocol,
			URI:      "localhost:12345",
		},
	)
	require.NoError(t, err)
	return db
}

func mustMakeDatabaseServerForEnv(t *testing.T, name, protocol, env string) types.DatabaseServer {
	t.Helper()
	db := mustMakeDatabaseForEnv(t, name, protocol, env)
	return mustMakeDatabaseServer(t, db)
}

func Test_ensureEachDatabase(t *testing.T) {
	devDB := mustMakeDatabaseForEnv(t, "dev", "postgres", "dev")
	stagingDB := mustMakeDatabaseForEnv(t, "staging", "postgres", "staging")
	prodDB1 := mustMakeDatabaseForEnv(t, "prod", "postgres", "prod")
	prodDB2 := mustMakeDatabaseForEnv(t, "prod", "postgres", "prod")
	common.SetDiscoveredResourceName(stagingDB, "staging") // edge case where discovered name is the same.
	common.SetDiscoveredResourceName(prodDB1, "prod-cloud1")
	common.SetDiscoveredResourceName(prodDB2, "prod-cloud2")

	tests := []struct {
		name                string
		inputNames          []string
		inputDatabases      []types.Database
		expectErrorContains string
	}{
		{
			name:           "exact match",
			inputNames:     []string{"dev", "staging"},
			inputDatabases: []types.Database{devDB, stagingDB},
		},
		{
			name:           "discovered name match",
			inputNames:     []string{"prod-cloud1", "prod-cloud2", "dev"},
			inputDatabases: []types.Database{devDB, prodDB1, prodDB2},
		},
		{
			name:                "database not found",
			inputNames:          []string{"dev", "staging", "prod-cloud5"},
			inputDatabases:      []types.Database{devDB, stagingDB},
			expectErrorContains: "\"prod-cloud5\" not found",
		},
		{
			name:                "ambiguous name",
			inputNames:          []string{"prod"},
			inputDatabases:      []types.Database{prodDB1, prodDB2},
			expectErrorContains: "\"prod\" matches multiple databases",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureEachDatabase(tt.inputNames, tt.inputDatabases)
			if tt.expectErrorContains == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectErrorContains)
			}
		})
	}
}

func Test_databaseExecSummary(t *testing.T) {
	summary := databaseExecSummary{}
	summary.add(databaseExecResult{
		RouteToDatabase: proto.RouteToDatabase{
			ServiceName: "db1",
			Protocol:    "postgres",
			Username:    "db-user",
		},
		Success: true,
	})
	summary.add(databaseExecResult{
		RouteToDatabase: proto.RouteToDatabase{
			ServiceName: "db2",
			Protocol:    "postgres",
			Username:    "db-user",
		},
		Error:    "some error",
		ExitCode: 1,
	})
	summary.add(databaseExecResult{
		RouteToDatabase: proto.RouteToDatabase{
			ServiceName: "db3",
			Protocol:    "postgres",
			Username:    "db-user",
		},
		Success: true,
	})

	var buf bytes.Buffer
	summary.print(&buf)
	require.Contains(t, buf.String(), "Summary: 2 of 3 succeeded")

	buf.Reset()
	dir := t.TempDir()
	expectPath := filepath.Join(dir, "summary.json")
	summary.printAndSave(&buf, dir)
	require.Contains(t, buf.String(), "Summary: 2 of 3 succeeded")
	require.Contains(t, buf.String(), fmt.Sprintf("Summary is saved at %q", expectPath))
	summaryData, err := os.ReadFile(expectPath)
	require.NoError(t, err)
	require.Equal(t, `{
  "databases": [
    {
      "database": {
        "service_name": "db1",
        "protocol": "postgres",
        "username": "db-user"
      },
      "command": "",
      "success": true,
      "exit_code": 0
    },
    {
      "database": {
        "service_name": "db2",
        "protocol": "postgres",
        "username": "db-user"
      },
      "command": "",
      "success": false,
      "error": "some error",
      "exit_code": 1
    },
    {
      "database": {
        "service_name": "db3",
        "protocol": "postgres",
        "username": "db-user"
      },
      "command": "",
      "success": true,
      "exit_code": 0
    }
  ],
  "success": 2,
  "failure": 1,
  "total": 3
}`, string(summaryData))
}
