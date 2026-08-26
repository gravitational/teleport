/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"github.com/gravitational/teleport/lib/ui"
)

func TestMakeUserGroups(t *testing.T) {
	tests := []struct {
		name             string
		userGroups       types.UserGroups
		userGroupsToApps map[string]types.Apps
		expected         []UserGroup
	}{
		{
			name:     "empty",
			expected: []UserGroup{},
		},
		{
			name: "user groups with no apps",
			userGroups: types.UserGroups{
				newGroup(t, "group1", "group1 desc", map[string]string{"label1": "value1"}),
				newGroup(t, "group2", "group2 desc", map[string]string{"label2": "value2", types.OriginLabel: types.OriginOkta}),
			},
			expected: []UserGroup{
				{
					Name:         "group1",
					Description:  "group1 desc",
					Labels:       []ui.Label{{Name: "label1", Value: "value1"}},
					Applications: []ApplicationAndFriendlyName{},
				},
				{
					Name:         "group2",
					Description:  "group2 desc",
					FriendlyName: "group2 desc",
					Labels: []ui.Label{
						{Name: "label2", Value: "value2"},
						{Name: types.OriginLabel, Value: types.OriginOkta},
					},
					Applications: []ApplicationAndFriendlyName{},
				},
			},
		},
		{
			name: "user groups with apps",
			userGroups: types.UserGroups{
				newGroup(t, "group1", "group1 desc", map[string]string{"label1": "value1"}),
				newGroup(t, "group2", "group2 desc", map[string]string{"label2": "value2", types.OriginLabel: types.OriginOkta}),
			},
			userGroupsToApps: map[string]types.Apps{
				"group1": {
					newApp(t, "1", "1.com", "1 desc", nil),
					newApp(t, "2", "2.com", "2 desc", map[string]string{types.OriginLabel: types.OriginOkta}),
				},
				"group2": {
					newApp(t, "2", "2.com", "2 desc", map[string]string{types.OriginLabel: types.OriginOkta}),
					newApp(t, "3", "3.com", "3 desc", nil),
				},
				// This should be ignored
				"group3": {
					newApp(t, "3", "3.com", "3 desc", nil),
				},
			},
			expected: []UserGroup{
				{
					Name:        "group1",
					Description: "group1 desc",
					Labels:      []ui.Label{{Name: "label1", Value: "value1"}},
					Applications: []ApplicationAndFriendlyName{
						{Name: "1"},
						{Name: "2", FriendlyName: "2 desc"},
					},
				},
				{
					Name:         "group2",
					Description:  "group2 desc",
					FriendlyName: "group2 desc",
					Labels: []ui.Label{
						{Name: "label2", Value: "value2"},
						{Name: types.OriginLabel, Value: types.OriginOkta},
					},
					Applications: []ApplicationAndFriendlyName{
						{Name: "2", FriendlyName: "2 desc"},
						{Name: "3"},
					},
				},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			userGroups, err := MakeUserGroups(test.userGroups, test.userGroupsToApps)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(test.expected, userGroups))
		})
	}
}

func newGroup(t *testing.T, name, description string, labels map[string]string) types.UserGroup {
	userGroup, err := types.NewUserGroup(types.Metadata{
		Name:        name,
		Description: description,
		Labels:      labels,
	}, types.UserGroupSpecV1{})
	require.NoError(t, err)
	return userGroup
}
