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

package services

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
)

func TestNewUserACL(t *testing.T) {
	t.Parallel()

	user := &types.UserV2{}

	// set some rules
	role1 := &types.RoleV6{}
	role1.SetNamespaces(types.Allow, []string{apidefaults.Namespace})
	role1.SetRules(types.Allow, []types.Rule{
		{
			Resources: []string{types.KindAuthConnector},
			Verbs:     RW(),
		},
		{
			Resources: []string{types.KindWindowsDesktop},
			Verbs:     RW(),
		},
		{
			Resources: []string{types.KindIntegration},
			Verbs:     append(RW(), types.VerbUse),
		},
	})

	// not setting the rule, or explicitly denying, both denies Access
	role1.SetRules(types.Deny, []types.Rule{
		{
			Resources: []string{types.KindEvent},
			Verbs:     RW(),
		},
	})

	role2 := &types.RoleV6{}
	role2.SetNamespaces(types.Allow, []string{apidefaults.Namespace})
	role2.SetRules(types.Allow, []types.Rule{
		{
			Resources: []string{types.KindTrustedCluster},
			Verbs:     RW(),
		},
		{
			Resources: []string{types.KindBilling},
			Verbs:     RO(),
		},
	})

	roleSet := []types.Role{role1, role2}
	userContext := NewUserACL(user, roleSet, proto.Features{}, true, false)

	allowedRW := ResourceAccess{true, true, true, true, true, false}
	denied := ResourceAccess{false, false, false, false, false, false}

	require.Empty(t, cmp.Diff(userContext.AuthConnectors, allowedRW))
	require.Empty(t, cmp.Diff(userContext.TrustedClusters, allowedRW))
	require.Empty(t, cmp.Diff(userContext.AppServers, denied))
	require.Empty(t, cmp.Diff(userContext.Locks, denied))
	require.Empty(t, cmp.Diff(userContext.DBServers, denied))
	require.Empty(t, cmp.Diff(userContext.KubeServers, denied))
	require.Empty(t, cmp.Diff(userContext.Events, denied))
	require.Empty(t, cmp.Diff(userContext.RecordedSessions, denied))
	require.Empty(t, cmp.Diff(userContext.Roles, denied))
	require.Empty(t, cmp.Diff(userContext.Users, denied))
	require.Empty(t, cmp.Diff(userContext.Tokens, denied))
	require.Empty(t, cmp.Diff(userContext.Nodes, denied))
	require.Empty(t, cmp.Diff(userContext.AccessRequests, denied))
	require.Empty(t, cmp.Diff(userContext.ConnectionDiagnostic, denied))
	require.Empty(t, cmp.Diff(userContext.Desktops, allowedRW))
	require.Empty(t, cmp.Diff(userContext.ExternalAuditStorage, denied))
	require.Empty(t, cmp.Diff(userContext.Bots, denied))

	require.Empty(t, cmp.Diff(userContext.Billing, denied))
	require.True(t, userContext.Clipboard)
	require.True(t, userContext.DesktopSessionRecording)
	require.Empty(t, cmp.Diff(userContext.License, denied))
	require.Empty(t, cmp.Diff(userContext.Download, denied))

	// test enabling of the 'Use' verb
	require.Empty(t, cmp.Diff(userContext.Integrations, ResourceAccess{true, true, true, true, true, true}))

	userContext = NewUserACL(user, roleSet, proto.Features{Cloud: true}, true, false)
	require.Empty(t, cmp.Diff(userContext.Billing, ResourceAccess{true, true, false, false, false, false}))

	// test that desktopRecordingEnabled being false overrides the roleSet.RecordDesktopSession() returning true
	userContext = NewUserACL(user, roleSet, proto.Features{}, false, false)
	require.False(t, userContext.DesktopSessionRecording)
}

func TestNewUserACLCloud(t *testing.T) {
	t.Parallel()

	user := &types.UserV2{
		Metadata: types.Metadata{},
	}

	role := &types.RoleV6{}
	role.SetNamespaces(types.Allow, []string{"*"})
	role.SetRules(types.Allow, []types.Rule{
		{
			Resources: []string{"*"},
			Verbs:     RW(),
		},
	})

	role.SetWindowsLogins(types.Allow, []string{"a", "b"})
	role.SetWindowsLogins(types.Deny, []string{"c"})

	roleSet := []types.Role{role}

	allowedRW := ResourceAccess{true, true, true, true, true, false}

	userContext := NewUserACL(user, roleSet, proto.Features{Cloud: true}, true, false)

	require.Empty(t, cmp.Diff(userContext.AuthConnectors, allowedRW))
	require.Empty(t, cmp.Diff(userContext.TrustedClusters, allowedRW))
	require.Empty(t, cmp.Diff(userContext.AppServers, allowedRW))
	require.Empty(t, cmp.Diff(userContext.DBServers, allowedRW))
	require.Empty(t, cmp.Diff(userContext.KubeServers, allowedRW))
	require.Empty(t, cmp.Diff(userContext.Events, allowedRW))
	require.Empty(t, cmp.Diff(userContext.RecordedSessions, allowedRW))
	require.Empty(t, cmp.Diff(userContext.Roles, allowedRW))
	require.Empty(t, cmp.Diff(userContext.Users, allowedRW))
	require.Empty(t, cmp.Diff(userContext.Tokens, allowedRW))
	require.Empty(t, cmp.Diff(userContext.Nodes, allowedRW))
	require.Empty(t, cmp.Diff(userContext.AccessRequests, allowedRW))
	require.Empty(t, cmp.Diff(userContext.DiscoveryConfig, allowedRW))
	require.Empty(t, cmp.Diff(userContext.ExternalAuditStorage, allowedRW))
	require.Empty(t, cmp.Diff(userContext.Bots, allowedRW))
	require.True(t, userContext.Clipboard)
	require.True(t, userContext.DesktopSessionRecording)

	// cloud-specific asserts
	require.Empty(t, cmp.Diff(userContext.Billing, allowedRW))
	require.Empty(t, cmp.Diff(userContext.Desktops, allowedRW))
}

func TestJoinSessionsACL(t *testing.T) {
	t.Parallel()

	user := &types.UserV2{
		Metadata: types.Metadata{},
	}
	// create a role denying list/read to all resources,
	// but allowing the ability to join sessions
	role := &types.RoleV6{}
	role.SetRules(types.Deny, []types.Rule{
		{
			Resources: []string{"*"},
			Verbs:     RO(),
		},
	})
	role.SetSessionJoinPolicies([]*types.SessionJoinPolicy{
		{
			Name:  "join all",
			Roles: []string{"*"},
			Modes: []string{string(types.SessionObserverMode)},
			Kinds: []string{string(types.SSHSessionKind), string(types.KubernetesSessionKind)},
		},
	})
	roleSet := []types.Role{role}
	acl := NewUserACL(user, roleSet, proto.Features{}, true, false)
	assert.True(t, acl.ActiveSessions.List)
	assert.True(t, acl.ActiveSessions.Read)
}

func TestNewAccessMonitoring(t *testing.T) {
	t.Parallel()
	user := &types.UserV2{
		Metadata: types.Metadata{},
	}
	role := &types.RoleV6{}
	role.SetNamespaces(types.Allow, []string{"*"})
	role.SetRules(types.Allow, []types.Rule{
		{
			Resources: []string{"*"},
			Verbs:     append(RW(), types.VerbUse),
		},
	})

	roleSet := []types.Role{role}

	t.Run("access monitoring enabled", func(t *testing.T) {
		allowed := ResourceAccess{true, true, true, true, true, true}
		userContext := NewUserACL(user, roleSet, proto.Features{}, false, true)
		require.Empty(t, cmp.Diff(userContext.AuditQuery, allowed))
		require.Empty(t, cmp.Diff(userContext.SecurityReport, allowed))
	})
	t.Run("access monitoring disabled", func(t *testing.T) {
		allowed := ResourceAccess{false, false, false, false, false, false}
		userContext := NewUserACL(user, roleSet, proto.Features{}, false, false)
		require.Empty(t, cmp.Diff(userContext.AuditQuery, allowed))
		require.Empty(t, cmp.Diff(userContext.SecurityReport, allowed))
	})
}

func TestNewAccessGraph(t *testing.T) {
	t.Parallel()
	user := &types.UserV2{
		Metadata: types.Metadata{},
	}
	role := &types.RoleV6{}
	role.SetNamespaces(types.Allow, []string{"*"})
	role.SetRules(types.Allow, []types.Rule{
		{
			Resources: []string{"*"},
			Verbs:     append(RW(), types.VerbUse),
		},
	})

	roleSet := []types.Role{role}

	t.Run("access graph enabled", func(t *testing.T) {
		allowed := ResourceAccess{true, true, true, true, true, true}
		userContext := NewUserACL(user, roleSet, proto.Features{AccessGraph: true}, false, true)
		require.Empty(t, cmp.Diff(userContext.AccessGraph, allowed))
	})
	t.Run("access graph disabled", func(t *testing.T) {
		allowed := ResourceAccess{false, false, false, false, false, false}
		userContext := NewUserACL(user, roleSet, proto.Features{}, false, false)
		require.Empty(t, cmp.Diff(userContext.AccessGraph, allowed))
	})

	user1 := &types.UserV2{
		Metadata: types.Metadata{},
	}
	t.Run("access graph ACL is false when user doesn't have access even when enabled", func(t *testing.T) {
		allowed := ResourceAccess{true, true, true, true, true, true}
		userContext := NewUserACL(user1, roleSet, proto.Features{AccessGraph: true}, false, true)
		require.Empty(t, cmp.Diff(userContext.AccessGraph, allowed))
	})
}
