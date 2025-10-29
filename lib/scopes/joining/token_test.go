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

package joining_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes/joining"
)

func TestValidateScopedToken(t *testing.T) {
	cases := []struct {
		name              string
		token             *joiningv1.ScopedToken
		expectedStrongErr string
		expectedWeakErr   string
	}{
		{
			name: "invalid kind",
			token: &joiningv1.ScopedToken{
				Version: types.V1,
				Scope:   "/aa",
				Metadata: &headerv1.Metadata{
					Name: "testtoken",
				},
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "/aa/bb",
					Roles:         []string{types.RoleNode.String()},
					JoinMethod:    string(types.JoinMethodToken),
				},
			},
			expectedStrongErr: fmt.Sprintf("expected kind %v, got %q", types.KindScopedToken, ""),
		},
		{
			name: "invalid version",
			token: &joiningv1.ScopedToken{
				Kind:  types.KindScopedToken,
				Scope: "/aa",
				Metadata: &headerv1.Metadata{
					Name: "testtoken",
				},
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "/aa/bb",
					Roles:         []string{types.RoleNode.String()},
					JoinMethod:    string(types.JoinMethodToken),
				},
			},
			expectedStrongErr: fmt.Sprintf("expected version %v, got %q", types.V1, ""),
		},
		{
			name: "invalid subkind",
			token: &joiningv1.ScopedToken{
				Kind:    types.KindScopedToken,
				Version: types.V1,
				Scope:   "/aa",
				SubKind: "subkind",
				Metadata: &headerv1.Metadata{
					Name: "testtoken",
				},
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "/aa/bb",
					Roles:         []string{types.RoleNode.String()},
					JoinMethod:    string(types.JoinMethodToken),
				},
			},
			expectedStrongErr: fmt.Sprintf("expected sub_kind %v, got %q", "", "subkind"),
		},
		{
			name: "missing name",
			token: &joiningv1.ScopedToken{
				Kind:    types.KindScopedToken,
				Version: types.V1,
				Scope:   "/aa",
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "/aa/bb",
					Roles:         []string{types.RoleNode.String()},
					JoinMethod:    string(types.JoinMethodToken),
				},
			},
			expectedStrongErr: "missing name",
		},
		{
			name: "missing spec",
			token: &joiningv1.ScopedToken{
				Kind:    types.KindScopedToken,
				Version: types.V1,
				Scope:   "/aa",
				Metadata: &headerv1.Metadata{
					Name: "testtoken",
				},
			},
			expectedStrongErr: "spec must not be nil",
			expectedWeakErr:   "validating scoped token assigned scope",
		},
		{
			name: "missing scope",
			token: &joiningv1.ScopedToken{
				Kind:    types.KindScopedToken,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "testtoken",
				},
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "/aa/bb",
					Roles:         []string{types.RoleNode.String()},
					JoinMethod:    string(types.JoinMethodToken),
				},
			},
			expectedStrongErr: "scoped token must have a scope assigned",
			expectedWeakErr:   "validating scoped token resource scope",
		},
		{
			name: "non-absolute scope",
			token: &joiningv1.ScopedToken{
				Kind:    types.KindScopedToken,
				Scope:   "aa/bb",
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "testtoken",
				},
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "/aa/bb",
					Roles:         []string{types.RoleNode.String()},
					JoinMethod:    string(types.JoinMethodToken),
				},
			},
			expectedStrongErr: "validating scoped token resource scope",
		},
		{
			name: "scope with invalid characters",
			token: &joiningv1.ScopedToken{
				Kind:    types.KindScopedToken,
				Scope:   "/aa/bb}",
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "testtoken",
				},
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "/aa/bb",
					Roles:         []string{types.RoleNode.String()},
					JoinMethod:    string(types.JoinMethodToken),
				},
			},
			expectedStrongErr: "validating scoped token resource scope",
			expectedWeakErr:   "validating scoped token resource scope",
		},
		{
			name: "missing assigned scope",
			token: &joiningv1.ScopedToken{
				Kind:    types.KindScopedToken,
				Scope:   "/aa/bb",
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "testtoken",
				},
				Spec: &joiningv1.ScopedTokenSpec{
					Roles:      []string{types.RoleNode.String()},
					JoinMethod: string(types.JoinMethodToken),
				},
			},
			expectedStrongErr: "validating scoped token assigned scope",
			expectedWeakErr:   "validating scoped token assigned scope",
		},
		{
			name: "non-absolute assigned scope",
			token: &joiningv1.ScopedToken{
				Kind:    types.KindScopedToken,
				Scope:   "/aa/bb",
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "testtoken",
				},
				Spec: &joiningv1.ScopedTokenSpec{
					Roles:         []string{types.RoleNode.String()},
					AssignedScope: "aa/bb",
					JoinMethod:    string(types.JoinMethodToken),
				},
			},
			expectedStrongErr: "validating scoped token assigned scope",
		},
		{
			name: "assigned scope with invalid character",
			token: &joiningv1.ScopedToken{
				Kind:    types.KindScopedToken,
				Scope:   "/aa/bb",
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "testtoken",
				},
				Spec: &joiningv1.ScopedTokenSpec{
					Roles:         []string{types.RoleNode.String()},
					AssignedScope: "aa/bb}",
					JoinMethod:    string(types.JoinMethodToken),
				},
			},
			expectedStrongErr: "validating scoped token assigned scope",
			expectedWeakErr:   "validating scoped token assigned scope",
		},
		{
			name: "assigned scope is not descendant of token scope",
			token: &joiningv1.ScopedToken{
				Kind:    types.KindScopedToken,
				Scope:   "/aa/bb",
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "testtoken",
				},
				Spec: &joiningv1.ScopedTokenSpec{
					Roles:         []string{types.RoleNode.String()},
					AssignedScope: "/bb/aa",
					JoinMethod:    string(types.JoinMethodToken),
				},
			},
			expectedStrongErr: "scoped token assigned scope must be descendant of its resource scope",
		},
		{
			name: "invalid join method",
			token: &joiningv1.ScopedToken{
				Kind:    types.KindScopedToken,
				Scope:   "/aa/bb",
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "testtoken",
				},
				Spec: &joiningv1.ScopedTokenSpec{
					Roles:         []string{types.RoleNode.String()},
					AssignedScope: "/aa/bb",
					JoinMethod:    string(types.JoinMethodUnspecified),
				},
			},
			expectedStrongErr: fmt.Sprintf("join method %q does not support scoping", types.JoinMethodUnspecified),
			expectedWeakErr:   fmt.Sprintf("join method %q does not support scoping", types.JoinMethodUnspecified),
		},
		{
			name: "missing roles",
			token: &joiningv1.ScopedToken{
				Kind:    types.KindScopedToken,
				Scope:   "/aa/bb",
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "testtoken",
				},
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "/aa/bb",
					JoinMethod:    string(types.JoinMethodToken),
				},
			},
			expectedStrongErr: "scoped token must have at least one role",
			expectedWeakErr:   "scoped token must have at least one role",
		},
		{
			name: "invalid roles",
			token: &joiningv1.ScopedToken{
				Kind:    types.KindScopedToken,
				Scope:   "/aa/bb",
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "testtoken",
				},
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "/aa/bb",
					Roles:         []string{"random_role"},
					JoinMethod:    string(types.JoinMethodToken),
				},
			},
			expectedStrongErr: "validating scoped token roles",
		},
		{
			name: "unsupported roles",
			token: &joiningv1.ScopedToken{
				Kind:    types.KindScopedToken,
				Scope:   "/aa/bb",
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "testtoken",
				},
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "/aa/bb",
					Roles:         []string{types.RoleNode.String(), types.RoleInstance.String()},
					JoinMethod:    string(types.JoinMethodToken),
				},
			},
			expectedStrongErr: fmt.Sprintf("role %q does not support scoping", types.RoleInstance),
		},
		{
			name: "valid scoped token",
			token: &joiningv1.ScopedToken{
				Kind:    types.KindScopedToken,
				Scope:   "/aa/bb",
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "testtoken",
				},
				Spec: &joiningv1.ScopedTokenSpec{
					Roles:         []string{types.RoleNode.String()},
					AssignedScope: "/aa/bb",
					JoinMethod:    string(types.JoinMethodToken),
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := joining.StrongValidateToken(c.token)
			if c.expectedStrongErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, c.expectedStrongErr)
			}

			err = joining.WeakValidateToken(c.token)
			if c.expectedWeakErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, c.expectedWeakErr)
			}
		})
	}
}
