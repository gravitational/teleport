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

package presencev1_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	presencev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
)

func TestPresenceServiceNodes(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)

	readUser, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"node-read-user",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindNode},
				Verbs:     []string{types.VerbRead},
			},
		},
	)
	require.NoError(t, err)

	listUser, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"node-list-user",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindNode},
				Verbs:     []string{types.VerbRead, types.VerbList},
			},
		},
	)
	require.NoError(t, err)

	deleteUser, _, err := authtest.CreateUserAndRole(
		srv.Auth(),
		"node-delete-user",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindNode},
				Verbs:     []string{types.VerbDelete},
			},
		},
	)
	require.NoError(t, err)

	const (
		parentScope     = "/aa"
		scope           = "/aa/aa"
		orthogonalScope = "/aa/bb"
	)
	createScopedRole := func(name string, verbs []string) *scopedaccessv1.ScopedRole {
		scopedRole, err := srv.Auth().ScopedAccess().CreateScopedRole(t.Context(), scopedaccessv1.CreateScopedRoleRequest_builder{
			Role: scopedaccessv1.ScopedRole_builder{
				Kind:    scopedaccess.KindScopedRole,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: name,
				}.Build(),
				Scope: parentScope,
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{scope, orthogonalScope},
					// need ssh block because of the CanAccessSSHServer checks
					Ssh: scopedaccessv1.ScopedRoleSSH_builder{
						Logins: []string{"root"},
						Labels: []*labelv1.Label{
							labelv1.Label_builder{
								Name:   types.Wildcard,
								Values: []string{types.Wildcard},
							}.Build(),
						},
					}.Build(),
					Rules: []*scopedaccessv1.ScopedRule{
						scopedaccessv1.ScopedRule_builder{
							Resources: []string{types.KindNode},
							Verbs:     verbs,
						}.Build(),
					},
				}.Build(),
			}.Build(),
		}.Build())
		require.NoError(t, err)
		return scopedRole.GetRole()
	}

	scopedReadRole := createScopedRole("node-read-role", []string{types.VerbRead})
	scopedListRole := createScopedRole("node-list-role", []string{types.VerbRead, types.VerbList})
	scopedDeleteRole := createScopedRole("node-delete-role", []string{types.VerbDelete})

	createAssignment := func(role *scopedaccessv1.ScopedRole, username, assignedScope string) *scopedaccessv1.ScopedRoleAssignment {
		sra, err := srv.Auth().ScopedAccess().CreateScopedRoleAssignment(t.Context(), scopedaccessv1.CreateScopedRoleAssignmentRequest_builder{
			Assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind:    scopedaccess.KindScopedRoleAssignment,
				SubKind: scopedaccess.SubKindDynamic,
				Version: types.V1,
				Metadata: headerv1.Metadata_builder{
					Name: uuid.NewString(),
				}.Build(),
				Scope: assignedScope,
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					User: username,
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  scopes.QualifiedName{Scope: role.GetScope(), Name: role.GetMetadata().GetName()}.String(),
							Scope: assignedScope,
						}.Build(),
					},
				}.Build(),
			}.Build(),
		}.Build())
		require.NoError(t, err)
		return sra.GetAssignment()
	}

	waitForSRACache(
		t,
		srv,
		createAssignment(scopedReadRole, readUser.GetName(), scope),
		createAssignment(scopedListRole, listUser.GetName(), scope),
		createAssignment(scopedDeleteRole, deleteUser.GetName(), scope),
	)

	unscopedNode := newNode(t, srv.Auth(), "", "node", map[string]string{"env": "test"})
	scopedNode := newNode(t, srv.Auth(), scope, "node", map[string]string{"env": "test"})
	orthogonalNode := newNode(t, srv.Auth(), orthogonalScope, "node", map[string]string{"env": "test"})

	newClient := func(t *testing.T, identity authtest.TestIdentity) *authclient.Client {
		client, err := srv.NewClient(identity)
		require.NoError(t, err)
		t.Cleanup(func() { client.Close() })
		return client
	}

	t.Run("GetNode", func(t *testing.T) {
		for _, tt := range []struct {
			name       string
			client     *authclient.Client
			node       types.Server
			shouldFail bool
		}{
			{
				name:   "unscoped read-user fetching unscoped node",
				client: newClient(t, authtest.TestUser(readUser.GetName())),
				node:   unscopedNode,
			},
			{
				name:   "unscoped read-user fetching scoped node",
				client: newClient(t, authtest.TestUser(readUser.GetName())),
				node:   scopedNode,
			},
			{
				name:   "unscoped read-user fetching orthogonal scoped node",
				client: newClient(t, authtest.TestUser(readUser.GetName())),
				node:   orthogonalNode,
			},
			{
				name:   "scoped read-user fetching scoped node",
				client: newClient(t, authtest.TestScopedUser(readUser.GetName(), scope)),
				node:   scopedNode,
			},
			{
				name:       "scoped read-user fetching orthogonal scoped node",
				client:     newClient(t, authtest.TestScopedUser(readUser.GetName(), scope)),
				node:       orthogonalNode,
				shouldFail: true,
			},
			{
				name:       "scoped read-user fetching unscoped node",
				client:     newClient(t, authtest.TestScopedUser(readUser.GetName(), scope)),
				node:       unscopedNode,
				shouldFail: true,
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				res, err := tt.client.GetSSHServer(t.Context(), presencev1pb.GetSSHServerRequest_builder{
					Name:  tt.node.GetName(),
					Scope: tt.node.GetScope(),
				}.Build())
				if tt.shouldFail {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Empty(t, cmp.Diff(tt.node, res))
				}
			})
		}
	})

	allNodes := []types.Server{unscopedNode, scopedNode, orthogonalNode}
	t.Run("ListNodes", func(t *testing.T) {
		for _, tt := range []struct {
			name          string
			client        *authclient.Client
			req           *presencev1pb.ListSSHServersRequest
			expectedNodes []types.Server
			shouldFail    bool
		}{
			{
				name:   "unscoped list-user listing all nodes without scope filter",
				client: newClient(t, authtest.TestUser(listUser.GetName())),
				req: presencev1pb.ListSSHServersRequest_builder{
					PageSize: 10,
				}.Build(),
				expectedNodes: []types.Server{unscopedNode},
			},
			{
				name:   "unscoped list-user listing all nodes filtering for all scopes",
				client: newClient(t, authtest.TestUser(listUser.GetName())),
				req: presencev1pb.ListSSHServersRequest_builder{
					PageSize: 10,
					ScopeFilter: scopesv1.Filter_builder{
						Mode: scopesv1.Mode_MODE_ALL,
					}.Build(),
				}.Build(),
				expectedNodes: allNodes,
			},
			{
				name:   "unscoped list-user listing nodes with scope filter",
				client: newClient(t, authtest.TestUser(listUser.GetName())),
				req: presencev1pb.ListSSHServersRequest_builder{
					PageSize: 10,
					ScopeFilter: scopesv1.Filter_builder{
						Scope: scope,
						Mode:  scopesv1.Mode_MODE_EXACT,
					}.Build(),
				}.Build(),
				expectedNodes: []types.Server{scopedNode},
			},
			{
				name:   "unscoped list-user listing nodes across a scope subtree",
				client: newClient(t, authtest.TestUser(listUser.GetName())),
				req: presencev1pb.ListSSHServersRequest_builder{
					PageSize: 10,
					ScopeFilter: scopesv1.Filter_builder{
						Scope: parentScope,
						Mode:  scopesv1.Mode_MODE_DESCENDANTS,
					}.Build(),
				}.Build(),
				expectedNodes: []types.Server{scopedNode, orthogonalNode},
			},
			{
				name:   "scoped list-user listing all nodes",
				client: newClient(t, authtest.TestScopedUser(listUser.GetName(), scope)),
				req: presencev1pb.ListSSHServersRequest_builder{
					PageSize: 10,
				}.Build(),
				expectedNodes: []types.Server{scopedNode},
			},
			{
				name:   "scoped list-user listing nodes with scope filter",
				client: newClient(t, authtest.TestScopedUser(listUser.GetName(), scope)),
				req: presencev1pb.ListSSHServersRequest_builder{
					PageSize: 10,
					ScopeFilter: scopesv1.Filter_builder{
						Scope: scope,
						Mode:  scopesv1.Mode_MODE_EXACT,
					}.Build(),
				}.Build(),
				expectedNodes: []types.Server{scopedNode},
			},
			{
				name:   "scoped list-user listing nodes with orthogonal scope filter",
				client: newClient(t, authtest.TestScopedUser(listUser.GetName(), scope)),
				req: presencev1pb.ListSSHServersRequest_builder{
					PageSize: 10,
					ScopeFilter: scopesv1.Filter_builder{
						Scope: orthogonalScope,
						Mode:  scopesv1.Mode_MODE_EXACT,
					}.Build(),
				}.Build(),
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				nodes, _, err := tt.client.ListSSHServers(t.Context(), tt.req)
				if tt.shouldFail {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.ElementsMatch(t, tt.expectedNodes, nodes)
				}
			})
		}
	})

	// Runs last and without t.Parallel: it creates and removes nodes in the
	// same scopes the read subtests assert over.
	t.Run("DeleteNode", func(t *testing.T) {
		for _, tt := range []struct {
			name string
			// nodeScope is the scope the target node is created in.
			nodeScope string
			// reqScope is the scope sent in the delete request. Defaults to
			// nodeScope when empty, unless mismatchedScope is set.
			mismatchedScope string
			client          *authclient.Client
			shouldFail      bool
		}{
			{
				name:       "unscoped read-user deleting unscoped node",
				nodeScope:  "",
				client:     newClient(t, authtest.TestUser(readUser.GetName())),
				shouldFail: true,
			},
			{
				name:       "unscoped read-user deleting scoped node",
				nodeScope:  scope,
				client:     newClient(t, authtest.TestUser(readUser.GetName())),
				shouldFail: true,
			},
			{
				name:       "scoped read-user deleting scoped node",
				nodeScope:  scope,
				client:     newClient(t, authtest.TestScopedUser(readUser.GetName(), scope)),
				shouldFail: true,
			},
			{
				name:       "scoped delete-user deleting unscoped node",
				nodeScope:  "",
				client:     newClient(t, authtest.TestScopedUser(deleteUser.GetName(), scope)),
				shouldFail: true,
			},
			{
				name:       "scoped delete-user deleting orthogonal scoped node",
				nodeScope:  orthogonalScope,
				client:     newClient(t, authtest.TestScopedUser(deleteUser.GetName(), scope)),
				shouldFail: true,
			},
			{
				name:      "scoped delete-user deleting scoped node",
				nodeScope: scope,
				client:    newClient(t, authtest.TestScopedUser(deleteUser.GetName(), scope)),
			},
			{
				name:      "unscoped delete-user deleting unscoped node",
				nodeScope: "",
				client:    newClient(t, authtest.TestUser(deleteUser.GetName())),
			},
			{
				name:      "unscoped delete-user deleting scoped node",
				nodeScope: scope,
				client:    newClient(t, authtest.TestUser(deleteUser.GetName())),
			},
			{
				// The node exists, but not in the requested scope, so the
				// delete must not fall through to the node of the same name in
				// another scope.
				name:            "delete with mismatched scope",
				nodeScope:       scope,
				mismatchedScope: orthogonalScope,
				client:          newClient(t, authtest.TestUser(deleteUser.GetName())),
				shouldFail:      true,
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				name := uuid.NewString()
				node := newNode(t, srv.Auth(), tt.nodeScope, name, map[string]string{"env": "test"})

				reqScope := tt.nodeScope
				if tt.mismatchedScope != "" {
					reqScope = tt.mismatchedScope
				}

				err := tt.client.DeleteSSHServer(t.Context(), presencev1pb.DeleteSSHServerRequest_builder{
					Name:  name,
					Scope: reqScope,
				}.Build())
				if tt.shouldFail {
					assert.Error(t, err)
					_, err := srv.Auth().GetSSHServer(t.Context(), presencev1pb.GetSSHServerRequest_builder{
						Name:  name,
						Scope: tt.nodeScope,
					}.Build())
					assert.NoError(t, err)
					return
				}

				assert.NoError(t, err)
				_, err = srv.Auth().GetSSHServer(t.Context(), presencev1pb.GetSSHServerRequest_builder{
					Name:  node.GetName(),
					Scope: tt.nodeScope,
				}.Build())
				assert.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)
			})
		}
	})
}

func newNode(t *testing.T, srv *auth.Server, scope, name string, labels map[string]string) types.Server {
	t.Helper()

	node, err := types.NewServerWithLabels(name, types.KindNode, types.ServerSpecV2{
		Addr:     "127.0.0.1:22",
		Hostname: name,
	}, labels)
	require.NoError(t, err)

	server, ok := node.(*types.ServerV2)
	require.True(t, ok, "expected types.ServerV2")
	server.Scope = scope

	_, err = srv.UpsertNode(t.Context(), server)
	require.NoError(t, err)

	res, err := srv.GetSSHServer(t.Context(), presencev1pb.GetSSHServerRequest_builder{
		Name:  name,
		Scope: scope,
	}.Build())
	require.NoError(t, err)

	return res
}
