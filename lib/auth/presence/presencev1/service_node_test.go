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

	presencev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	accessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestPresenceServiceNodes(t *testing.T) {
	t.Parallel()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })
	presence := local.NewPresenceService(bk)

	roles := fakeScopedRoleReader{roles: map[string]*accessv1.ScopedRole{}}
	scopedReadRole := roles.createScopedRole("node-read-role", types.VerbRead)
	scopedListRole := roles.createScopedRole("node-list-role", types.VerbRead, types.VerbList)
	scopedDeleteRole := roles.createScopedRole("node-delete-role", types.VerbDelete)

	const (
		readUser   = "node-read-user"
		listUser   = "node-list-user"
		deleteUser = "node-delete-user"
	)

	unscopedNode := newNode(t, presence, scopes.QualifiedName{Name: "node", Scope: ""}, map[string]string{"env": "test"})
	scopedNode := newNode(t, presence, scopes.QualifiedName{Name: "node", Scope: testScope}, map[string]string{"env": "test"})
	orthogonalNode := newNode(t, presence, scopes.QualifiedName{Name: "node", Scope: testOrthogonalScope}, map[string]string{"env": "test"})

	t.Run("GetNode", func(t *testing.T) {
		for _, tt := range []struct {
			name       string
			authorizer authz.ScopedAuthorizer
			node       types.Server
			shouldFail bool
		}{
			{
				name:       "unscoped read-user fetching unscoped node",
				authorizer: newFakeAuthorizer(t, readUser, types.KindNode, types.VerbRead),
				node:       unscopedNode,
			},
			{
				name:       "unscoped read-user fetching scoped node",
				authorizer: newFakeAuthorizer(t, readUser, types.KindNode, types.VerbRead),
				node:       scopedNode,
			},
			{
				name:       "unscoped read-user fetching orthogonal scoped node",
				authorizer: newFakeAuthorizer(t, readUser, types.KindNode, types.VerbRead),
				node:       orthogonalNode,
			},
			{
				name:       "scoped read-user fetching scoped node",
				authorizer: newFakeScopedAuthorizer(t, readUser, testScope, roles, scopedReadRole),
				node:       scopedNode,
			},
			{
				name:       "scoped read-user fetching orthogonal scoped node",
				authorizer: newFakeScopedAuthorizer(t, readUser, testScope, roles, scopedReadRole),
				node:       orthogonalNode,
				shouldFail: true,
			},
			{
				name:       "scoped read-user fetching unscoped node",
				authorizer: newFakeScopedAuthorizer(t, readUser, testScope, roles, scopedReadRole),
				node:       unscopedNode,
				shouldFail: true,
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				srv := newPresenceService(t, backend{presence: presence}, tt.authorizer)
				res, err := srv.GetSSHServer(t.Context(), presencev1pb.GetSSHServerRequest_builder{
					Name:  tt.node.GetName(),
					Scope: tt.node.GetScope(),
				}.Build())
				if tt.shouldFail {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Empty(t, cmp.Diff(tt.node, res.GetServer()))
				}
			})
		}
	})

	allNodes := []types.Server{unscopedNode, scopedNode, orthogonalNode}
	t.Run("ListNodes", func(t *testing.T) {
		for _, tt := range []struct {
			name          string
			authorizer    authz.ScopedAuthorizer
			req           *presencev1pb.ListSSHServersRequest
			expectedNodes []types.Server
			shouldFail    bool
		}{
			{
				name:       "unscoped list-user listing all nodes without scope filter",
				authorizer: newFakeAuthorizer(t, listUser, types.KindNode, types.VerbRead, types.VerbList),
				req: presencev1pb.ListSSHServersRequest_builder{
					PageSize: 10,
				}.Build(),
				expectedNodes: []types.Server{unscopedNode},
			},
			{
				name:       "unscoped list-user listing all nodes filtering for all scopes",
				authorizer: newFakeAuthorizer(t, listUser, types.KindNode, types.VerbRead, types.VerbList),
				req: presencev1pb.ListSSHServersRequest_builder{
					PageSize: 10,
					ScopeFilter: scopesv1.Filter_builder{
						Mode: scopesv1.Mode_MODE_ALL,
					}.Build(),
				}.Build(),
				expectedNodes: allNodes,
			},
			{
				name:       "unscoped list-user listing nodes with scope filter",
				authorizer: newFakeAuthorizer(t, listUser, types.KindNode, types.VerbRead, types.VerbList),
				req: presencev1pb.ListSSHServersRequest_builder{
					PageSize: 10,
					ScopeFilter: scopesv1.Filter_builder{
						Scope: testScope,
						Mode:  scopesv1.Mode_MODE_EXACT,
					}.Build(),
				}.Build(),
				expectedNodes: []types.Server{scopedNode},
			},
			{
				name:       "unscoped list-user listing nodes across a scope subtree",
				authorizer: newFakeAuthorizer(t, listUser, types.KindNode, types.VerbRead, types.VerbList),
				req: presencev1pb.ListSSHServersRequest_builder{
					PageSize: 10,
					ScopeFilter: scopesv1.Filter_builder{
						Scope: testParentScope,
						Mode:  scopesv1.Mode_MODE_DESCENDANTS,
					}.Build(),
				}.Build(),
				expectedNodes: []types.Server{scopedNode, orthogonalNode},
			},
			{
				name:       "scoped list-user listing all nodes",
				authorizer: newFakeScopedAuthorizer(t, listUser, testScope, roles, scopedListRole),
				req: presencev1pb.ListSSHServersRequest_builder{
					PageSize: 10,
				}.Build(),
				expectedNodes: []types.Server{scopedNode},
			},
			{
				name:       "scoped list-user listing nodes with scope filter",
				authorizer: newFakeScopedAuthorizer(t, listUser, testScope, roles, scopedListRole),
				req: presencev1pb.ListSSHServersRequest_builder{
					PageSize: 10,
					ScopeFilter: scopesv1.Filter_builder{
						Scope: testScope,
						Mode:  scopesv1.Mode_MODE_EXACT,
					}.Build(),
				}.Build(),
				expectedNodes: []types.Server{scopedNode},
			},
			{
				name:       "scoped list-user listing nodes with orthogonal scope filter",
				authorizer: newFakeScopedAuthorizer(t, listUser, testScope, roles, scopedListRole),
				req: presencev1pb.ListSSHServersRequest_builder{
					PageSize: 10,
					ScopeFilter: scopesv1.Filter_builder{
						Scope: testOrthogonalScope,
						Mode:  scopesv1.Mode_MODE_EXACT,
					}.Build(),
				}.Build(),
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				srv := newPresenceService(t, backend{presence: presence}, tt.authorizer)
				res, err := srv.ListSSHServers(t.Context(), tt.req)
				if tt.shouldFail {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.ElementsMatch(t, tt.expectedNodes, res.GetServers())
				}
			})
		}
	})

	t.Run("DeleteNode", func(t *testing.T) {
		for _, tt := range []struct {
			name      string
			nodeScope string
			// reqScope is the scope sent in the delete request. Defaults to
			// nodeScope when empty, unless mismatchedScope is set.
			mismatchedScope string
			authorizer      authz.ScopedAuthorizer
			shouldFail      bool
		}{
			{
				name:       "unscoped read-user deleting unscoped node",
				nodeScope:  "",
				authorizer: newFakeAuthorizer(t, readUser, types.KindNode, types.VerbRead),
				shouldFail: true,
			},
			{
				name:       "unscoped read-user deleting scoped node",
				nodeScope:  testScope,
				authorizer: newFakeAuthorizer(t, readUser, types.KindNode, types.VerbRead),
				shouldFail: true,
			},
			{
				name:       "scoped read-user deleting scoped node",
				nodeScope:  testScope,
				authorizer: newFakeScopedAuthorizer(t, readUser, testScope, roles, scopedReadRole),
				shouldFail: true,
			},
			{
				name:       "scoped delete-user deleting unscoped node",
				nodeScope:  "",
				authorizer: newFakeScopedAuthorizer(t, deleteUser, testScope, roles, scopedDeleteRole),
				shouldFail: true,
			},
			{
				name:       "scoped delete-user deleting orthogonal scoped node",
				nodeScope:  testOrthogonalScope,
				authorizer: newFakeScopedAuthorizer(t, deleteUser, testScope, roles, scopedDeleteRole),
				shouldFail: true,
			},
			{
				name:       "scoped delete-user deleting scoped node",
				nodeScope:  testScope,
				authorizer: newFakeScopedAuthorizer(t, deleteUser, testScope, roles, scopedDeleteRole),
			},
			{
				name:       "unscoped delete-user deleting unscoped node",
				nodeScope:  "",
				authorizer: newFakeAuthorizer(t, deleteUser, types.KindNode, types.VerbDelete),
			},
			{
				name:       "unscoped delete-user deleting scoped node",
				nodeScope:  testScope,
				authorizer: newFakeAuthorizer(t, deleteUser, types.KindNode, types.VerbDelete),
			},
			{
				name:            "delete with mismatched scope",
				nodeScope:       testScope,
				mismatchedScope: testOrthogonalScope,
				authorizer:      newFakeAuthorizer(t, deleteUser, types.KindNode, types.VerbDelete),
				shouldFail:      true,
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				name := uuid.NewString()
				node := newNode(t, presence, scopes.QualifiedName{Name: name, Scope: tt.nodeScope}, map[string]string{"env": "test"})

				reqScope := tt.nodeScope
				if tt.mismatchedScope != "" {
					reqScope = tt.mismatchedScope
				}

				srv := newPresenceService(t, backend{presence: presence}, tt.authorizer)
				_, err := srv.DeleteSSHServer(t.Context(), presencev1pb.DeleteSSHServerRequest_builder{
					Name:  name,
					Scope: reqScope,
				}.Build())
				if tt.shouldFail {
					assert.Error(t, err)
					_, err := presence.GetSSHServer(t.Context(), presencev1pb.GetSSHServerRequest_builder{
						Name:  name,
						Scope: tt.nodeScope,
					}.Build())
					assert.NoError(t, err)
					return
				}

				assert.NoError(t, err)
				_, err = presence.GetSSHServer(t.Context(), presencev1pb.GetSSHServerRequest_builder{
					Name:  node.GetName(),
					Scope: tt.nodeScope,
				}.Build())
				assert.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)
			})
		}
	})
}

func newNode(t *testing.T, presence *local.PresenceService, sqn scopes.QualifiedName, labels map[string]string) types.Server {
	t.Helper()
	server := &types.ServerV2{
		Kind: types.KindNode,
		Metadata: types.Metadata{
			Name:   sqn.Name,
			Labels: labels,
		},
		Scope: sqn.Scope,
		Spec: types.ServerSpecV2{
			Addr:     "127.0.0.1:22",
			Hostname: sqn.Name,
		},
	}

	_, err := presence.UpsertNode(t.Context(), server)
	require.NoError(t, err)

	res, err := presence.GetSSHServer(t.Context(), presencev1pb.GetSSHServerRequest_builder{
		Name:  sqn.Name,
		Scope: sqn.Scope,
	}.Build())
	require.NoError(t, err)

	return res
}
