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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

type listedScopedToken struct {
	Kind     string
	Version  string
	Metadata struct {
		Name    string
		Expires timestamppb.Timestamp
		ID      uint
	}
	Spec struct {
		Roles      []string
		JoinMethod string
	}
}

func TestScopedTokens(t *testing.T) {
	dynAddr := helpers.NewDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
		},
	}

	process := makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))
	clt, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clt.Close() })

	scopeFlags := []string{"--scope=/aa", "--assign-scope=/aa/bb"}
	// Test all output formats of "tokens add".
	buf, err := runScopedCommand(t, clt, append([]string{"tokens", "add", "--type=node"}, scopeFlags...))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(buf.String(), "The invite token:"))

	buf, err = runScopedCommand(t, clt, append([]string{"tokens", "add", "--type=node", "--format", teleport.Text}, scopeFlags...))
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(buf.String(), "\n"))

	buf, err = runScopedCommand(t, clt, append([]string{"tokens", "add", "--type=node", "--format", teleport.JSON}, scopeFlags...))
	require.NoError(t, err)
	out := mustDecodeJSON[addedToken](t, buf)

	require.Len(t, out.Roles, 1)
	require.Equal(t, types.KindNode, strings.ToLower(out.Roles[0]))

	buf, err = runScopedCommand(t, clt, append([]string{"tokens", "add", "--type=node", "--format", teleport.YAML}, scopeFlags...))
	require.NoError(t, err)
	out = mustDecodeYAML[addedToken](t, buf)

	require.Len(t, out.Roles, 1)
	require.Equal(t, types.KindNode, strings.ToLower(out.Roles[0]))

	// with --mode
	buf, err = runScopedCommand(t, clt, append([]string{"tokens", "add", "--type=node", "--format", teleport.JSON, "--mode", "single_use"}, scopeFlags...))
	require.NoError(t, err)
	out = mustDecodeJSON[addedToken](t, buf)

	// Create a GCP scoped token to verify it appears in listings.
	_, err = clt.CreateScopedToken(t.Context(), &joiningv1.ScopedToken{
		Kind:    types.KindScopedToken,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "gcp-test-token",
		},
		Scope: "/aa",
		Spec: &joiningv1.ScopedTokenSpec{
			Roles:         []string{types.KindNode},
			AssignedScope: "/aa/bb",
			JoinMethod:    "gcp",
			UsageMode:     "unlimited",
			Gcp: &joiningv1.GCP{
				Allow: []*joiningv1.GCP_Rule{
					{
						ProjectIds:      []string{"example-project-123456"},
						Locations:       []string{"us-west1"},
						ServiceAccounts: []string{"123456789-compute@developer.gserviceaccount.com"},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	// Test all output formats of "tokens ls".
	buf, err = runScopedCommand(t, clt, []string{"tokens", "ls"})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(buf.String(), "Token "))
	require.Equal(t, 8, strings.Count(buf.String(), "\n")) // account for header lines

	buf, err = runScopedCommand(t, clt, []string{"tokens", "ls", "--format", teleport.Text})
	require.NoError(t, err)
	require.Equal(t, 6, strings.Count(buf.String(), "\n"))

	buf, err = runScopedCommand(t, clt, []string{"tokens", "ls", "--format", teleport.JSON})
	require.NoError(t, err)
	jsonOut := mustDecodeJSON[[]listedScopedToken](t, buf)
	require.Len(t, jsonOut, 6)

	buf, err = runScopedCommand(t, clt, []string{"tokens", "ls", "--format", teleport.YAML})
	require.NoError(t, err)
	yamlOut := []listedScopedToken{}
	mustDecodeYAMLDocuments(t, buf, &yamlOut)
	require.Len(t, yamlOut, 6)
	require.Equal(t, jsonOut, yamlOut)
}
