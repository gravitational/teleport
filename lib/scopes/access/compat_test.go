// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package access

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
)

// TestEmptyRoleConverts verifies that a scoped role that is empty except for
// scoping/metadata converts to a reasonable default/unprivileged classic role.
func TestEmptyRoleConverts(t *testing.T) {
	t.Parallel()

	role, err := ScopedRoleToRole(scopedaccessv1.ScopedRole_builder{
		Kind: KindScopedRole,
		Metadata: headerv1.Metadata_builder{
			Name: "test",
		}.Build(),
		Scope: "/foo",
		Spec: scopedaccessv1.ScopedRoleSpec_builder{
			AssignableScopes: []string{"/foo/bar"},
		}.Build(),
		Version: types.V1,
	}.Build(), "/foo/bar")
	require.NoError(t, err)
	require.NotNil(t, role)
	require.Equal(t, "test@/foo/bar", role.GetName())
}

// TestSSHConversion verifies the various SSH-related scoped role conversion scenarios.
func TestSSHConversion(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name   string
		ssh    *scopedaccessv1.ScopedRoleSSH
		expect types.RoleConditions
	}{
		{
			name:   "nil ssh block",
			ssh:    nil,
			expect: types.RoleConditions{},
		},
		{
			name:   "empty ssh block",
			ssh:    &scopedaccessv1.ScopedRoleSSH{},
			expect: types.RoleConditions{},
		},
		{
			name: "sparse",
			ssh: scopedaccessv1.ScopedRoleSSH_builder{
				Logins: []string{"root"},
				Labels: []*labelv1.Label{
					labelv1.Label_builder{
						Name:   "team",
						Values: []string{"red"},
					}.Build(),
				},
			}.Build(),
			expect: types.RoleConditions{
				Logins: []string{"root"},
				NodeLabels: types.Labels{
					"team": apiutils.Strings{"red"},
				},
			},
		},
		{
			name: "full",
			ssh: scopedaccessv1.ScopedRoleSSH_builder{
				Logins: []string{"root", "admin"},
				Labels: []*labelv1.Label{
					labelv1.Label_builder{
						Name:   "env",
						Values: []string{"prod", "staging"},
					}.Build(),
					labelv1.Label_builder{
						Name:   "team",
						Values: []string{"blue"},
					}.Build(),
				},
			}.Build(),
			expect: types.RoleConditions{
				Logins: []string{"root", "admin"},
				NodeLabels: types.Labels{
					"env":  apiutils.Strings{"prod", "staging"},
					"team": apiutils.Strings{"blue"},
				},
			},
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			role, err := ScopedRoleToRole(scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/foo",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo/bar"},
					Ssh:              tt.ssh,
				}.Build(),
				Version: types.V1,
			}.Build(), "/foo/bar")
			require.NoError(t, err)
			tt.expect.Namespaces = []string{"default"}
			require.Empty(t, cmp.Diff(tt.expect, role.GetRoleConditions(types.Allow)))
		})
	}
}

// TestRulesConversion verifies various scoped role rule conversion scenarios.
func TestRulesConversion(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name   string
		rules  []*scopedaccessv1.ScopedRule
		expect []types.Rule
	}{
		{
			name: "basic",
			rules: []*scopedaccessv1.ScopedRule{
				scopedaccessv1.ScopedRule_builder{
					Resources: []string{KindScopedRole},
					Verbs:     []string{types.VerbList, types.VerbReadNoSecrets},
				}.Build(),
				scopedaccessv1.ScopedRule_builder{
					Resources: []string{KindScopedRoleAssignment},
					Verbs:     []string{types.VerbList, types.VerbReadNoSecrets, types.VerbCreate, types.VerbUpdate, types.VerbDelete},
				}.Build(),
			},
			expect: []types.Rule{
				{
					Resources: []string{KindScopedRole},
					Verbs:     []string{types.VerbList, types.VerbReadNoSecrets},
				},
				{
					Resources: []string{KindScopedRoleAssignment},
					Verbs:     []string{types.VerbList, types.VerbReadNoSecrets, types.VerbCreate, types.VerbUpdate, types.VerbDelete},
				},
			},
		},
		{
			name: "unsupported verb",
			rules: []*scopedaccessv1.ScopedRule{
				scopedaccessv1.ScopedRule_builder{
					Resources: []string{KindScopedRole},
					Verbs:     []string{types.VerbList, types.VerbRead},
				}.Build(),
			},
			expect: []types.Rule{
				{
					Resources: []string{KindScopedRole},
					Verbs:     []string{types.VerbList},
				},
			},
		},
		{
			name: "unsupported resource",
			rules: []*scopedaccessv1.ScopedRule{
				scopedaccessv1.ScopedRule_builder{
					Resources: []string{types.KindCertAuthority},
					Verbs:     []string{types.VerbList, types.VerbReadNoSecrets},
				}.Build(),
				scopedaccessv1.ScopedRule_builder{
					Resources: []string{KindScopedRole},
					Verbs:     []string{types.VerbList, types.VerbReadNoSecrets},
				}.Build(),
			},
			expect: []types.Rule{
				{
					Resources: []string{KindScopedRole},
					Verbs:     []string{types.VerbList, types.VerbReadNoSecrets},
				},
			},
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			role, err := ScopedRoleToRole(scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/foo",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo/bar"},
					Rules:            tt.rules,
				}.Build(),
				Version: types.V1,
			}.Build(), "/foo/bar")
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(tt.expect, role.GetRules(types.Allow)))
		})
	}
}

func baseScopedRole() *scopedaccessv1.ScopedRole {
	return scopedaccessv1.ScopedRole_builder{
		Kind:     KindScopedRole,
		Metadata: headerv1.Metadata_builder{Name: "test"}.Build(),
		Scope:    "/foo",
		Spec: scopedaccessv1.ScopedRoleSpec_builder{
			AssignableScopes: []string{"/foo/bar"},
			Ssh:              &scopedaccessv1.ScopedRoleSSH{},
			Kube:             &scopedaccessv1.ScopedRoleKube{},
		}.Build(),
		Version: types.V1,
	}.Build()
}

func ptr[T any](v T) *T { return &v }

// TestClientIdleTimeoutNotInClassicRole verifies that ScopedRoleToRole does not populate ClientIdleTimeout
// in the classic role options. Per the scoped role design, client_idle_timeout is read directly from the
// appropriate protocol block on the scoped role, which does not have a direct classic role equivalent.
func TestClientIdleTimeoutNotInClassicRole(t *testing.T) {
	t.Parallel()

	sr := baseScopedRole()
	sr.GetSpec().GetSsh().SetClientIdleTimeout("30m")

	role, err := ScopedRoleToRole(sr, "/foo/bar")
	require.NoError(t, err)
	require.NotNil(t, role)
	// ClientIdleTimeout must be zero in the converted role; it is evaluated at access-check time
	// via SSHAccessChecker.AdjustClientIdleTimeout reading directly from the scoped role proto.
	require.Zero(t, role.GetOptions().ClientIdleTimeout.Duration())
}

func TestX11ForwardingNotInClassicRole(t *testing.T) {
	t.Parallel()

	sr := baseScopedRole()
	sr.GetSpec().GetSsh().PermitX11Forwarding = ptr(true)

	role, err := ScopedRoleToRole(sr, "/foo/bar")
	require.NoError(t, err)
	require.Equal(t, types.NewBool(false), role.GetOptions().PermitX11Forwarding)
}

func TestForwardAgentNotInClassicRole(t *testing.T) {
	t.Parallel()

	sr := baseScopedRole()
	sr.GetSpec().GetSsh().ForwardAgent = ptr(true)

	role, err := ScopedRoleToRole(sr, "/foo/bar")
	require.NoError(t, err)
	require.Equal(t, types.NewBool(UnstableGetScopedForwardAgent()), role.GetOptions().ForwardAgent)
}

func TestMaxSessionsNotInClassicRole(t *testing.T) {
	t.Parallel()

	sr := baseScopedRole()
	sr.GetSpec().GetSsh().MaxSessions = ptr(int64(10))

	role, err := ScopedRoleToRole(sr, "/foo/bar")
	require.NoError(t, err)
	require.Equal(t, int64(0), role.GetOptions().MaxSessions)
}

func TestSSHPortForwardingNotInClassicRole(t *testing.T) {
	t.Parallel()

	sr := baseScopedRole()
	sr.GetSpec().GetSsh().SetPortForwarding(scopedaccessv1.SSHPortForwarding_builder{
		Local:  scopedaccessv1.SSHLocalPortForwarding_builder{Enabled: ptr(false)}.Build(),
		Remote: scopedaccessv1.SSHRemotePortForwarding_builder{Enabled: ptr(false)}.Build(),
	}.Build())

	role, err := ScopedRoleToRole(sr, "/foo/bar")
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(getScopedPortForwardingConfig(), role.GetOptions().SSHPortForwarding))
}

func TestHostUserCreationNotInClassicRole(t *testing.T) {
	t.Parallel()

	sr := baseScopedRole()
	sr.GetSpec().GetSsh().SetHostUserCreation(scopedaccessv1.CreateHostUser_builder{
		Mode: "keep",
	}.Build())

	role, err := ScopedRoleToRole(sr, "/foo/bar")
	require.NoError(t, err)
	require.Equal(t, types.CreateHostUserMode_HOST_USER_MODE_UNSPECIFIED, role.GetOptions().CreateHostUserMode)
}

func TestSSHFileCopyNotInClassicRole(t *testing.T) {
	t.Parallel()

	sr := baseScopedRole()
	sr.GetSpec().GetSsh().FileCopy = ptr(false)

	role, err := ScopedRoleToRole(sr, "/foo/bar")
	require.NoError(t, err)
	require.Equal(t, types.NewBoolOption(true), role.GetOptions().SSHFileCopy)
}

func TestEnhancedRecordingNotInClassicRole(t *testing.T) {
	t.Parallel()

	sr := baseScopedRole()
	sr.GetSpec().GetSsh().SetEnhancedRecording(scopedaccessv1.EnhancedRecording_builder{
		Command: ptr(true),
		Network: ptr(true),
		Disk:    ptr(true),
	}.Build())
	role, err := ScopedRoleToRole(sr, "/foo/bar")
	require.NoError(t, err)
	// BPF is the classic role equivalent of enhanced_recording.
	require.Equal(t, apidefaults.EnhancedEvents(), role.GetOptions().BPF)
}

func TestDisconnectExpiredCertNotInClassicRole(t *testing.T) {
	t.Parallel()

	sr := baseScopedRole()
	sr.GetSpec().GetSsh().DisconnectExpiredCert = ptr(true)

	role, err := ScopedRoleToRole(sr, "/foo/bar")
	require.NoError(t, err)
	require.Equal(t, types.NewBool(false), role.GetOptions().DisconnectExpiredCert)
}

func TestKubeDisconnectExpiredCertNotInClassicRole(t *testing.T) {
	t.Parallel()

	sr := baseScopedRole()
	sr.GetSpec().SetKube(scopedaccessv1.ScopedRoleKube_builder{
		DisconnectExpiredCert: ptr(true),
	}.Build())

	role, err := ScopedRoleToRole(sr, "/foo/bar")
	require.NoError(t, err)
	require.Equal(t, types.NewBool(false), role.GetOptions().DisconnectExpiredCert)
}

func TestSessionRecordingNotInClassicRole(t *testing.T) {
	t.Parallel()

	sr := baseScopedRole()
	sr.GetSpec().GetSsh().SetSessionRecording(scopedaccessv1.SessionRecording_builder{
		Mode: string(constants.SessionRecordingModeBestEffort),
	}.Build())

	role, err := ScopedRoleToRole(sr, "/foo/bar")
	require.NoError(t, err)

	// RecordSession is the classic role equivalent of session_recording_mode.
	require.Equal(t, &types.RecordSession{
		Desktop: types.NewBoolOption(true),
		Default: constants.SessionRecordingModeBestEffort,
	}, role.GetOptions().RecordSession)
}

func TestSSHLockingModeNotInClassicRole(t *testing.T) {
	t.Parallel()

	sr := baseScopedRole()
	sr.GetSpec().GetSsh().SetLock(scopedaccessv1.Lock_builder{
		Mode: "strict",
	}.Build())

	role, err := ScopedRoleToRole(sr, "/foo/bar")
	require.NoError(t, err)

	require.Empty(t, string(role.GetOptions().Lock))
}

func TestKubeLockingModeNotInClassicRole(t *testing.T) {
	t.Parallel()

	sr := baseScopedRole()
	sr.GetSpec().GetKube().SetLock(scopedaccessv1.Lock_builder{
		Mode: "strict",
	}.Build())

	role, err := ScopedRoleToRole(sr, "/foo/bar")
	require.NoError(t, err)
	require.Empty(t, string(role.GetOptions().Lock))
}

// TestKubeConversion verifies the various kube-related scoped role conversion scenarios.
func TestKubeConversion(t *testing.T) {
	t.Parallel()

	wildcardResources := []types.KubernetesResource{
		{
			Kind:      types.Wildcard,
			Namespace: types.Wildcard,
			Name:      types.Wildcard,
			APIGroup:  types.Wildcard,
			Verbs:     []string{types.Wildcard},
		},
	}
	tts := []struct {
		name   string
		kube   *scopedaccessv1.ScopedRoleKube
		expect types.RoleConditions
	}{
		{
			name:   "empty conditions",
			kube:   &scopedaccessv1.ScopedRoleKube{},
			expect: types.RoleConditions{},
		},
		{
			name: "sparse",
			kube: scopedaccessv1.ScopedRoleKube_builder{
				Users:  []string{"system:user"},
				Groups: []string{"viewer"},
				Labels: []*labelv1.Label{
					labelv1.Label_builder{
						Name:   "team",
						Values: []string{"red"},
					}.Build(),
				},
			}.Build(),
			expect: types.RoleConditions{
				KubeUsers:  []string{"system:user"},
				KubeGroups: []string{"viewer"},
				KubernetesLabels: types.Labels{
					"team": apiutils.Strings{"red"},
				},
				KubernetesResources: wildcardResources,
			},
		},
		{
			name: "full",
			kube: scopedaccessv1.ScopedRoleKube_builder{
				Users:  []string{"system:user", "system:admin"},
				Groups: []string{"viewer", "editor"},
				Labels: []*labelv1.Label{
					labelv1.Label_builder{
						Name:   "env",
						Values: []string{"prod", "staging"},
					}.Build(),
					labelv1.Label_builder{
						Name:   "team",
						Values: []string{"blue"},
					}.Build(),
				},
			}.Build(),
			expect: types.RoleConditions{
				KubeUsers:  []string{"system:user", "system:admin"},
				KubeGroups: []string{"viewer", "editor"},
				KubernetesLabels: types.Labels{
					"env":  apiutils.Strings{"prod", "staging"},
					"team": apiutils.Strings{"blue"},
				},
				KubernetesResources: wildcardResources,
			},
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			role, err := ScopedRoleToRole(scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/foo",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					Kube:             tt.kube,
					AssignableScopes: []string{"/foo/bar"},
				}.Build(),
				Version: types.V1,
			}.Build(), "/foo/bar")
			require.NoError(t, err)
			tt.expect.Namespaces = []string{"default"}
			require.Empty(t, cmp.Diff(tt.expect, role.GetRoleConditions(types.Allow)))
		})
	}
}
