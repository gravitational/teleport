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

package common

import (
	"testing"

	"github.com/stretchr/testify/require"

	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/lib/client"
)

func TestResolveDesiredScope(t *testing.T) {
	tests := []struct {
		name            string
		cf              *CLIConf
		profile         *client.ProfileStatus
		wantScope       string
		wantScopeChange bool
	}{
		{
			name:            "no flag, no profile -> empty scope, no change",
			cf:              &CLIConf{},
			profile:         nil,
			wantScope:       "",
			wantScopeChange: false,
		},
		{
			name:            "no flag, unscoped profile -> inherit empty, no change",
			cf:              &CLIConf{},
			profile:         &client.ProfileStatus{},
			wantScope:       "",
			wantScopeChange: false,
		},
		{
			name:            "no flag, scoped profile -> inherit scope, no change",
			cf:              &CLIConf{},
			profile:         &client.ProfileStatus{ScopePin: &scopesv1.Pin{Scope: "/staging/west"}},
			wantScope:       "/staging/west",
			wantScopeChange: false,
		},
		{
			name: "explicit scope, no profile -> set scope, changed",
			cf: &CLIConf{
				Scope:          "/staging/east",
				ScopeSetByUser: true,
			},
			profile:         nil,
			wantScope:       "/staging/east",
			wantScopeChange: true,
		},
		{
			name: "explicit scope, unscoped profile -> set scope, changed",
			cf: &CLIConf{
				Scope:          "/staging/east",
				ScopeSetByUser: true,
			},
			profile:         &client.ProfileStatus{},
			wantScope:       "/staging/east",
			wantScopeChange: true,
		},
		{
			name: "explicit scope, same scoped profile -> same scope, no change",
			cf: &CLIConf{
				Scope:          "/staging/east",
				ScopeSetByUser: true,
			},
			profile:         &client.ProfileStatus{ScopePin: &scopesv1.Pin{Scope: "/staging/east"}},
			wantScope:       "/staging/east",
			wantScopeChange: false,
		},
		{
			name: "explicit scope, different scoped profile -> new scope, changed",
			cf: &CLIConf{
				Scope:          "/staging/east",
				ScopeSetByUser: true,
			},
			profile:         &client.ProfileStatus{ScopePin: &scopesv1.Pin{Scope: "/staging/west"}},
			wantScope:       "/staging/east",
			wantScopeChange: true,
		},
		{
			name: "descope with empty string, scoped profile -> empty, changed",
			cf: &CLIConf{
				Scope:          "",
				ScopeSetByUser: true,
			},
			profile:         &client.ProfileStatus{ScopePin: &scopesv1.Pin{Scope: "/staging/west"}},
			wantScope:       "",
			wantScopeChange: true,
		},
		{
			name: "descope with empty string, unscoped profile -> empty, no change",
			cf: &CLIConf{
				Scope:          "",
				ScopeSetByUser: true,
			},
			profile:         &client.ProfileStatus{ScopePin: &scopesv1.Pin{Scope: ""}},
			wantScope:       "",
			wantScopeChange: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotScope, gotChanged := resolveScope(tt.cf, tt.profile)
			require.Equal(t, tt.wantScope, gotScope)
			require.Equal(t, tt.wantScopeChange, gotChanged)
		})
	}
}
