/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package services

import (
	"testing"

	"github.com/google/go-cmp/cmp"
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
	userContext := NewUserACL(user, roleSet, proto.Features{}, true)

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

	require.Empty(t, cmp.Diff(userContext.Billing, denied))
	require.Equal(t, userContext.Clipboard, true)
	require.Equal(t, userContext.DesktopSessionRecording, true)
	require.Empty(t, cmp.Diff(userContext.License, denied))
	require.Empty(t, cmp.Diff(userContext.Download, denied))

	// test enabling of the 'Use' verb
	require.Empty(t, cmp.Diff(userContext.Integrations, ResourceAccess{true, true, true, true, true, true}))

	userContext = NewUserACL(user, roleSet, proto.Features{Cloud: true}, true)
	require.Empty(t, cmp.Diff(userContext.Billing, ResourceAccess{true, true, false, false, false, false}))

	// test that desktopRecordingEnabled being false overrides the roleSet.RecordDesktopSession() returning true
	userContext = NewUserACL(user, roleSet, proto.Features{}, false)
	require.Equal(t, userContext.DesktopSessionRecording, false)
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

	userContext := NewUserACL(user, roleSet, proto.Features{Cloud: true}, true)

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

	require.Equal(t, userContext.Clipboard, true)
	require.Equal(t, userContext.DesktopSessionRecording, true)

	// cloud-specific asserts
	require.Empty(t, cmp.Diff(userContext.Billing, allowedRW))
	require.Empty(t, cmp.Diff(userContext.Desktops, allowedRW))
}
