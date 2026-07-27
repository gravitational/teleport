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

package scopes

import (
	"testing"

	"github.com/stretchr/testify/require"

	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
)

// TestMatchScope verifies the core scope-matching logic across all modes and scope relationships,
// including the unscoped ("") resource scope and the wildcard (nil/unspecified/all) cases.
func TestMatchScope(t *testing.T) {
	t.Parallel()

	// resourceScopes exercises the full set of relationships relative to a filter scope of "/aa/bb":
	// the exact scope, an ancestor, the root ancestor, a descendant, an orthogonal scope, and unscoped.
	const filterScope = "/aa/bb"

	tests := []struct {
		name string
		mode scopesv1.Mode
		// want maps a resource scope to whether it should match.
		want map[string]bool
	}{
		{
			name: "exact",
			mode: scopesv1.Mode_MODE_EXACT,
			want: map[string]bool{
				"/aa/bb":    true,
				"/aa":       false,
				"/":         false,
				"/aa/bb/cc": false,
				"/aa/cc":    false,
				"/xx":       false,
				"":          false,
			},
		},
		{
			name: "descendants",
			mode: scopesv1.Mode_MODE_DESCENDANTS,
			want: map[string]bool{
				"/aa/bb":    true,
				"/aa":       false,
				"/":         false,
				"/aa/bb/cc": true,
				"/aa/cc":    false,
				"/xx":       false,
				"":          false,
			},
		},
		{
			name: "ancestors",
			mode: scopesv1.Mode_MODE_ANCESTORS,
			want: map[string]bool{
				"/aa/bb":    true,
				"/aa":       true,
				"/":         true,
				"/aa/bb/cc": false,
				"/aa/cc":    false,
				"/xx":       false,
				"":          false,
			},
		},
		{
			name: "relatives",
			mode: scopesv1.Mode_MODE_RELATIVES,
			want: map[string]bool{
				"/aa/bb":    true,
				"/aa":       true,
				"/":         true,
				"/aa/bb/cc": true,
				"/aa/cc":    false,
				"/xx":       false,
				"":          false,
			},
		},
		{
			name: "unscoped",
			mode: scopesv1.Mode_MODE_UNSCOPED,
			want: map[string]bool{
				"/aa/bb":    false,
				"/aa":       false,
				"/":         false,
				"/aa/bb/cc": false,
				"/aa/cc":    false,
				"/xx":       false,
				"":          true,
			},
		},
		{
			name: "all is a wildcard",
			mode: scopesv1.Mode_MODE_ALL,
			want: map[string]bool{
				"/aa/bb":    true,
				"/aa":       true,
				"/":         true,
				"/aa/bb/cc": true,
				"/aa/cc":    true,
				"/xx":       true,
				"":          true,
			},
		},
		{
			name: "unspecified is a wildcard",
			mode: scopesv1.Mode_MODE_UNSPECIFIED,
			want: map[string]bool{
				"/aa/bb":    true,
				"/aa":       true,
				"/":         true,
				"/aa/bb/cc": true,
				"/aa/cc":    true,
				"/xx":       true,
				"":          true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := scopesv1.Filter_builder{
				Scope: filterScope,
				Mode:  tt.mode,
			}.Build()

			for resourceScope, want := range tt.want {
				require.Equal(t, want, MatchScope(filter, resourceScope), "resourceScope=%q", resourceScope)
			}
		})
	}
}

// TestMatchScopeNilFilter verifies that a nil filter is treated as a wildcard match.
func TestMatchScopeNilFilter(t *testing.T) {
	t.Parallel()

	for _, resourceScope := range []string{"", "/", "/aa", "/aa/bb"} {
		require.True(t, MatchScope(nil, resourceScope), "resourceScope=%q", resourceScope)
	}
}

// TestValidateFilter verifies basic filter validation.
func TestValidateFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		filter  *scopesv1.Filter
		wantErr bool
	}{
		{
			name:   "nil filter is valid",
			filter: nil,
		},
		{
			name:   "unspecified with empty scope is valid",
			filter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_UNSPECIFIED}.Build(),
		},
		{
			name:    "unspecified with scope is invalid",
			filter:  scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_UNSPECIFIED, Scope: "/aa"}.Build(),
			wantErr: true,
		},
		{
			name:   "exact with scope is valid",
			filter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_EXACT, Scope: "/aa/bb"}.Build(),
		},
		{
			name:    "exact without scope is invalid",
			filter:  scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_EXACT}.Build(),
			wantErr: true,
		},
		{
			name:   "descendants with scope is valid",
			filter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_DESCENDANTS, Scope: "/aa"}.Build(),
		},
		{
			name:   "ancestors with scope is valid",
			filter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ANCESTORS, Scope: "/aa"}.Build(),
		},
		{
			name:   "relatives with scope is valid",
			filter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_RELATIVES, Scope: "/aa"}.Build(),
		},
		{
			name:    "relationship mode with invalid scope is invalid",
			filter:  scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_EXACT, Scope: "/aa/b@b"}.Build(),
			wantErr: true,
		},
		{
			name:   "unscoped with empty scope is valid",
			filter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_UNSCOPED}.Build(),
		},
		{
			name:    "unscoped with scope is invalid",
			filter:  scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_UNSCOPED, Scope: "/aa"}.Build(),
			wantErr: true,
		},
		{
			name:   "all with empty scope is valid",
			filter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ALL}.Build(),
		},
		{
			name:    "all with scope is invalid",
			filter:  scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ALL, Scope: "/aa"}.Build(),
			wantErr: true,
		},
		{
			name:    "unknown mode is invalid",
			filter:  scopesv1.Filter_builder{Mode: scopesv1.Mode(999)}.Build(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilter(tt.filter)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
