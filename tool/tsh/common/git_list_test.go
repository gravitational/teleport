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

package common

import (
	"bytes"
	"fmt"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/gravitational/teleport/api/types"
)

func makeGitServer(t *testing.T, gitHubOrg string) types.Server {
	t.Helper()
	server, err := types.NewGitHubServer(
		types.GitHubServerMetadata{
			Integration:  gitHubOrg,
			Organization: gitHubOrg,
		})
	require.NoError(t, err)
	return server
}

func TestGitListCommand(t *testing.T) {
	server1 := makeGitServer(t, "org1")
	server2 := makeGitServer(t, "org2")

	tests := []struct {
		name           string
		format         string
		fetchFn        func(*CLIConf, *client.TeleportClient) ([]types.Server, error)
		wantError      bool
		containsOutput []string
	}{
		{
			name: "fetch error",
			fetchFn: func(c *CLIConf, client *client.TeleportClient) ([]types.Server, error) {
				return nil, trace.ConnectionProblem(fmt.Errorf("bad connection"), "bad connection")
			},
			wantError: true,
		},
		{
			name: "text format",
			fetchFn: func(c *CLIConf, client *client.TeleportClient) ([]types.Server, error) {
				return []types.Server{server1, server2}, nil
			},
			containsOutput: []string{
				"Type   Organization Username URL",
				"GitHub org1         (n/a)*   https://github.com/org1",
				"GitHub org2         (n/a)*   https://github.com/org2",
			},
		},
		{
			name:   "json format",
			format: "json",
			fetchFn: func(c *CLIConf, client *client.TeleportClient) ([]types.Server, error) {
				return []types.Server{server1, server2}, nil
			},
			containsOutput: []string{
				`"kind": "git_server"`,
				`"hostname": "org1.github-org"`,
				`"hostname": "org2.github-org"`,
			},
		},
		{
			name:   "yaml format",
			format: "yaml",
			fetchFn: func(c *CLIConf, client *client.TeleportClient) ([]types.Server, error) {
				return []types.Server{server1, server2}, nil
			},
			containsOutput: []string{
				"- kind: git_server",
				"hostname: org1.github-org",
				"hostname: org2.github-org",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var capture bytes.Buffer
			cf := &CLIConf{
				OverrideStdout: &capture,
			}
			cmd := gitListCommand{
				format:  test.format,
				fetchFn: test.fetchFn,
			}
			err := cmd.run(cf)
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				for _, output := range test.containsOutput {
					require.Contains(t, capture.String(), output)
				}
			}
		})
	}
}
