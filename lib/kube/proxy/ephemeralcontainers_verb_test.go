/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package proxy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubetypes "k8s.io/apimachinery/pkg/types"

	"github.com/gravitational/teleport/api/types"
	tkm "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
)

// TestEphemeralContainersRequiresExecAndPatch verifies that a PATCH to
// pods/{name}/ephemeralcontainers (what `kubectl debug` issues) is admitted
// only when the user is granted both the exec verb and the patch/update
// mutation verb on pods. The two verbs are checked independently, so they may
// be granted by different roles: Teleport RBAC is additive and permissions
// union across roles. Neither verb on its own is enough.
func TestEphemeralContainersRequiresExecAndPatch(t *testing.T) {
	t.Parallel()

	kubeMock, err := tkm.NewKubeAPIMock()
	require.NoError(t, err)
	t.Cleanup(func() { kubeMock.Close() })

	testCtx := SetupTestContext(
		context.Background(),
		t,
		TestConfig{
			Clusters: []KubeClusterConfig{{Name: kubeCluster, APIEndpoint: kubeMock.URL}},
		},
	)
	t.Cleanup(func() { require.NoError(t, testCtx.Close()) })

	// podVerbs returns a SetupRoleFunc granting the given verbs on all pods.
	podVerbs := func(verbs ...string) func(types.Role) {
		return func(r types.Role) {
			r.SetKubeResources(types.Allow, []types.KubernetesResource{{
				Kind:      "pods",
				Name:      types.Wildcard,
				Namespace: types.Wildcard,
				Verbs:     verbs,
				APIGroup:  types.Wildcard,
			}})
		}
	}

	patchOnlyUser, _ := testCtx.CreateUserAndRole(testCtx.Context, t, "eph_patch_only", RoleSpec{
		Name:          "eph_patch_only",
		KubeUsers:     roleKubeUsers,
		KubeGroups:    roleKubeGroups,
		SetupRoleFunc: podVerbs("get", "list", "watch", "patch", "update"),
	})
	execOnlyUser, _ := testCtx.CreateUserAndRole(testCtx.Context, t, "eph_exec_only", RoleSpec{
		Name:          "eph_exec_only",
		KubeUsers:     roleKubeUsers,
		KubeGroups:    roleKubeGroups,
		SetupRoleFunc: podVerbs("get", "list", "watch", "exec"),
	})
	execPatchUser, _ := testCtx.CreateUserAndRole(testCtx.Context, t, "eph_exec_patch", RoleSpec{
		Name:          "eph_exec_patch",
		KubeUsers:     roleKubeUsers,
		KubeGroups:    roleKubeGroups,
		SetupRoleFunc: podVerbs("get", "list", "watch", "exec", "patch", "update"),
	})

	// splitUser has exec from one role and patch from another. Under additive
	// RBAC the user effectively has both, so the request must be admitted.
	splitUser := createUserWithRoles(t, testCtx, "eph_split",
		podVerbs("get", "list", "watch", "exec"),
		podVerbs("get", "list", "watch", "patch", "update"),
	)

	tests := []struct {
		name    string
		user    types.User
		admit   bool
		message string
	}{
		{name: "patch only is denied", user: patchOnlyUser, admit: false},
		{name: "exec only is denied", user: execOnlyUser, admit: false},
		{name: "exec and patch in one role is admitted", user: execPatchUser, admit: true},
		{name: "exec and patch split across roles is admitted", user: splitUser, admit: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client, _ := testCtx.GenTestKubeClientTLSCert(t, tt.user.GetName(), kubeCluster)

			// PATCH /api/v1/namespaces/default/pods/test/ephemeralcontainers.
			err := client.CoreV1().RESTClient().
				Patch(kubetypes.StrategicMergePatchType).
				Namespace(metav1.NamespaceDefault).
				Resource("pods").
				Name("test").
				SubResource("ephemeralcontainers").
				Body([]byte(`{"spec":{"ephemeralContainers":[{"name":"debugger","image":"busybox"}]}}`)).
				Do(testCtx.Context).
				Error()

			if tt.admit {
				// Teleport forwards the request. The mock has no
				// ephemeralcontainers route, so the upstream answers
				// not-found rather than Teleport answering forbidden.
				require.False(t, kubeerrors.IsForbidden(err),
					"request should be admitted by Teleport, got %v", err)
			} else {
				require.True(t, kubeerrors.IsForbidden(err),
					"request should be denied by Teleport, got %v", err)
			}
		})
	}
}

// createUserWithRoles creates a user assigned multiple Teleport roles, each
// with the given pod verbs, so a single user can draw different verbs from
// different roles. Returns the created user.
func createUserWithRoles(t *testing.T, testCtx *TestContext, prefix string, setups ...func(types.Role)) types.User {
	t.Helper()
	auth := testCtx.TLSServer.Auth()

	var roleNames []string
	for i, setup := range setups {
		roleName := prefix + "_role_" + string(rune('a'+i))
		role, err := types.NewRole(roleName, types.RoleSpecV6{
			Allow: types.RoleConditions{
				KubernetesLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				KubeGroups:       roleKubeGroups,
				KubeUsers:        roleKubeUsers,
			},
		})
		require.NoError(t, err)
		setup(role)
		_, err = auth.UpsertRole(testCtx.Context, role)
		require.NoError(t, err)
		roleNames = append(roleNames, roleName)
	}

	user, err := types.NewUser(prefix + "_user")
	require.NoError(t, err)
	user.SetRoles(roleNames)
	user, err = auth.UpsertUser(testCtx.Context, user)
	require.NoError(t, err)
	return user
}
