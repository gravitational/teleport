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

package auth

import (
	"context"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"github.com/vulcand/predicate/builder"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

func TestUpsertDeleteRoleEventsEmitted(t *testing.T) {
	t.Parallel()
	clientAddr := &net.TCPAddr{IP: net.IPv4(10, 255, 0, 0)}
	ctx := authz.ContextWithClientSrcAddr(context.Background(), clientAddr)
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	// test create new role
	role, err := types.NewRole("test-role", types.RoleSpecV6{
		Options: types.RoleOptions{},
		Allow:   types.RoleConditions{},
	})
	require.NoError(t, err)

	// Creating a role should emit a RoleCreatedEvent.
	role, err = p.a.CreateRole(ctx, role)
	require.NoError(t, err)
	require.Equal(t, events.RoleCreatedEvent, p.mockEmitter.LastEvent().GetType())
	createEvt := p.mockEmitter.LastEvent().(*apievents.RoleCreate)
	require.Equal(t, role.GetName(), createEvt.Name)
	require.Equal(t, clientAddr.String(), createEvt.ConnectionMetadata.RemoteAddr)
	p.mockEmitter.Reset()

	// Upserting a role should emit a RoleCreatedEvent.
	role, err = p.a.UpsertRole(ctx, role)
	require.NoError(t, err)
	require.Equal(t, events.RoleCreatedEvent, p.mockEmitter.LastEvent().GetType())
	createEvt = p.mockEmitter.LastEvent().(*apievents.RoleCreate)
	require.Equal(t, role.GetName(), createEvt.Name)
	require.Equal(t, clientAddr.String(), createEvt.ConnectionMetadata.RemoteAddr)
	p.mockEmitter.Reset()

	// Updating a role should emit a RoleUpdatedEvent.
	role.SetLogins(types.Allow, []string{"llama"})
	role, err = p.a.UpdateRole(ctx, role)
	require.NoError(t, err)
	require.Equal(t, events.RoleUpdatedEvent, p.mockEmitter.LastEvent().GetType())
	updateEvt := p.mockEmitter.LastEvent().(*apievents.RoleUpdate)
	require.Equal(t, role.GetName(), updateEvt.Name)
	require.Equal(t, clientAddr.String(), updateEvt.ConnectionMetadata.RemoteAddr)
	p.mockEmitter.Reset()

	// Deleting a role should emit a RoleDeletedEvent.
	err = p.a.DeleteRole(ctx, role.GetName())
	require.NoError(t, err)
	require.Equal(t, events.RoleDeletedEvent, p.mockEmitter.LastEvent().GetType())
	deleteEvt := p.mockEmitter.LastEvent().(*apievents.RoleDelete)
	require.Equal(t, role.GetName(), deleteEvt.Name)
	require.Equal(t, clientAddr.String(), deleteEvt.ConnectionMetadata.RemoteAddr)
	p.mockEmitter.Reset()

	// When deleting a nonexistent role, no event should be emitted.
	err = p.a.DeleteRole(ctx, role.GetName())
	require.True(t, trace.IsNotFound(err))
	require.Nil(t, p.mockEmitter.LastEvent())
}

func TestUpsertDeleteDependentRoles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	// test create new role
	role, err := types.NewRole("test-role", types.RoleSpecV6{
		Options: types.RoleOptions{},
		Allow:   types.RoleConditions{},
	})
	require.NoError(t, err)

	// Create a role and assign it to a user.
	role, err = p.a.UpsertRole(ctx, role)
	require.NoError(t, err)
	user, err := types.NewUser("test-user")
	require.NoError(t, err)
	user.AddRole(role.GetName())
	_, err = p.a.CreateUser(ctx, user)
	require.NoError(t, err)

	// Deletion should fail.
	require.ErrorIs(t, p.a.DeleteRole(ctx, role.GetName()), errDeleteRoleUser)
	require.NoError(t, p.a.DeleteUser(ctx, user.GetName()))

	clusterName, err := p.a.GetClusterName()
	require.NoError(t, err)

	// Update the user CA with the role.
	ca, err := p.a.GetCertAuthority(ctx, types.CertAuthID{Type: types.UserCA, DomainName: clusterName.GetClusterName()}, true)
	require.NoError(t, err)
	ca.AddRole(role.GetName())
	require.NoError(t, p.a.UpsertCertAuthority(ctx, ca))

	// Deletion should fail.
	require.ErrorIs(t, p.a.DeleteRole(ctx, role.GetName()), errDeleteRoleCA)

	// Clear out the roles for the CA.
	ca.SetRoles([]string{})
	require.NoError(t, p.a.UpsertCertAuthority(ctx, ca))

	// Create an access list that references the role.
	accessList, err := accesslist.NewAccessList(header.Metadata{
		Name: "test-access-list",
	}, accesslist.Spec{
		Title: "simple",
		Owners: []accesslist.Owner{
			{Name: "some-user"},
		},
		Grants: accesslist.Grants{
			Roles: []string{role.GetName()},
		},
		Audit: accesslist.Audit{
			NextAuditDate: time.Now(),
		},
		MembershipRequires: accesslist.Requires{
			Roles: []string{role.GetName()},
		},
		OwnershipRequires: accesslist.Requires{
			Roles: []string{role.GetName()},
		},
	})
	require.NoError(t, err)
	_, err = p.a.UpsertAccessList(ctx, accessList)
	require.NoError(t, err)

	// Deletion should fail due to the grant.
	require.ErrorIs(t, p.a.DeleteRole(ctx, role.GetName()), errDeleteRoleAccessList)

	accessList.Spec.Grants.Roles = []string{"non-existent-role"}
	_, err = p.a.UpsertAccessList(ctx, accessList)
	require.NoError(t, err)

	// Deletion should fail due to membership requires.
	require.ErrorIs(t, p.a.DeleteRole(ctx, role.GetName()), errDeleteRoleAccessList)

	accessList.Spec.MembershipRequires.Roles = []string{"non-existent-role"}
	_, err = p.a.UpsertAccessList(ctx, accessList)
	require.NoError(t, err)

	// Deletion should fail due to ownership requires.
	require.ErrorIs(t, p.a.DeleteRole(ctx, role.GetName()), errDeleteRoleAccessList)

	accessList.Spec.OwnershipRequires.Roles = []string{"non-existent-role"}
	_, err = p.a.UpsertAccessList(ctx, accessList)
	require.NoError(t, err)

	// Deletion should succeed
	require.NoError(t, p.a.DeleteRole(ctx, role.GetName()))
}

// newRole will create a new role for testing purposes.
func newRole(t *testing.T, name string, labels map[string]string, allowRc types.RoleConditions, denyRc types.RoleConditions) types.Role {
	t.Helper()

	role, err := types.NewRole(name, types.RoleSpecV6{
		Options: types.RoleOptions{},
		Allow:   allowRc,
		Deny:    denyRc,
	})
	require.NoError(t, err)
	role.SetStaticLabels(labels)

	return role
}

func rcWithRoleRule(verbs ...string) types.RoleConditions {
	return rcWithRoleRuleWhere(verbs, "")
}

func rcWithRoleRuleWhere(verbs []string, where string) types.RoleConditions {
	return types.RoleConditions{
		Rules: []types.Rule{
			{
				Resources: []string{types.KindRole},
				Verbs:     verbs,
				Where:     where,
			},
		},
	}
}

func TestCreateRole(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		userRole     types.Role
		startingRole types.Role
		createRole   types.Role
		wantErr      require.ErrorAssertionFunc
		want         types.Role
	}{
		{
			name:       "create role",
			userRole:   newRole(t, "urole", nil, rcWithRoleRule(types.VerbCreate), types.RoleConditions{}),
			createRole: newRole(t, "create", nil, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			wantErr:    require.NoError,
			want:       newRole(t, "create", nil, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
		},
		{
			name:       "create role denied",
			userRole:   newRole(t, "urole", nil, types.RoleConditions{}, types.RoleConditions{}),
			createRole: newRole(t, "create", nil, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name:         "create role collision",
			userRole:     newRole(t, "urole", nil, rcWithRoleRule(types.VerbCreate), types.RoleConditions{}),
			startingRole: newRole(t, "create", nil, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			createRole:   newRole(t, "create", nil, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAlreadyExists(err))
			},
		},
		{
			name:         "create role collision with access denied",
			userRole:     newRole(t, "urole", nil, types.RoleConditions{}, types.RoleConditions{}),
			startingRole: newRole(t, "create", nil, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			createRole:   newRole(t, "create", nil, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "create role where clause",
			userRole: newRole(t, "urole", nil, rcWithRoleRuleWhere([]string{types.VerbCreate},
				builder.Equals(
					builder.Identifier(`resource.metadata.labels["label"]`),
					builder.String("value"),
				).String(),
			), types.RoleConditions{}),
			createRole: newRole(t, "create", map[string]string{
				"label": "value",
			}, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			wantErr: require.NoError,
			want: newRole(t, "create", map[string]string{
				"label": "value",
			}, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
		},
		{
			name: "create role where clause denied",
			userRole: newRole(t, "urole", nil, rcWithRoleRuleWhere([]string{types.VerbCreate},
				builder.Equals(
					builder.Identifier(`resource.metadata.labels["label"]`),
					builder.String("value"),
				).String(),
			), types.RoleConditions{}),
			createRole: newRole(t, "create", nil, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			srv := newTestTLSServer(t)
			as := srv.AuthServer.AuthServer

			// Create a starting role if needed. This will be used to test collisions.
			if test.startingRole != nil {
				_, err := as.CreateRole(ctx, test.startingRole)
				require.NoError(t, err)
			}

			user, err := types.NewUser("user")
			require.NoError(t, err)
			user.AddRole(test.userRole.GetName())

			// Create a test user with the given user role.
			_, err = as.CreateRole(ctx, test.userRole)
			require.NoError(t, err)
			_, err = as.CreateUser(ctx, user)
			require.NoError(t, err)
			clt, err := srv.NewClient(TestUser(user.GetName()))
			require.NoError(t, err)

			got, err := clt.CreateRole(ctx, test.createRole)
			test.wantErr(t, err)
			if test.want == nil {
				require.Nil(t, got)
			} else {
				require.Empty(t, cmp.Diff(test.want, got,
					cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
				))
			}
		})
	}
}

func TestUpdateRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		userRole     types.Role
		startingRole types.Role
		updateRole   types.Role
		wantErr      require.ErrorAssertionFunc
		want         types.Role
	}{
		{
			name:         "update role",
			userRole:     newRole(t, "urole", nil, rcWithRoleRule(types.VerbUpdate), types.RoleConditions{}),
			startingRole: newRole(t, "update", nil, types.RoleConditions{}, types.RoleConditions{}),
			updateRole:   newRole(t, "update", nil, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			wantErr:      require.NoError,
			want:         newRole(t, "update", nil, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
		},
		{
			name:         "update role denied",
			userRole:     newRole(t, "urole", nil, types.RoleConditions{}, types.RoleConditions{}),
			startingRole: newRole(t, "update", nil, types.RoleConditions{}, types.RoleConditions{}),
			updateRole:   newRole(t, "update", nil, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name:       "update role not found",
			userRole:   newRole(t, "urole", nil, rcWithRoleRule(types.VerbUpdate), types.RoleConditions{}),
			updateRole: newRole(t, "update", nil, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				// This returns a compare failed instead of a NotFound. In the interests of not breaking anything,
				// I'll maintain this for now.
				require.True(t, trace.IsCompareFailed(err))
			},
		},
		{
			name:       "update role not found with access denied",
			userRole:   newRole(t, "urole", nil, types.RoleConditions{}, types.RoleConditions{}),
			updateRole: newRole(t, "update", nil, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "update role where clause",
			userRole: newRole(t, "urole", nil, rcWithRoleRuleWhere([]string{types.VerbUpdate},
				builder.Equals(
					builder.Identifier(`resource.metadata.labels["label"]`),
					builder.String("value"),
				).String(),
			), types.RoleConditions{}),
			startingRole: newRole(t, "update", map[string]string{
				"label": "value",
			}, types.RoleConditions{}, types.RoleConditions{}),
			updateRole: newRole(t, "update", map[string]string{
				"label": "value",
			}, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			wantErr: require.NoError,
			want: newRole(t, "update", map[string]string{
				"label": "value",
			}, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
		},
		{
			name: "update role where clause denied",
			userRole: newRole(t, "urole", nil, rcWithRoleRuleWhere([]string{types.VerbUpdate},
				builder.Equals(
					builder.Identifier(`resource.metadata.labels["label"]`),
					builder.String("value"),
				).String(),
			), types.RoleConditions{}),
			startingRole: newRole(t, "update", nil, types.RoleConditions{}, types.RoleConditions{}),
			updateRole:   newRole(t, "update", nil, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "update role old role doesn't match where",
			userRole: newRole(t, "urole", nil, rcWithRoleRuleWhere([]string{types.VerbUpdate},
				builder.Equals(
					builder.Identifier(`resource.metadata.labels["label"]`),
					builder.String("value"),
				).String(),
			), types.RoleConditions{}),
			startingRole: newRole(t, "update", nil, types.RoleConditions{}, types.RoleConditions{}),
			updateRole: newRole(t, "update", map[string]string{
				"label": "value",
			}, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "update role new role doesn't match where",
			userRole: newRole(t, "urole", nil, rcWithRoleRuleWhere([]string{types.VerbUpdate},
				builder.Equals(
					builder.Identifier(`resource.metadata.labels["label"]`),
					builder.String("value"),
				).String(),
			), types.RoleConditions{}),
			startingRole: newRole(t, "update", map[string]string{
				"label": "value",
			}, types.RoleConditions{}, types.RoleConditions{}),
			updateRole: newRole(t, "update", nil, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			srv := newTestTLSServer(t)
			as := srv.AuthServer.AuthServer

			// Create a starting role if needed. This will be used to test collisions.
			var revision string
			if test.startingRole != nil {
				startingRole, err := as.CreateRole(ctx, test.startingRole)
				require.NoError(t, err)
				revision = startingRole.GetMetadata().Revision
			}

			user, err := types.NewUser("user")
			require.NoError(t, err)
			user.AddRole(test.userRole.GetName())

			// Create a test user with the given user role.
			_, err = as.CreateRole(ctx, test.userRole)
			require.NoError(t, err)
			_, err = as.CreateUser(ctx, user)
			require.NoError(t, err)
			clt, err := srv.NewClient(TestUser(user.GetName()))
			require.NoError(t, err)
			test.updateRole.SetRevision(revision)

			got, err := clt.UpdateRole(ctx, test.updateRole)
			test.wantErr(t, err)
			if test.want == nil {
				require.Nil(t, got)
			} else {
				require.Empty(t, cmp.Diff(test.want, got,
					cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
				))
			}
		})
	}
}

func TestUpsertRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		userRole     types.Role
		startingRole types.Role
		upsertRole   types.Role
		wantErr      require.ErrorAssertionFunc
		want         types.Role
	}{
		{
			name:       "create role",
			userRole:   newRole(t, "urole", nil, rcWithRoleRule(types.VerbCreate), types.RoleConditions{}),
			upsertRole: newRole(t, "upsert", nil, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			wantErr:    require.NoError,
			want:       newRole(t, "upsert", nil, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
		},
		{
			name:         "update role",
			userRole:     newRole(t, "urole", nil, rcWithRoleRule(types.VerbUpdate), types.RoleConditions{}),
			startingRole: newRole(t, "upsert", nil, types.RoleConditions{}, types.RoleConditions{}),
			upsertRole:   newRole(t, "upsert", nil, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			wantErr:      require.NoError,
			want:         newRole(t, "upsert", nil, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
		},
		{
			name:         "update role denied",
			userRole:     newRole(t, "urole", nil, types.RoleConditions{}, types.RoleConditions{}),
			startingRole: newRole(t, "upsert", nil, types.RoleConditions{}, types.RoleConditions{}),
			upsertRole:   newRole(t, "upsert", nil, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "update role where clause",
			userRole: newRole(t, "urole", nil, rcWithRoleRuleWhere([]string{types.VerbUpdate},
				builder.Equals(
					builder.Identifier(`resource.metadata.labels["label"]`),
					builder.String("value"),
				).String(),
			), types.RoleConditions{}),
			startingRole: newRole(t, "upsert", map[string]string{
				"label": "value",
			}, types.RoleConditions{}, types.RoleConditions{}),
			upsertRole: newRole(t, "upsert", map[string]string{
				"label": "value",
			}, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			wantErr: require.NoError,
			want: newRole(t, "upsert", map[string]string{
				"label": "value",
			}, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
		},
		{
			name: "update role where clause denied",
			userRole: newRole(t, "urole", nil, rcWithRoleRuleWhere([]string{types.VerbUpdate},
				builder.Equals(
					builder.Identifier(`resource.metadata.labels["label"]`),
					builder.String("value"),
				).String(),
			), types.RoleConditions{}),
			startingRole: newRole(t, "upsert", nil, types.RoleConditions{}, types.RoleConditions{}),
			upsertRole:   newRole(t, "upsert", nil, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "update role old role doesn't match where",
			userRole: newRole(t, "urole", nil, rcWithRoleRuleWhere([]string{types.VerbUpdate},
				builder.Equals(
					builder.Identifier(`resource.metadata.labels["label"]`),
					builder.String("value"),
				).String(),
			), types.RoleConditions{}),
			startingRole: newRole(t, "upsert", nil, types.RoleConditions{}, types.RoleConditions{}),
			upsertRole: newRole(t, "upsert", map[string]string{
				"label": "value",
			}, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "update role new role doesn't match where",
			userRole: newRole(t, "urole", nil, rcWithRoleRuleWhere([]string{types.VerbUpdate},
				builder.Equals(
					builder.Identifier(`resource.metadata.labels["label"]`),
					builder.String("value"),
				).String(),
			), types.RoleConditions{}),
			startingRole: newRole(t, "upsert", map[string]string{
				"label": "value",
			}, types.RoleConditions{}, types.RoleConditions{}),
			upsertRole: newRole(t, "upsert", nil, rcWithRoleRule(services.RW()...), types.RoleConditions{}),
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			srv := newTestTLSServer(t)
			as := srv.AuthServer.AuthServer

			// Create a starting role if needed. This will be used to test collisions.
			var revision string
			if test.startingRole != nil {
				startingRole, err := as.CreateRole(ctx, test.startingRole)
				require.NoError(t, err)
				revision = startingRole.GetMetadata().Revision
			}

			user, err := types.NewUser("user")
			require.NoError(t, err)
			user.AddRole(test.userRole.GetName())

			// Create a test user with the given user role.
			_, err = as.CreateRole(ctx, test.userRole)
			require.NoError(t, err)
			_, err = as.CreateUser(ctx, user)
			require.NoError(t, err)
			clt, err := srv.NewClient(TestUser(user.GetName()))
			require.NoError(t, err)
			test.upsertRole.SetRevision(revision)

			got, err := clt.UpsertRole(ctx, test.upsertRole)
			test.wantErr(t, err)
			if test.want == nil {
				require.Nil(t, got)
			} else {
				require.Empty(t, cmp.Diff(test.want, got,
					cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
				))
			}
		})
	}
}

func TestGetRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		userRole     types.Role
		roleToCreate types.Role
		roleToGet    string
		wantErr      require.ErrorAssertionFunc
		want         types.Role
	}{
		{
			name:         "get role",
			userRole:     newRole(t, "urole", nil, rcWithRoleRule(types.VerbRead), types.RoleConditions{}),
			roleToCreate: newRole(t, "get", nil, types.RoleConditions{}, types.RoleConditions{}),
			roleToGet:    "get",
			wantErr:      require.NoError,
			want:         newRole(t, "get", nil, types.RoleConditions{}, types.RoleConditions{}),
		},
		{
			name:         "get role denied",
			userRole:     newRole(t, "urole", nil, types.RoleConditions{}, types.RoleConditions{}),
			roleToCreate: newRole(t, "get", nil, types.RoleConditions{}, types.RoleConditions{}),
			roleToGet:    "get",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name:      "get role does not exist",
			userRole:  newRole(t, "urole", nil, rcWithRoleRule(types.VerbRead), types.RoleConditions{}),
			roleToGet: "get",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name:         "get role does not exist with access denied",
			userRole:     newRole(t, "urole", nil, types.RoleConditions{}, types.RoleConditions{}),
			roleToCreate: newRole(t, "get", nil, types.RoleConditions{}, types.RoleConditions{}),
			roleToGet:    "get",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "get role where clause",
			userRole: newRole(t, "urole", nil, rcWithRoleRuleWhere([]string{types.VerbRead},
				builder.Equals(
					builder.Identifier(`resource.metadata.labels["label"]`),
					builder.String("value"),
				).String(),
			), types.RoleConditions{}),
			roleToCreate: newRole(t, "get", map[string]string{
				"label": "value",
			}, types.RoleConditions{}, types.RoleConditions{}),
			roleToGet: "get",
			wantErr:   require.NoError,
			want: newRole(t, "get", map[string]string{
				"label": "value",
			}, types.RoleConditions{}, types.RoleConditions{}),
		},
		{
			name: "get role where clause denied",
			userRole: newRole(t, "urole", nil, rcWithRoleRuleWhere([]string{types.VerbRead},
				builder.Equals(
					builder.Identifier(`resource.metadata.labels["label"]`),
					builder.String("value"),
				).String(),
			), types.RoleConditions{}),
			roleToCreate: newRole(t, "get", nil, types.RoleConditions{}, types.RoleConditions{}),
			roleToGet:    "get",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			srv := newTestTLSServer(t)
			as := srv.AuthServer.AuthServer

			// Only create the role if it's been supplied.
			if test.roleToCreate != nil {
				_, err := as.CreateRole(ctx, test.roleToCreate)
				require.NoError(t, err)
			}

			user, err := types.NewUser("user")
			require.NoError(t, err)
			user.AddRole(test.userRole.GetName())

			// Create a test user with the given user role.
			_, err = as.CreateRole(ctx, test.userRole)
			require.NoError(t, err)
			_, err = as.CreateUser(ctx, user)
			require.NoError(t, err)
			clt, err := srv.NewClient(TestUser(user.GetName()))
			require.NoError(t, err)

			got, err := clt.GetRole(ctx, test.roleToGet)
			test.wantErr(t, err)
			if test.want == nil {
				require.Nil(t, got)
			} else {
				require.Empty(t, cmp.Diff(test.want, got,
					cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
				))
			}
		})
	}
}

func TestGetRoles(t *testing.T) {
	t.Parallel()

	allRoles := func() []types.Role {
		return []types.Role{
			newRole(t, "1", nil, types.RoleConditions{}, types.RoleConditions{}),
			newRole(t, "2", map[string]string{
				"label": "value",
			}, types.RoleConditions{}, types.RoleConditions{}),
			newRole(t, "3", map[string]string{
				"label": "value",
			}, types.RoleConditions{}, types.RoleConditions{}),
			newRole(t, "4", nil, types.RoleConditions{}, types.RoleConditions{}),
			newRole(t, "5", nil, types.RoleConditions{}, types.RoleConditions{}),
		}
	}

	tests := []struct {
		name          string
		userRole      types.Role
		rolesToCreate []types.Role
		wantErr       require.ErrorAssertionFunc
		want          []types.Role
	}{
		{
			name:          "get roles",
			userRole:      newRole(t, "urole", nil, rcWithRoleRule(types.VerbList, types.VerbRead), types.RoleConditions{}),
			rolesToCreate: allRoles(),
			wantErr:       require.NoError,
			want:          allRoles(),
		},
		{
			name:          "get roles access denied",
			userRole:      newRole(t, "urole", nil, types.RoleConditions{}, types.RoleConditions{}),
			rolesToCreate: allRoles(),
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "get roles where",
			userRole: newRole(t, "urole", nil, rcWithRoleRuleWhere([]string{types.VerbList, types.VerbRead},
				builder.Equals(
					builder.Identifier(`resource.metadata.labels["label"]`),
					builder.String("value"),
				).String(),
			), types.RoleConditions{}),
			rolesToCreate: allRoles(),
			wantErr:       require.NoError,
			want: []types.Role{
				newRole(t, "2", map[string]string{
					"label": "value",
				}, types.RoleConditions{}, types.RoleConditions{}),
				newRole(t, "3", map[string]string{
					"label": "value",
				}, types.RoleConditions{}, types.RoleConditions{}),
			},
		},
		{
			name: "get roles where none found (access denied)",
			userRole: newRole(t, "urole", nil, rcWithRoleRuleWhere([]string{types.VerbList, types.VerbRead},
				builder.Equals(
					builder.Identifier(`resource.metadata.labels["something-else"]`),
					builder.String("non-existent"),
				).String(),
			), types.RoleConditions{}),
			rolesToCreate: allRoles(),
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			srv := newTestTLSServer(t)
			as := srv.AuthServer.AuthServer

			// Only create the role if it's been supplied.
			for _, role := range test.rolesToCreate {
				_, err := as.CreateRole(ctx, role)
				require.NoError(t, err)
			}

			user, err := types.NewUser("user")
			require.NoError(t, err)
			user.AddRole(test.userRole.GetName())

			// Create a test user with the given user role.
			_, err = as.CreateRole(ctx, test.userRole)
			require.NoError(t, err)
			_, err = as.CreateUser(ctx, user)
			require.NoError(t, err)
			clt, err := srv.NewClient(TestUser(user.GetName()))
			require.NoError(t, err)

			got, err := clt.GetRoles(ctx)
			test.wantErr(t, err)
			if test.want == nil {
				require.Nil(t, got)
			} else {
				require.Empty(t, cmp.Diff(test.want, got,
					cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
					cmpopts.SortSlices(func(r1, r2 types.Role) bool {
						return r1.GetName() < r2.GetName()
					}),
					// Ignore the user role and the default implicit role.
					cmpopts.IgnoreSliceElements(func(r types.Role) bool {
						return r.GetName() == test.userRole.GetName() ||
							r.GetName() == constants.DefaultImplicitRole
					}),
				))
			}

			// verify that ListRoles behavior is equivalent to GetRoles
			var lgot []types.Role
			var req proto.ListRolesRequest
			for {
				rsp, err := clt.ListRoles(ctx, &req)
				test.wantErr(t, err)
				if test.want == nil {
					require.Nil(t, rsp)
					break
				}

				for _, r := range rsp.Roles {
					lgot = append(lgot, r)
				}
				req.StartKey = rsp.NextKey
				if req.StartKey == "" {
					break
				}
			}

			require.Equal(t, got, lgot)
		})
	}
}

func TestListRolesFiltering(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	testRoles := func(t *testing.T) []types.Role {
		return []types.Role{
			newRole(t, "nop", nil, types.RoleConditions{}, types.RoleConditions{}),
			newRole(t, "foo", map[string]string{
				"ord": "odd",
				"grp": "low",
			}, types.RoleConditions{}, types.RoleConditions{}),
			newRole(t, "bar", map[string]string{
				"ord": "even",
				"grp": "low",
			}, types.RoleConditions{}, types.RoleConditions{}),
			newRole(t, "bin", map[string]string{
				"ord": "odd",
				"grp": "high",
			}, types.RoleConditions{}, types.RoleConditions{}),
			newRole(t, "baz", map[string]string{
				"ord": "even",
				"grp": "high",
			}, types.RoleConditions{}, types.RoleConditions{}),
		}
	}

	tts := []struct {
		name   string
		search []string
		expect []string
	}{
		{
			name:   "all",
			search: nil,
			expect: []string{
				"nop",
				"foo",
				"bar",
				"bin",
				"baz",
			},
		},
		{
			name:   "nothing",
			search: []string{"this-matches-nothing"},
			expect: nil,
		},
		{
			name:   "simple label",
			search: []string{"odd"},
			expect: []string{
				"foo",
				"bin",
			},
		},
		{
			name:   "label substring",
			search: []string{"eve"},
			expect: []string{
				"bar",
				"baz",
			},
		},
		{
			name:   "multi lable",
			search: []string{"high", "even"},
			expect: []string{
				"baz",
			},
		},
		{
			name:   "name substring",
			search: []string{"ba"},
			expect: []string{
				"bar",
				"baz",
			},
		},
	}

	for _, tt := range tts {
		search, expect := tt.search, tt.expect
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := newTestTLSServer(t)

			clt, err := srv.NewClient(TestAdmin())
			require.NoError(t, err)

			// Only create the role if it's been supplied.
			for _, role := range testRoles(t) {
				_, err := clt.CreateRole(ctx, role)
				require.NoError(t, err)
			}

			req := proto.ListRolesRequest{
				Filter: &types.RoleFilter{
					SearchKeywords: search,
				},
			}
			var gotRoles []string
			for {
				rsp, err := clt.ListRoles(ctx, &req)
				require.NoError(t, err)

				for _, role := range rsp.Roles {
					if role.GetName() == constants.DefaultImplicitRole {
						continue
					}
					gotRoles = append(gotRoles, role.GetName())
				}

				req.StartKey = rsp.NextKey
				if req.StartKey == "" {
					break
				}
			}

			slices.Sort(expect)
			slices.Sort(gotRoles)
			require.Equal(t, expect, gotRoles)
		})
	}
}

func TestDeleteRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		userRole     types.Role
		roleToCreate types.Role
		roleToDelete string
		wantErr      require.ErrorAssertionFunc
	}{
		{
			name:         "delete role",
			userRole:     newRole(t, "urole", nil, rcWithRoleRule(types.VerbDelete), types.RoleConditions{}),
			roleToCreate: newRole(t, "delete", nil, types.RoleConditions{}, types.RoleConditions{}),
			roleToDelete: "delete",
			wantErr:      require.NoError,
		},
		{
			name:         "delete role denied",
			userRole:     newRole(t, "urole", nil, types.RoleConditions{}, types.RoleConditions{}),
			roleToCreate: newRole(t, "delete", nil, types.RoleConditions{}, types.RoleConditions{}),
			roleToDelete: "delete",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name:         "delete role does not exist",
			userRole:     newRole(t, "urole", nil, rcWithRoleRule(types.VerbDelete), types.RoleConditions{}),
			roleToDelete: "delete",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name:         "delete role does not exist with access denied",
			userRole:     newRole(t, "urole", nil, types.RoleConditions{}, types.RoleConditions{}),
			roleToCreate: newRole(t, "delete", nil, types.RoleConditions{}, types.RoleConditions{}),
			roleToDelete: "delete",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "delete role where clause",
			userRole: newRole(t, "urole", nil, rcWithRoleRuleWhere([]string{types.VerbDelete},
				builder.Equals(
					builder.Identifier(`resource.metadata.labels["label"]`),
					builder.String("value"),
				).String(),
			), types.RoleConditions{}),
			roleToCreate: newRole(t, "delete", map[string]string{
				"label": "value",
			}, types.RoleConditions{}, types.RoleConditions{}),
			roleToDelete: "delete",
			wantErr:      require.NoError,
		},
		{
			name: "delete role where clause denied",
			userRole: newRole(t, "urole", nil, rcWithRoleRuleWhere([]string{types.VerbDelete},
				builder.Equals(
					builder.Identifier(`resource.metadata.labels["label"]`),
					builder.String("value"),
				).String(),
			), types.RoleConditions{}),
			roleToCreate: newRole(t, "delete", nil, types.RoleConditions{}, types.RoleConditions{}),
			roleToDelete: "delete",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			srv := newTestTLSServer(t)
			as := srv.AuthServer.AuthServer

			// Only create the role if it's been supplied.
			if test.roleToCreate != nil {
				_, err := as.CreateRole(ctx, test.roleToCreate)
				require.NoError(t, err)
			}

			user, err := types.NewUser("user")
			require.NoError(t, err)
			user.AddRole(test.userRole.GetName())

			// Create a test user with the given user role.
			_, err = as.CreateRole(ctx, test.userRole)
			require.NoError(t, err)
			_, err = as.CreateUser(ctx, user)
			require.NoError(t, err)
			clt, err := srv.NewClient(TestUser(user.GetName()))
			require.NoError(t, err)

			test.wantErr(t, clt.DeleteRole(ctx, test.roleToDelete))
		})
	}
}

func TestUpsertDeleteLockEventsEmitted(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	lock, err := types.NewLock("test-lock", types.LockSpecV2{
		Target: types.LockTarget{MFADevice: "mfa-device-id"},
	})
	require.NoError(t, err)
	futureTime := time.Now().UTC().Add(12 * time.Hour)
	lock.SetLockExpiry(&futureTime)

	// Creating a lock should emit a LockCreatedEvent.
	err = p.a.UpsertLock(ctx, lock)
	require.NoError(t, err)
	require.Equal(t, events.LockCreatedEvent, p.mockEmitter.LastEvent().GetType())
	require.Equal(t, lock.GetName(), p.mockEmitter.LastEvent().(*apievents.LockCreate).Name)
	require.Equal(t, lock.Target(), p.mockEmitter.LastEvent().(*apievents.LockCreate).Target)
	require.Equal(t, lock.LockExpiry().UTC(), p.mockEmitter.LastEvent().(*apievents.LockCreate).Expires)
	p.mockEmitter.Reset()

	// When a lock update results in an error, no event should be emitted.
	lock.SetTarget(types.LockTarget{})
	err = p.a.UpsertLock(ctx, lock)
	require.Error(t, err)
	require.Nil(t, p.mockEmitter.LastEvent())

	// Updating a lock should emit a LockCreatedEvent.
	lock.SetTarget(types.LockTarget{Role: "test-role"})
	err = p.a.UpsertLock(ctx, lock)
	require.NoError(t, err)
	require.Equal(t, events.LockCreatedEvent, p.mockEmitter.LastEvent().GetType())
	require.Equal(t, lock.GetName(), p.mockEmitter.LastEvent().(*apievents.LockCreate).Name)
	require.Equal(t, lock.Target(), p.mockEmitter.LastEvent().(*apievents.LockCreate).Target)
	p.mockEmitter.Reset()

	// Deleting a lock should emit a LockDeletedEvent.
	err = p.a.DeleteLock(ctx, lock.GetName())
	require.NoError(t, err)
	require.Equal(t, events.LockDeletedEvent, p.mockEmitter.LastEvent().GetType())
	require.Equal(t, lock.GetName(), p.mockEmitter.LastEvent().(*apievents.LockDelete).Name)
	p.mockEmitter.Reset()

	// When deleting a nonexistent lock, no event should be emitted.
	err = p.a.DeleteLock(ctx, lock.GetName())
	require.True(t, trace.IsNotFound(err))
	require.Nil(t, p.mockEmitter.LastEvent())
}
