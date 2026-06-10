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
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/gravitational/teleport/api/constants"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/testutils"
)

// TestValidateRole verifies basic functionality of strong and weak role validation functions.
func TestValidateRole(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name     string
		role     *scopedaccessv1.ScopedRole
		strongOk bool
		weakOk   bool
	}{
		{
			name: "basic",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: true,
			weakOk:   true,
		},
		{
			name: "unknown sub_kind",
			role: scopedaccessv1.ScopedRole_builder{
				Kind:    KindScopedRole,
				SubKind: "unknown",
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing name",
			role: scopedaccessv1.ScopedRole_builder{
				Kind:     KindScopedRole,
				Metadata: &headerv1.Metadata{},
				Scope:    "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing kind",
			role: scopedaccessv1.ScopedRole_builder{
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing scope",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing version",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
				}.Build(),
			}.Build(),
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "malformed name",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "foo/bar",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "malformed kind",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: "role",
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "slightly malformed scope",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "foo/bar",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo/bar"},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "siginifcantly malformed scope",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "foo@bar",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo/bar"},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing assignable scopes",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope:   "/",
				Spec:    &scopedaccessv1.ScopedRoleSpec{},
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "slightly malformed assignable scope",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"foo/bar"},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "siginifcantly malformed assignable scope",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"foo@bar"},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true, // invalid assignable scopes are totally ignored, even if they don't pass weak validation rules
		},
		{
			name: "impermissable assignable scope",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/foo/bar",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "root assignable scope",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/"},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "invalid ssh.client_idle_timeout",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
					Ssh: scopedaccessv1.ScopedRoleSSH_builder{
						ClientIdleTimeout: "not-a-duration",
					}.Build(),
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "invalid defaults.client_idle_timeout",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
					Defaults: scopedaccessv1.ScopedRoleDefaults_builder{
						ClientIdleTimeout: "not-a-duration",
					}.Build(),
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "invalid create_host_user_mode",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
					Ssh: scopedaccessv1.ScopedRoleSSH_builder{
						HostUserCreation: scopedaccessv1.CreateHostUser_builder{
							Mode: "invalid-mode",
						}.Build(),
					}.Build(),
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "valid create_host_user_mode",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
					Ssh: scopedaccessv1.ScopedRoleSSH_builder{
						HostUserCreation: scopedaccessv1.CreateHostUser_builder{
							Mode: "keep",
						}.Build(),
					}.Build(),
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: true,
			weakOk:   true,
		},
		{
			name: "negative max_sessions",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
					Ssh: scopedaccessv1.ScopedRoleSSH_builder{
						MaxSessions: proto.Int64(-1),
					}.Build(),
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "positive max_sessions",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
					Ssh: scopedaccessv1.ScopedRoleSSH_builder{
						MaxSessions: proto.Int64(1),
					}.Build(),
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: true,
			weakOk:   true,
		},
		{
			name: "invalid defaults.session_recording_mode",
			role: scopedaccessv1.ScopedRole_builder{
				Kind:     KindScopedRole,
				Metadata: headerv1.Metadata_builder{Name: "test"}.Build(),
				Scope:    "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
					Defaults: scopedaccessv1.ScopedRoleDefaults_builder{
						SessionRecording: scopedaccessv1.SessionRecording_builder{
							Mode: "blah",
						}.Build(),
					}.Build(),
				}.Build(),
			}.Build(),
		},
		{
			name: "invalid defaults.lock.mode",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
					Defaults: scopedaccessv1.ScopedRoleDefaults_builder{
						Lock: scopedaccessv1.Lock_builder{
							Mode: "invalid",
						}.Build(),
					}.Build(),
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "valid defaults.session_recording_mode",
			role: scopedaccessv1.ScopedRole_builder{
				Kind:     KindScopedRole,
				Metadata: headerv1.Metadata_builder{Name: "test"}.Build(),
				Scope:    "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
					Defaults: scopedaccessv1.ScopedRoleDefaults_builder{
						SessionRecording: scopedaccessv1.SessionRecording_builder{
							Mode: string(constants.SessionRecordingModeStrict),
						}.Build(),
					}.Build(),
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: true,
			weakOk:   true,
		},
		{
			name: "invalid ssh.session_recording_mode",
			role: scopedaccessv1.ScopedRole_builder{
				Kind:     KindScopedRole,
				Metadata: headerv1.Metadata_builder{Name: "test"}.Build(),
				Scope:    "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
					Ssh: scopedaccessv1.ScopedRoleSSH_builder{
						SessionRecording: scopedaccessv1.SessionRecording_builder{
							Mode: "blah",
						}.Build(),
					}.Build(),
				}.Build(),
			}.Build(),
		},
		{
			name: "invalid ssh.lock.mode",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
					Ssh: scopedaccessv1.ScopedRoleSSH_builder{
						Lock: scopedaccessv1.Lock_builder{
							Mode: "invalid",
						}.Build(),
					}.Build(),
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "valid ssh.session_recording_mode",
			role: scopedaccessv1.ScopedRole_builder{
				Kind:     KindScopedRole,
				Metadata: headerv1.Metadata_builder{Name: "test"}.Build(),
				Scope:    "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
					Ssh: scopedaccessv1.ScopedRoleSSH_builder{
						SessionRecording: scopedaccessv1.SessionRecording_builder{
							Mode: string(constants.SessionRecordingModeStrict),
						}.Build(),
					}.Build(),
				}.Build(),
			}.Build(),
		},
		{
			name: "valid ssh.lock.mode",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
					Ssh: scopedaccessv1.ScopedRoleSSH_builder{
						Lock: scopedaccessv1.Lock_builder{
							Mode: string(constants.LockingModeStrict),
						}.Build(),
					}.Build(),
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: true,
			weakOk:   true,
		},

		{
			name: "empty ssh.lock.mode",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
					Ssh: scopedaccessv1.ScopedRoleSSH_builder{
						Lock: scopedaccessv1.Lock_builder{
							Mode: "",
						}.Build(),
					}.Build(),
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: true,
			weakOk:   true,
		},
		{
			name: "invalid kube.lock.mode",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
					Kube: scopedaccessv1.ScopedRoleKube_builder{
						Lock: scopedaccessv1.Lock_builder{
							Mode: "invalid",
						}.Build(),
					}.Build(),
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "valid Kube.lock.mode",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
					Kube: scopedaccessv1.ScopedRoleKube_builder{
						Lock: scopedaccessv1.Lock_builder{
							Mode: string(constants.LockingModeStrict),
						}.Build(),
					}.Build(),
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: true,
			weakOk:   true,
		},
		{
			name: "empty Kube.lock.mode",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
					Kube: scopedaccessv1.ScopedRoleKube_builder{
						Lock: scopedaccessv1.Lock_builder{
							Mode: "",
						}.Build(),
					}.Build(),
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: true,
			weakOk:   true,
		},
		{
			name: "invalid defaults.lock.mode",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
					Defaults: scopedaccessv1.ScopedRoleDefaults_builder{
						Lock: scopedaccessv1.Lock_builder{
							Mode: "invalid",
						}.Build(),
					}.Build(),
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "valid defaults.lock.mode",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
					Defaults: scopedaccessv1.ScopedRoleDefaults_builder{
						Lock: scopedaccessv1.Lock_builder{
							Mode: string(constants.LockingModeStrict),
						}.Build(),
					}.Build(),
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: true,
			weakOk:   true,
		},
		{
			name: "empty defaults.lock.mode",
			role: scopedaccessv1.ScopedRole_builder{
				Kind: KindScopedRole,
				Metadata: headerv1.Metadata_builder{
					Name: "test",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: []string{"/foo"},
					Defaults: scopedaccessv1.ScopedRoleDefaults_builder{
						Lock: scopedaccessv1.Lock_builder{
							Mode: "",
						}.Build(),
					}.Build(),
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: true,
			weakOk:   true,
		},
	}
	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			err := StrongValidateRole(tt.role)
			if tt.strongOk {
				require.NoError(t, err, "strong validation should not fail")
			} else {
				require.Error(t, err, "strong validation should fail")
			}

			err = WeakValidateRole(tt.role)
			if tt.weakOk {
				require.NoError(t, err, "weak validation should not fail")
			} else {
				require.Error(t, err, "weak validation should fail")
			}
		})
	}
}

func TestValidateAsssignment(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name       string
		assignment *scopedaccessv1.ScopedRoleAssignment
		strongOk   bool
		weakOk     bool
	}{
		{
			name: "basic",
			assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind:    KindScopedRoleAssignment,
				SubKind: SubKindDynamic,
				Metadata: headerv1.Metadata_builder{
					Name: uuid.New().String(),
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					User: "alice",
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  "test",
							Scope: "/foo",
						}.Build(),
					},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: true,
			weakOk:   true,
		},
		{
			name: "unknown sub_kind",
			assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind:    KindScopedRoleAssignment,
				SubKind: "unknown",
				Metadata: headerv1.Metadata_builder{
					Name: uuid.New().String(),
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					User: "alice",
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  "test",
							Scope: "/foo",
						}.Build(),
					},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "missing name",
			assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind:     KindScopedRoleAssignment,
				Metadata: &headerv1.Metadata{},
				Scope:    "/",
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					User: "alice",
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  "test",
							Scope: "/foo",
						}.Build(),
					},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing kind",
			assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Metadata: headerv1.Metadata_builder{
					Name: uuid.New().String(),
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					User: "alice",
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  "test",
							Scope: "/foo",
						}.Build(),
					},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing sub_kind",
			assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind: KindScopedRoleAssignment,
				Metadata: headerv1.Metadata_builder{
					Name: uuid.New().String(),
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					User: "alice",
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  "test",
							Scope: "/foo",
						}.Build(),
					},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "missing scope",
			assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind:    KindScopedRoleAssignment,
				SubKind: SubKindDynamic,
				Metadata: headerv1.Metadata_builder{
					Name: uuid.New().String(),
				}.Build(),
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					User: "alice",
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  "test",
							Scope: "/foo",
						}.Build(),
					},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "missing version",
			assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind:    KindScopedRoleAssignment,
				SubKind: SubKindDynamic,
				Metadata: headerv1.Metadata_builder{
					Name: uuid.New().String(),
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					User: "alice",
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  "test",
							Scope: "/foo",
						}.Build(),
					},
				}.Build(),
			}.Build(),
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "malformed name - long name",
			assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind:    KindScopedRoleAssignment,
				SubKind: SubKindDynamic,
				Metadata: headerv1.Metadata_builder{
					Name: "thisiswaytoolongofanameaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					User: "alice",
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  "test",
							Scope: "/foo",
						}.Build(),
					},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "malformed kind",
			assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind: "not_scoped_role_assignment",
				Metadata: headerv1.Metadata_builder{
					Name: uuid.New().String(),
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					User: "alice",
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  "test",
							Scope: "/foo",
						}.Build(),
					},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "slightly malformed scope",
			assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind:    KindScopedRoleAssignment,
				SubKind: SubKindDynamic,
				Metadata: headerv1.Metadata_builder{
					Name: uuid.New().String(),
				}.Build(),
				Scope: "foo",
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					User: "alice",
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  "test",
							Scope: "/foo",
						}.Build(),
					},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "significantly malformed scope",
			assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind:    KindScopedRoleAssignment,
				SubKind: SubKindDynamic,
				Metadata: headerv1.Metadata_builder{
					Name: uuid.New().String(),
				}.Build(),
				Scope: "foo@bar",
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					User: "alice",
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  "test",
							Scope: "/foo",
						}.Build(),
					},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   false,
		},
		{
			name: "impermissable assigned scope",
			assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind:    KindScopedRoleAssignment,
				SubKind: SubKindDynamic,
				Metadata: headerv1.Metadata_builder{
					Name: uuid.New().String(),
				}.Build(),
				Scope: "/foo/bar",
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					User: "alice",
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  "test",
							Scope: "/foo",
						}.Build(),
					},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "malformed assigned scope",
			assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind:    KindScopedRoleAssignment,
				SubKind: SubKindDynamic,
				Metadata: headerv1.Metadata_builder{
					Name: uuid.New().String(),
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					User: "alice",
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  "test",
							Scope: "foo",
						}.Build(),
					},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "root scope of effect",
			assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind: KindScopedRoleAssignment,
				Metadata: headerv1.Metadata_builder{
					Name: uuid.New().String(),
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					User: "alice",
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  "test",
							Scope: "/",
						}.Build(),
					},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "basic",
			assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind:    KindScopedRoleAssignment,
				SubKind: SubKindDynamic,
				Metadata: headerv1.Metadata_builder{
					Name: uuid.New().String(),
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					User: "alice",
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  "test",
							Scope: "/foo",
						}.Build(),
					},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: true,
			weakOk:   true,
		},
		{
			name: "valid bot assignment",
			assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind:    KindScopedRoleAssignment,
				SubKind: SubKindDynamic,
				Metadata: headerv1.Metadata_builder{
					Name: uuid.New().String(),
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					BotName:  "mybot",
					BotScope: "/foo/bar",
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  "test",
							Scope: "/foo/bar/child",
						}.Build(),
					},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: true,
			weakOk:   true,
		},
		{
			name: "bot_name and user both set",
			assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind:    KindScopedRoleAssignment,
				SubKind: SubKindDynamic,
				Metadata: headerv1.Metadata_builder{
					Name: uuid.New().String(),
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					User:     "alice",
					BotName:  "mybot",
					BotScope: "/foo/bar",
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  "test",
							Scope: "/foo",
						}.Build(),
					},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "bot_name without bot_scope",
			assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind:    KindScopedRoleAssignment,
				SubKind: SubKindDynamic,
				Metadata: headerv1.Metadata_builder{
					Name: uuid.New().String(),
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					BotName: "mybot",
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  "test",
							Scope: "/foo",
						}.Build(),
					},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "bot_scope without bot_name",
			assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind:    KindScopedRoleAssignment,
				SubKind: SubKindDynamic,
				Metadata: headerv1.Metadata_builder{
					Name: uuid.New().String(),
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					User:     "alice",
					BotScope: "/foo/bar",
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  "test",
							Scope: "/foo",
						}.Build(),
					},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "invalid bot_scope",
			assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind:    KindScopedRoleAssignment,
				SubKind: SubKindDynamic,
				Metadata: headerv1.Metadata_builder{
					Name: uuid.New().String(),
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					BotName:  "mybot",
					BotScope: "not-a-scope",
					Assignments: []*scopedaccessv1.Assignment{
						scopedaccessv1.Assignment_builder{
							Role:  "test",
							Scope: "/foo",
						}.Build(),
					},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
		{
			name: "sub-assignment scope outside bot scope",
			assignment: scopedaccessv1.ScopedRoleAssignment_builder{
				Kind:    KindScopedRoleAssignment,
				SubKind: SubKindDynamic,
				Metadata: headerv1.Metadata_builder{
					Name: uuid.New().String(),
				}.Build(),
				Scope: "/",
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					BotName:  "mybot",
					BotScope: "/foo/bar",
					Assignments: []*scopedaccessv1.Assignment{
						// Valid
						scopedaccessv1.Assignment_builder{
							Role:  "test",
							Scope: "/foo/bar",
						}.Build(),
						// Invalid
						scopedaccessv1.Assignment_builder{
							Role:  "test",
							Scope: "/foo/baz",
						}.Build(),
					},
				}.Build(),
				Version: types.V1,
			}.Build(),
			strongOk: false,
			weakOk:   true,
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			err := StrongValidateAssignment(tt.assignment)
			if tt.strongOk {
				require.NoError(t, err, "strong validation should not fail")
			} else {
				require.Error(t, err, "strong validation should fail")
			}

			err = WeakValidateAssignment(tt.assignment)
			if tt.weakOk {
				require.NoError(t, err, "weak validation should not fail")
			} else {
				require.Error(t, err, "weak validation should fail")
			}
		})
	}
}

func TestWeakValidatedAssignableScopes(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name             string
		scope            string
		assignableScopes []string
		expect           []string
	}{
		{
			name:             "basic",
			scope:            "/foo",
			assignableScopes: []string{"/foo/bar", "/foo/baz"},
			expect:           []string{"/foo/bar", "/foo/baz"},
		},
		{
			name:             "empty",
			scope:            "/foo",
			assignableScopes: nil,
			expect:           nil,
		},
		{
			name:             "mildly malformed",
			scope:            "/foo",
			assignableScopes: []string{"foo/bar", "foo/baz"},
			expect:           []string{"foo/bar", "foo/baz"},
		},
		{
			name:             "significantly malformed",
			scope:            "/foo",
			assignableScopes: []string{"foo@bar", "foo@baz"},
			expect:           nil,
		},
		{
			name:             "impermissible",
			scope:            "/foo/bar",
			assignableScopes: []string{"/foo", "/foo/bar"},
			expect:           []string{"/foo/bar"},
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			result := slices.Collect(WeakValidatedAssignableScopes(scopedaccessv1.ScopedRole_builder{
				Scope: tt.scope,
				Spec: scopedaccessv1.ScopedRoleSpec_builder{
					AssignableScopes: tt.assignableScopes,
				}.Build(),
			}.Build()))
			require.Equal(t, tt.expect, result)
		})
	}
}

func TestWeakValidatedSubAssignments(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name        string
		scope       string
		assignments []*scopedaccessv1.Assignment
		expect      []*scopedaccessv1.Assignment
	}{
		{
			name:  "basic",
			scope: "/foo",
			assignments: []*scopedaccessv1.Assignment{
				scopedaccessv1.Assignment_builder{
					Role:  "test",
					Scope: "/foo/bar",
				}.Build(),
				scopedaccessv1.Assignment_builder{
					Role:  "test",
					Scope: "/foo/baz",
				}.Build(),
			},
			expect: []*scopedaccessv1.Assignment{
				scopedaccessv1.Assignment_builder{
					Role:  "test",
					Scope: "/foo/bar",
				}.Build(),
				scopedaccessv1.Assignment_builder{
					Role:  "test",
					Scope: "/foo/baz",
				}.Build(),
			},
		},
		{
			name:  "mildly malformed scope",
			scope: "/foo",
			assignments: []*scopedaccessv1.Assignment{
				scopedaccessv1.Assignment_builder{
					Role:  "test",
					Scope: "foo/bar",
				}.Build(),
				scopedaccessv1.Assignment_builder{
					Role:  "test",
					Scope: "/foo/baz",
				}.Build(),
			},
			expect: []*scopedaccessv1.Assignment{
				scopedaccessv1.Assignment_builder{
					Role:  "test",
					Scope: "foo/bar",
				}.Build(),
				scopedaccessv1.Assignment_builder{
					Role:  "test",
					Scope: "/foo/baz",
				}.Build(),
			},
		},
		{
			name:  "significantly malformed scope",
			scope: "/foo",
			assignments: []*scopedaccessv1.Assignment{
				scopedaccessv1.Assignment_builder{
					Role:  "test",
					Scope: "foo@bar",
				}.Build(),
				scopedaccessv1.Assignment_builder{
					Role:  "test",
					Scope: "/foo/baz",
				}.Build(),
			},
			expect: []*scopedaccessv1.Assignment{
				scopedaccessv1.Assignment_builder{
					Role:  "test",
					Scope: "/foo/baz",
				}.Build(),
			},
		},
		{
			name:  "impermissible scope",
			scope: "/foo/bar",
			assignments: []*scopedaccessv1.Assignment{
				scopedaccessv1.Assignment_builder{
					Role:  "test",
					Scope: "/foo/bar",
				}.Build(),
				scopedaccessv1.Assignment_builder{
					Role:  "test",
					Scope: "/foo/baz",
				}.Build(),
			},
			expect: []*scopedaccessv1.Assignment{
				scopedaccessv1.Assignment_builder{
					Role:  "test",
					Scope: "/foo/bar",
				}.Build(),
			},
		},
		{
			name:  "missing scope",
			scope: "/foo",
			assignments: []*scopedaccessv1.Assignment{
				scopedaccessv1.Assignment_builder{
					Role:  "test",
					Scope: "",
				}.Build(),
				scopedaccessv1.Assignment_builder{
					Role:  "test",
					Scope: "/foo/baz",
				}.Build(),
			},
			expect: []*scopedaccessv1.Assignment{
				scopedaccessv1.Assignment_builder{
					Role:  "test",
					Scope: "/foo/baz",
				}.Build(),
			},
		},
		{
			name:  "missing role",
			scope: "/foo",
			assignments: []*scopedaccessv1.Assignment{
				scopedaccessv1.Assignment_builder{
					Role:  "test",
					Scope: "/foo/bar",
				}.Build(),
				scopedaccessv1.Assignment_builder{
					Role:  "",
					Scope: "/foo/baz",
				}.Build(),
			},
			expect: []*scopedaccessv1.Assignment{
				scopedaccessv1.Assignment_builder{
					Role:  "test",
					Scope: "/foo/bar",
				}.Build(),
			},
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			result := slices.Collect(WeakValidatedSubAssignments(scopedaccessv1.ScopedRoleAssignment_builder{
				Scope: tt.scope,
				Spec: scopedaccessv1.ScopedRoleAssignmentSpec_builder{
					Assignments: tt.assignments,
				}.Build(),
			}.Build()))
			require.Equal(t, tt.expect, result)
		})
	}
}

// requireAllFieldsHavePresence recursively verifies that all non-sequence fileds in a proto message have
// presence enabled (for proto3 this typically means that scalar fields are marked optional).
func requireAllFieldsHavePresence(t *testing.T, msg proto.Message) {
	requireAllFieldsHavePresenceRecursive(t, msg.ProtoReflect().Descriptor())
}

func requireAllFieldsHavePresenceRecursive(t *testing.T, descriptor protoreflect.MessageDescriptor) {
	fields := descriptor.Fields()
	for i := range fields.Len() {
		field := fields.Get(i)

		// recursively check nested fields.
		if field.Kind() == protoreflect.MessageKind {
			// note: MessageKind fields covers both singular and repeated messages
			requireAllFieldsHavePresenceRecursive(t, field.Message())
		}
		if field.IsMap() && field.MapValue().Kind() == protoreflect.MessageKind {
			requireAllFieldsHavePresenceRecursive(t, field.MapValue().Message())
		}

		// skip lists/strings/bytes since empty sequences aren't as concerning as false/0 when it comes to distinguishing
		// unset vs set to zero value.
		if field.IsList() || field.Kind() == protoreflect.StringKind || field.Kind() == protoreflect.BytesKind {
			continue
		}

		// require that presence is enabled. if you are adding a new field to scoped roles and run into this check failing,
		// you likely need to make the field optional.
		require.True(t, field.HasPresence(),
			"field %s.%s must have presence enabled (use optional for scalar fields)",
			descriptor.FullName(), field.Name())
	}
}

// TestScopedRoleSpecFieldsHavePresence verifies that the scoped role spec and its members
// have presence enabled for all non-sequence fields.
// See proto comments for the ScopedRoleSpec type for discussion of this policy.
func TestScopedRoleSpecFieldsHavePresence(t *testing.T) {
	t.Parallel()
	requireAllFieldsHavePresence(t, (*scopedaccessv1.ScopedRoleSpec)(nil))
}

// TestScopedRoleSpecTopLevelFieldsAreMessages verifies that all top-level fields of ScopedRoleSpec
// are message types (singular or repeated), with the exception of assignable_scopes which is
// grandfathered in. This policy exists to ensure that top-level spec fields remain extensible and
// composable over time. Scalar and enum fields added at the top level cannot be grouped or namespaced
// after the fact without breaking changes, whereas message fields can always grow new sub-fields.
// If you are adding a new top-level field to ScopedRoleSpec and this test fails, wrap it in a message.
func TestScopedRoleSpecTopLevelFieldsAreMessages(t *testing.T) {
	t.Parallel()

	// grandfathered fields that predate this policy and are exempt from the requirement.
	grandfathered := map[protoreflect.Name]bool{
		"assignable_scopes": true,
	}

	descriptor := (*scopedaccessv1.ScopedRoleSpec)(nil).ProtoReflect().Descriptor()
	fields := descriptor.Fields()
	for i := range fields.Len() {
		field := fields.Get(i)
		if grandfathered[field.Name()] {
			continue
		}
		require.Equal(t, protoreflect.MessageKind, field.Kind(),
			"top-level field %s.%s must be a message type (singular or repeated), not a scalar or enum; wrap it in a message or add it to an existing one",
			descriptor.FullName(), field.Name())
	}
}

// TestStrongValidateRoleSpecAllFieldsValidated verifies that a maximally-populated valid ScopedRoleSpec
// passes StrongValidateRole. The ExhaustiveNonEmpty assertion acts as a coverage guard: if you add a new
// field to ScopedRoleSpec (or any of its nested message types) and this test begins failing, you must set
// that field to a valid non-zero value in the spec below. Once you've done that, consider whether
// StrongValidateRole needs to validate the new field and add coverage to TestValidateRole if so.
func TestStrongValidateRoleSpecAllFieldsValidated(t *testing.T) {
	t.Parallel()

	spec := scopedaccessv1.ScopedRoleSpec_builder{
		AssignableScopes: []string{"/foo"},
		Defaults: scopedaccessv1.ScopedRoleDefaults_builder{
			ClientIdleTimeout: "30m",
			SessionRecording: scopedaccessv1.SessionRecording_builder{
				Mode: string(constants.SessionRecordingModeStrict),
			}.Build(),
			Lock: scopedaccessv1.Lock_builder{
				Mode: "strict",
			}.Build(),
			DisconnectExpiredCert: ptr(true),
		}.Build(),
		Rules: []*scopedaccessv1.ScopedRule{
			scopedaccessv1.ScopedRule_builder{
				Resources: []string{KindScopedRole},
				Verbs:     []string{types.VerbReadNoSecrets},
			}.Build(),
		},
		Ssh: scopedaccessv1.ScopedRoleSSH_builder{
			Logins: []string{"alice"},
			Labels: []*labelv1.Label{
				labelv1.Label_builder{Name: "env", Values: []string{"prod"}}.Build(),
			},
			ClientIdleTimeout:   "1h",
			PermitX11Forwarding: proto.Bool(true),
			FileCopy:            proto.Bool(true),
			ForwardAgent:        proto.Bool(true),
			PortForwarding: scopedaccessv1.SSHPortForwarding_builder{
				Local:  scopedaccessv1.SSHLocalPortForwarding_builder{Enabled: proto.Bool(true)}.Build(),
				Remote: scopedaccessv1.SSHRemotePortForwarding_builder{Enabled: proto.Bool(true)}.Build(),
			}.Build(),
			HostUserCreation: scopedaccessv1.CreateHostUser_builder{
				Mode:   "keep",
				Groups: []string{"wheel"},
				Shell:  "/bin/bash",
			}.Build(),
			HostSudoers: []string{"ALL=(ALL) NOPASSWD:ALL"},
			MaxSessions: proto.Int64(10),
			EnhancedRecording: scopedaccessv1.EnhancedRecording_builder{
				Disk:    proto.Bool(true),
				Network: proto.Bool(true),
				Command: proto.Bool(true),
			}.Build(),
			SessionRecording: scopedaccessv1.SessionRecording_builder{
				Mode: string(constants.SessionRecordingModeStrict),
			}.Build(),
			Lock: scopedaccessv1.Lock_builder{
				Mode: string(constants.LockingModeBestEffort),
			}.Build(),
			DisconnectExpiredCert: proto.Bool(true),
		}.Build(),
		Kube: scopedaccessv1.ScopedRoleKube_builder{
			Groups: []string{"viewer"},
			Users:  []string{"alice"},
			Labels: []*labelv1.Label{
				labelv1.Label_builder{Name: "env", Values: []string{"prod"}}.Build(),
			},
			ClientIdleTimeout:     "1h",
			DisconnectExpiredCert: ptr(true),
			Lock: scopedaccessv1.Lock_builder{
				Mode: "strict",
			}.Build(),
		}.Build(),
	}.Build()

	require.True(t, testutils.ExhaustiveNonEmpty(spec),
		"spec is not exhaustively non-empty; if you added a new field, set it to a valid non-zero value here AND evaluate whether StrongValidateRole needs to validate it (adding test cases to TestValidateRole if so) — empty fields: %v",
		testutils.FindAllEmpty(spec),
	)

	role := scopedaccessv1.ScopedRole_builder{
		Kind:     KindScopedRole,
		Metadata: headerv1.Metadata_builder{Name: "test"}.Build(),
		Scope:    "/",
		Spec:     spec,
		Version:  types.V1,
	}.Build()
	require.NoError(t, StrongValidateRole(role))
}
