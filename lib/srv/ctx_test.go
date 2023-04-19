/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package srv

import (
	"bytes"
	"os/user"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// TestDecodeChildError ensures that child error message marshaling
// and unmarshaling returns the original values.
func TestDecodeChildError(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, DecodeChildError(&buf))

	targetErr := trace.NotFound(user.UnknownUserError("test").Error())

	writeChildError(&buf, targetErr)

	require.ErrorIs(t, DecodeChildError(&buf), targetErr)
}

func TestCheckSFTPAllowed(t *testing.T) {
	srv := newMockServer(t)
	ctx := newTestServerContext(t, srv, nil)

	tests := []struct {
		name                 string
		nodeAllowFileCopying bool
		roles                []types.Role
		expectedErr          error
	}{
		{
			name:                 "node disallowed",
			nodeAllowFileCopying: false,
			roles: []types.Role{
				&types.RoleV5{
					Kind: types.KindNode,
				},
			},
			expectedErr: ErrNodeFileCopyingNotPermitted,
		},
		{
			name:                 "node allowed",
			nodeAllowFileCopying: true,
			roles: []types.Role{
				&types.RoleV5{
					Kind: types.KindNode,
				},
			},
			expectedErr: nil,
		},
		{
			name:                 "role disallowed",
			nodeAllowFileCopying: true,
			roles: []types.Role{
				&types.RoleV5{
					Kind: types.KindNode,
					Spec: types.RoleSpecV5{
						Options: types.RoleOptions{
							SSHFileCopy: types.NewBoolOption(false),
						},
					},
				},
			},
			expectedErr: errRoleFileCopyingNotPermitted,
		},
		{
			name:                 "role allowed",
			nodeAllowFileCopying: true,
			roles: []types.Role{
				&types.RoleV5{
					Kind: types.KindNode,
					Spec: types.RoleSpecV5{
						Options: types.RoleOptions{
							SSHFileCopy: types.NewBoolOption(true),
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name:                 "conflicting roles",
			nodeAllowFileCopying: true,
			roles: []types.Role{
				&types.RoleV5{
					Kind: types.KindNode,
					Spec: types.RoleSpecV5{
						Options: types.RoleOptions{
							SSHFileCopy: types.NewBoolOption(true),
						},
					},
				},
				&types.RoleV5{
					Kind: types.KindNode,
					Spec: types.RoleSpecV5{
						Options: types.RoleOptions{
							SSHFileCopy: types.NewBoolOption(false),
						},
					},
				},
			},
			expectedErr: errRoleFileCopyingNotPermitted,
		},
		{
			name:                 "moderated sessions enforced",
			nodeAllowFileCopying: true,
			roles: []types.Role{
				&types.RoleV5{
					Kind: types.KindNode,
					Spec: types.RoleSpecV5{
						Allow: types.RoleConditions{
							RequireSessionJoin: []*types.SessionRequirePolicy{
								{
									Name:   "test",
									Filter: `contains(user.roles, "auditor")`,
									Kinds:  []string{string(types.SSHSessionKind)},
									Modes:  []string{string(types.SessionModeratorMode)},
									Count:  3,
								},
							},
						},
					},
				},
			},
			expectedErr: errCannotStartUnattendedSession,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx.AllowFileCopying = tt.nodeAllowFileCopying

			roles := services.NewRoleSet(tt.roles...)

			ctx.Identity.AccessChecker = services.NewAccessCheckerWithRoleSet(
				&services.AccessInfo{
					Roles: roles.RoleNames(),
				},
				"localhost",
				roles,
			)

			err := ctx.CheckSFTPAllowed()
			if tt.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.expectedErr.Error())
			}
		})
	}
}
