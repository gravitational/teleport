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

func TestStrongValidateScopedToken(t *testing.T) {
	cases := []struct {
		name        string
		token       *joiningv1.ScopedToken
		expectedErr string
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
				},
			},
			expectedErr: fmt.Sprintf("expected kind %v, got %q", types.KindScopedToken, ""),
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
				},
			},
			expectedErr: fmt.Sprintf("expected version %v, got %q", types.V1, ""),
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
				},
			},
			expectedErr: fmt.Sprintf("expected sub_kind %v, got %q", "", "subkind"),
		},
		{
			name: "missing name",
			token: &joiningv1.ScopedToken{
				Kind:    types.KindScopedToken,
				Version: types.V1,
				Scope:   "/aa",
				Spec: &joiningv1.ScopedTokenSpec{
					Roles: []string{types.RoleNode.String()},
				},
			},
			expectedErr: "missing name",
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
			expectedErr: "spec must not be nil",
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
				},
			},
			expectedErr: "scoped token must have a scope assigned",
		},
		{
			name: "invalid scope",
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
				},
			},
			expectedErr: "validating scoped token resource scope",
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
					Roles: []string{types.RoleNode.String()},
				},
			},
			expectedErr: "validating scoped token assigned scope",
		},
		{
			name: "invalid assigned scope",
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
				},
			},
			expectedErr: "validating scoped token assigned scope",
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
				},
			},
			expectedErr: "scoped token assigned scope must be descendant of its resource scope",
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
				},
			},
			expectedErr: "scoped token must have at least one role",
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
				},
			},
			expectedErr: "validating scoped token roles",
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
				},
			},
			expectedErr: fmt.Sprintf("role %q does not support scoping", types.RoleInstance),
		},
		{
			// TODO (eriktate): remove this when scoped tokens support bot joins
			name: "includes bot name",
			token: &joiningv1.ScopedToken{
				Kind:    types.KindScopedToken,
				Scope:   "/aa/bb",
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "testtoken",
				},
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "/aa/bb",
					Roles:         []string{types.RoleNode.String()},
					BotName:       "test_bot",
				},
			},
			expectedErr: "scoped tokens do not support the bot role or bot names",
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
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := joining.StrongValidateToken(c.token)
			if c.expectedErr != "" {
				assert.ErrorContains(t, err, c.expectedErr)
			}
		})
	}
}

func TestWeakValidateScopedToken(t *testing.T) {
	cases := []struct {
		name        string
		token       *joiningv1.ScopedToken
		expectedErr string
	}{
		{
			name:        "nil token",
			expectedErr: "missing scoped token",
		},
		{
			name: "missing scope",
			token: &joiningv1.ScopedToken{
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "/aa/bb",
					Roles:         []string{types.RoleNode.String()},
				},
			},
			expectedErr: "validating scoped token resource scope",
		},
		{
			name: "invalid scope",
			token: &joiningv1.ScopedToken{
				Scope: "/aa/bb}",
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "/aa/bb",
					Roles:         []string{types.RoleNode.String()},
				},
			},
			expectedErr: "validating scoped token resource scope",
		},
		{
			name: "missing assigned scope",
			token: &joiningv1.ScopedToken{
				Scope: "/aa",
				Spec: &joiningv1.ScopedTokenSpec{
					Roles: []string{types.RoleNode.String()},
				},
			},
			expectedErr: "validating scoped token assigned scope",
		},
		{
			name: "invalid assigned scope",
			token: &joiningv1.ScopedToken{
				Scope: "/aa/bb",
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "/aa/bb}",
					Roles:         []string{types.RoleNode.String()},
				},
			},
			expectedErr: "validating scoped token assigned scope",
		},
		{
			name: "missing roles",
			token: &joiningv1.ScopedToken{
				Scope: "/aa",
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "/aa/bb",
				},
			},
			expectedErr: "scoped token must have at least one role",
		},
		{
			name: "valid scoped tokens",
			token: &joiningv1.ScopedToken{
				Scope: "/aa",
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "/aa/bb",
					Roles:         []string{types.RoleNode.String()},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := joining.WeakValidateToken(c.token)
			if c.expectedErr != "" {
				assert.ErrorContains(t, err, c.expectedErr)
			}
		})
	}
}
