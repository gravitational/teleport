// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/config"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
	"github.com/gravitational/teleport/tool/tctl/common/resources"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestScopedAssignmentListCommand(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_SCOPES", "yes")

	dynAddr := helpers.NewDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag: "false",
			},
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

	assignments := map[string]*scopedaccessv1.ScopedRoleAssignment{
		"alice-role1": {
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Version: types.V1,
			Scope:   "/testscope",
			Metadata: &headerv1.Metadata{
				Name: "alice-role1",
			},
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: "alice",
				Assignments: []*scopedaccessv1.Assignment{{
					Role:  "role1",
					Scope: "/testscope",
				}},
			},
		},
		"bob-role1": {
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Version: types.V1,
			Scope:   "/testscope",
			Metadata: &headerv1.Metadata{
				Name: "bob-role1",
			},
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: "bob",
				Assignments: []*scopedaccessv1.Assignment{{
					Role:  "role1",
					Scope: "/testscope",
				}},
			},
		},
		"charlie-role2": {
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Version: types.V1,
			Scope:   "/testscope",
			Metadata: &headerv1.Metadata{
				Name: "charlie-role2",
			},
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: "charlie",
				Assignments: []*scopedaccessv1.Assignment{{
					Role:  "role2",
					Scope: "/testscope",
				}},
			},
		},
		"charlie-role3": {
			Kind:    scopedaccess.KindScopedRoleAssignment,
			SubKind: scopedaccess.SubKindDynamic,
			Version: types.V1,
			Scope:   "/testscope",
			Metadata: &headerv1.Metadata{
				Name: "charlie-role3",
			},
			Spec: &scopedaccessv1.ScopedRoleAssignmentSpec{
				User: "charlie",
				Assignments: []*scopedaccessv1.Assignment{{
					Role:  "role3",
					Scope: "/testscope",
				}},
			},
		},
	}

	scopedClt := clt.ScopedAccessServiceClient()
	for name, assignment := range assignments {
		created, err := scopedClt.CreateScopedRoleAssignment(t.Context(), &scopedaccessv1.CreateScopedRoleAssignmentRequest{
			Assignment: assignment,
		})
		require.NoError(t, err)
		assignments[name] = created.Assignment
	}

	allAssignmentNames := slices.Collect(maps.Keys(assignments))
	slices.Sort(allAssignmentNames)

	collectExpectedAssignments := func(names []string) resources.Collection {
		slice := make([]*scopedaccessv1.ScopedRoleAssignment, 0, len(names))
		for _, name := range names {
			slice = append(slice, assignments[name])
		}
		return resources.NewScopedRoleAssignmentCollection(slice)
	}

	ctx := t.Context()
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		resp, err := scopedClt.ListScopedRoleAssignments(ctx, &scopedaccessv1.ListScopedRoleAssignmentsRequest{})
		require.NoError(t, err)
		require.Len(t, resp.Assignments, len(assignments))
	}, 10*time.Second, 50*time.Millisecond, "waiting for scoped role assignments to be present in cache")

	for _, tc := range []struct {
		desc                    string
		args                    []string
		expectedAssignmentNames []string
	}{
		{
			desc:                    "unfiltered list",
			args:                    []string{"assignments", "list"},
			expectedAssignmentNames: allAssignmentNames,
		},
		{
			desc:                    "unfiltered ls",
			args:                    []string{"assignments", "ls"},
			expectedAssignmentNames: allAssignmentNames,
		},
		{
			desc:                    "charlie",
			args:                    []string{"assignments", "ls", "--user", "charlie"},
			expectedAssignmentNames: []string{"charlie-role2", "charlie-role3"},
		},
		{
			desc:                    "charlie role2",
			args:                    []string{"assignments", "ls", "--user", "charlie", "--role", "role2"},
			expectedAssignmentNames: []string{"charlie-role2"},
		},
		{
			desc:                    "role1",
			args:                    []string{"assignments", "ls", "--role", "role1"},
			expectedAssignmentNames: []string{"alice-role1", "bob-role1"},
		},
		{
			desc: "nouser",
			args: []string{"assignments", "ls", "--user", "nouser"},
		},
		{
			desc: "norole",
			args: []string{"assignments", "ls", "--role", "norole"},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			t.Run("text", func(t *testing.T) {
				output, err := runScopedCommand(t, clt, tc.args)
				require.NoError(t, err)
				for _, name := range tc.expectedAssignmentNames {
					require.Contains(t, output.String(), name)
				}
			})
			t.Run("json", func(t *testing.T) {
				output, err := runScopedCommand(t, clt, append(tc.args, "-f", "json"))
				require.NoError(t, err)
				var expectedJSON strings.Builder
				require.NoError(t, writeJSON(collectExpectedAssignments(tc.expectedAssignmentNames), &expectedJSON))
				require.Equal(t, expectedJSON.String(), output.String())
			})
			t.Run("yaml", func(t *testing.T) {
				output, err := runScopedCommand(t, clt, append(tc.args, "-f", "yaml"))
				require.NoError(t, err)
				var expectedYAML strings.Builder
				require.NoError(t, writeYAML(collectExpectedAssignments(tc.expectedAssignmentNames), &expectedYAML))
				require.Equal(t, expectedYAML.String(), output.String())
			})
		})
	}
}
