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
	"slices"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/services"
)

func Test_fetchMCPServers(t *testing.T) {
	devLabels := map[string]string{"env": "dev"}
	prodLabels := map[string]string{"env": "prod"}

	nonMCPAppServer := mustMakeNewAppServer(t, mustMakeNewAppV3(t,
		types.Metadata{
			Name:   "non-mcp-app",
			Labels: devLabels,
		},
		types.AppSpecV3{
			URI: "https://example.com",
		},
	), "host1")

	devMCPAppHost1 := mustMakeNewAppServer(t, mustMakeMCPAppWithNameAndLabels(t, "dev", devLabels), "host1")
	devMCPAppHost2 := mustMakeNewAppServer(t, mustMakeMCPAppWithNameAndLabels(t, "dev", devLabels), "host2")
	proMCPApp1 := mustMakeNewAppServer(t, mustMakeMCPAppWithNameAndLabels(t, "prod-1", prodLabels), "host1")
	proMCPApp2 := mustMakeNewAppServer(t, mustMakeMCPAppWithNameAndLabels(t, "prod-2", prodLabels), "host1")

	fakeClient := &fakeResourcesClient{
		resources: []types.ResourceWithLabels{
			proMCPApp1, nonMCPAppServer, devMCPAppHost1, devMCPAppHost2, proMCPApp2,
		},
	}

	tests := []struct {
		name         string
		searchConfig client.Config
		wantNames    []string
	}{
		{
			name: "no match",
			searchConfig: client.Config{
				Labels: map[string]string{"env": "not-found"},
			},
			wantNames: nil,
		},
		{
			name:         "all",
			searchConfig: client.Config{},
			wantNames:    []string{"dev", "prod-1", "prod-2"},
		},
		{
			name: "by label",
			searchConfig: client.Config{
				Labels: map[string]string{"env": "prod"},
			},
			wantNames: []string{"prod-1", "prod-2"},
		},
		{
			name: "by keywords",
			searchConfig: client.Config{
				SearchKeywords: []string{"prod"},
			},
			wantNames: []string{"prod-1", "prod-2"},
		},
		{
			name: "by predicate",
			searchConfig: client.Config{
				PredicateExpression: "name==\"dev\"",
			},
			wantNames: []string{"dev"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &client.TeleportClient{
				Config: tt.searchConfig,
			}
			tc.Tracer = tracing.NoopTracer(teleport.ComponentTSH)

			mcpServers, err := fetchMCPServers(context.Background(), tc, fakeClient)
			require.NoError(t, err)
			require.Equal(t, tt.wantNames, slices.Collect(types.ResourceNames(mcpServers)))
		})
	}
}

// Test_mcpListCommand tests "tsh mcp ls".
// Note that mcpListCommand.fetch is not interesting to test and some of its
// logic is tested separately (see Test_fetchMCPServers above). Thus, this test
// mocks fetch results and tests mcpListCommand.print.
func Test_mcpListCommand(t *testing.T) {
	devLabels := map[string]string{"env": "dev"}
	mcpServers := []types.Application{
		mustMakeMCPAppWithNameAndLabels(t, "allow-read", devLabels),
		mustMakeMCPAppWithNameAndLabels(t, "deny-write", devLabels),
	}
	accessChecker := fakeMCPServerAccessChecker{}

	tests := []struct {
		name       string
		cf         *CLIConf
		wantOutput string
	}{
		{
			name: "text format",
			cf:   &CLIConf{},
			wantOutput: `Name       Description Type  Labels  
---------- ----------- ----- ------- 
allow-read description stdio env=dev 
deny-write description stdio env=dev 

`,
		},
		{
			name: "text format in verbose",
			cf: &CLIConf{
				Verbose: true,
			},
			wantOutput: `Name       Description Type  Labels  Command Args Allowed Tools          
---------- ----------- ----- ------- ------- ---- ---------------------- 
allow-read description stdio env=dev test    arg  [read_*]               
deny-write description stdio env=dev test    arg  [*], except: [write_*] 

`,
		},
		{
			name: "JSON format",
			cf: &CLIConf{
				Format: "json",
			},
			wantOutput: `[
  {
    "kind": "app",
    "sub_kind": "mcp",
    "version": "v3",
    "metadata": {
      "name": "allow-read",
      "description": "description",
      "labels": {
        "env": "dev"
      }
    },
    "spec": {
      "uri": "mcp+stdio://",
      "insecure_skip_verify": false,
      "mcp": {
        "command": "test",
        "args": [
          "arg"
        ],
        "run_as_host_user": "test"
      }
    },
    "permissions": {
      "mcp": {
        "tools": {
          "allowed": [
            "read_*"
          ]
        }
      }
    }
  },
  {
    "kind": "app",
    "sub_kind": "mcp",
    "version": "v3",
    "metadata": {
      "name": "deny-write",
      "description": "description",
      "labels": {
        "env": "dev"
      }
    },
    "spec": {
      "uri": "mcp+stdio://",
      "insecure_skip_verify": false,
      "mcp": {
        "command": "test",
        "args": [
          "arg"
        ],
        "run_as_host_user": "test"
      }
    },
    "permissions": {
      "mcp": {
        "tools": {
          "allowed": [
            "*"
          ],
          "denied": [
            "write_*"
          ]
        }
      }
    }
  }
]
`,
		},
		{
			name: "YAML format",
			cf: &CLIConf{
				Format: "yaml",
			},
			wantOutput: `- kind: app
  metadata:
    description: description
    labels:
      env: dev
    name: allow-read
  permissions:
    mcp:
      tools:
        allowed:
        - read_*
  spec:
    insecure_skip_verify: false
    mcp:
      args:
      - arg
      command: test
      run_as_host_user: test
    uri: mcp+stdio://
  sub_kind: mcp
  version: v3
- kind: app
  metadata:
    description: description
    labels:
      env: dev
    name: deny-write
  permissions:
    mcp:
      tools:
        allowed:
        - '*'
        denied:
        - write_*
  spec:
    insecure_skip_verify: false
    mcp:
      args:
      - arg
      command: test
      run_as_host_user: test
    uri: mcp+stdio://
  sub_kind: mcp
  version: v3

`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			cf := tt.cf
			cf.OverrideStdout = &buf
			cf.Context = context.Background()

			cmd := &mcpListCommand{
				cf:            tt.cf,
				mcpServers:    mcpServers,
				accessChecker: accessChecker,
			}

			err := cmd.print()
			require.NoError(t, err)
			require.Equal(t, tt.wantOutput, buf.String())
		})
	}
}

type fakeResourcesClient struct {
	resources []types.ResourceWithLabels
}

func (f *fakeResourcesClient) GetResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error) {
	filtered, err := matchResources(req, f.resources)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	paginatedResources, err := services.MakePaginatedResources(req.ResourceType, filtered, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.ListResourcesResponse{
		Resources:  paginatedResources,
		TotalCount: int32(len(filtered)),
	}, nil
}

func mustMakeNewAppServer(t *testing.T, app *types.AppV3, host string) types.AppServer {
	t.Helper()
	appServer, err := types.NewAppServerV3FromApp(app, host, host)
	require.NoError(t, err)
	return appServer
}

func mustMakeMCPAppWithNameAndLabels(t *testing.T, name string, labels map[string]string) *types.AppV3 {
	t.Helper()
	return mustMakeNewAppV3(t,
		types.Metadata{
			Name:        name,
			Description: "description",
			Labels:      labels,
		},
		types.AppSpecV3{
			MCP: &types.MCP{
				Command:       "test",
				Args:          []string{"arg"},
				RunAsHostUser: "test",
			},
		},
	)
}

type fakeMCPServerAccessChecker struct {
	services.AccessChecker
}

func (f fakeMCPServerAccessChecker) EnumerateMCPTools(app types.Application) services.EnumerationResult {
	switch app.GetName() {
	case "allow-read":
		return services.NewEnumerationResultFromEntities([]string{"read_*"}, nil)
	case "deny-write":
		return services.NewEnumerationResultFromEntities([]string{"*"}, []string{"write_*"})
	default:
		return services.NewEnumerationResult()
	}
}
