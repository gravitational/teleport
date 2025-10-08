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

package services_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func TestValidateScopedToken(t *testing.T) {
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
					Roles: []string{types.RoleNode.String()},
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
					Roles: []string{types.RoleNode.String()},
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
					Roles: []string{types.RoleNode.String()},
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
					Roles: []string{types.RoleNode.String()},
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
					Roles: []string{types.RoleNode.String()},
				},
			},
			expectedErr: "validating scoped token resource scope",
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
				Spec: &joiningv1.ScopedTokenSpec{},
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
					Roles:   []string{"random_role"},
					BotName: "test_bot",
				},
			},
			expectedErr: "validating scoped token roles",
		},
		{
			name: "missing bot_name when assigning bot role",
			token: &joiningv1.ScopedToken{
				Kind:    types.KindScopedToken,
				Scope:   "/aa/bb",
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "testtoken",
				},
				Spec: &joiningv1.ScopedTokenSpec{
					Roles: []string{types.RoleNode.String(), types.RoleBot.String()},
				},
			},
			expectedErr: fmt.Sprintf("scoped token with role %q must set bot_name", types.RoleBot),
		},
		{
			name: "missing bot role when assigning bot_name",
			token: &joiningv1.ScopedToken{
				Kind:    types.KindScopedToken,
				Scope:   "/aa/bb",
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "testtoken",
				},
				Spec: &joiningv1.ScopedTokenSpec{
					Roles:   []string{types.RoleNode.String()},
					BotName: "test_bot",
				},
			},
			expectedErr: fmt.Sprintf("can only set bot_name on scoped token with role %q", types.RoleBot),
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
					Roles:         []string{types.RoleNode.String(), types.RoleBot.String()},
					AssignedScope: "/aa/bb",
					BotName:       "test_bot",
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := services.ValidateScopedToken(c.token)
			if c.expectedErr != "" {
				assert.ErrorContains(t, err, c.expectedErr)
			}
		})
	}
}
