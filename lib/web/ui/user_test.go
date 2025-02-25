/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package ui

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestNewUserListEntry(t *testing.T) {
	tests := []struct {
		name string
		user types.User
		want *UserListEntry
	}{
		{
			name: "bot",
			user: &types.UserV2{
				Metadata: types.Metadata{
					Name: "bot-bernard",
					Labels: map[string]string{
						types.BotLabel: "true",
					},
				},
				Spec: types.UserSpecV2{
					Roles: []string{"behavioral-analyst"},
					Traits: map[string][]string{
						"logins": {"arnold"},
					},
				},
			},
			want: &UserListEntry{
				Name:     "bot-bernard",
				Roles:    []string{"behavioral-analyst"},
				AuthType: "local",
				IsBot:    true,
				AllTraits: map[string][]string{
					"logins": {"arnold"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewUserListEntry(tt.user)
			require.NoError(t, err)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("NewUserListEntry() mismatch (-want +got):\n%s", diff)
			}
		})
	}

}
