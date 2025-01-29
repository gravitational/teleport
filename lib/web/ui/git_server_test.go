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
package ui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/ui"
)

func TestMakeGitServer(t *testing.T) {
	server, err := types.NewGitHubServer(types.GitHubServerMetadata{
		Integration:  "my-integration",
		Organization: "my-org",
	})
	require.NoError(t, err)

	expect := GitServer{
		ClusterName: "cluster",
		Kind:        "git_server",
		SubKind:     "github",
		Addr:        "github.com:22",
		Name:        server.GetName(),
		Hostname:    "my-org.teleport-github-org",
		GitHub: &GitHubServerMetadata{
			Integration:  "my-integration",
			Organization: "my-org",
		},
		// Internal labels get filtered.
		Labels: []ui.Label{},
	}
	require.Equal(t, expect, MakeGitServer("cluster", server, false))
}
