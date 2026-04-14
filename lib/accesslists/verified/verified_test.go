//go:build verified_accesslists

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

package verified

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/trait"
	accesslists "github.com/gravitational/teleport/lib/accesslists"
)

// TestEquivalence verifies that the Rust FFI implementation produces the same
// results as the Go implementation for a variety of inputs.
func TestEquivalence(t *testing.T) {
	tests := []struct {
		name     string
		roles    []string
		traits   trait.Traits
		requires accesslist.Requires
	}{
		{
			name:   "empty requirements",
			roles:  []string{"admin"},
			traits: trait.Traits{"team": {"infra"}},
			requires: accesslist.Requires{
				Roles:  nil,
				Traits: nil,
			},
		},
		{
			name:   "empty user empty requirements",
			roles:  nil,
			traits: nil,
			requires: accesslist.Requires{
				Roles:  nil,
				Traits: nil,
			},
		},
		{
			name:   "user has all required roles",
			roles:  []string{"admin", "editor", "viewer"},
			traits: nil,
			requires: accesslist.Requires{
				Roles: []string{"admin", "editor"},
			},
		},
		{
			name:   "user missing one required role",
			roles:  []string{"editor", "viewer"},
			traits: nil,
			requires: accesslist.Requires{
				Roles: []string{"admin", "editor"},
			},
		},
		{
			name:   "user has all required traits",
			roles:  nil,
			traits: trait.Traits{"team": {"infra", "platform"}, "org": {"eng"}},
			requires: accesslist.Requires{
				Traits: trait.Traits{"team": {"infra"}, "org": {"eng"}},
			},
		},
		{
			name:   "user missing trait key",
			roles:  nil,
			traits: trait.Traits{"team": {"infra"}},
			requires: accesslist.Requires{
				Traits: trait.Traits{"org": {"eng"}},
			},
		},
		{
			name:   "user missing trait value",
			roles:  nil,
			traits: trait.Traits{"team": {"infra"}},
			requires: accesslist.Requires{
				Traits: trait.Traits{"team": {"platform"}},
			},
		},
		{
			name:   "roles and traits both required and met",
			roles:  []string{"admin"},
			traits: trait.Traits{"team": {"infra"}},
			requires: accesslist.Requires{
				Roles:  []string{"admin"},
				Traits: trait.Traits{"team": {"infra"}},
			},
		},
		{
			name:   "roles pass but traits fail",
			roles:  []string{"admin"},
			traits: trait.Traits{"team": {"infra"}},
			requires: accesslist.Requires{
				Roles:  []string{"admin"},
				Traits: trait.Traits{"team": {"platform"}},
			},
		},
		{
			name:   "traits pass but roles fail",
			roles:  []string{"viewer"},
			traits: trait.Traits{"team": {"infra"}},
			requires: accesslist.Requires{
				Roles:  []string{"admin"},
				Traits: trait.Traits{"team": {"infra"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := types.NewUser("test-user")
			require.NoError(t, err)
			if tt.roles != nil {
				user.SetRoles(tt.roles)
			}
			if tt.traits != nil {
				user.SetTraits(tt.traits)
			}

			goResult := accesslists.UserMeetsRequirements(user, tt.requires)
			rustResult, err := UserMeetsRequirements(user, tt.requires)
			require.NoError(t, err)

			require.Equal(t, goResult, rustResult,
				"Go and Rust implementations disagree for test case %q: Go=%v, Rust=%v",
				tt.name, goResult, rustResult)
		})
	}
}
