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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/userprovisioning"
)

func TestValidateStaticHostUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		hostUser *userprovisioning.StaticHostUser
		assert   require.ErrorAssertionFunc
	}{
		{
			name:   "nil user",
			assert: require.Error,
		},
		{
			name: "no name",
			hostUser: userprovisioning.NewStaticHostUser(&headerv1.Metadata{}, userprovisioning.Spec{
				Login: "alice",
			}),
			assert: require.Error,
		},
		{
			name:     "missing login",
			hostUser: userprovisioning.NewStaticHostUser(&headerv1.Metadata{Name: "alice_user"}, userprovisioning.Spec{}),
			assert:   require.Error,
		},
		{
			name: "invalid node labels",
			hostUser: userprovisioning.NewStaticHostUser(&headerv1.Metadata{Name: "alice_user"}, userprovisioning.Spec{
				Login:      "alice",
				NodeLabels: types.Labels{types.Wildcard: {"bar"}},
			}),
			assert: require.Error,
		},
		{
			name: "invalid node labels expression",
			hostUser: userprovisioning.NewStaticHostUser(&headerv1.Metadata{Name: "alice_user"}, userprovisioning.Spec{
				Login:                "alice",
				NodeLabelsExpression: "foo bar xyz",
			}),
			assert: require.Error,
		},
		{
			name: "valid wildcard labels",
			hostUser: userprovisioning.NewStaticHostUser(&headerv1.Metadata{Name: "alice_user"}, userprovisioning.Spec{
				Login:      "alice",
				NodeLabels: types.Labels{"foo": {types.Wildcard}, types.Wildcard: {types.Wildcard}},
			}),
			assert: require.NoError,
		},
		{
			name: "non-numeric uid",
			hostUser: userprovisioning.NewStaticHostUser(&headerv1.Metadata{Name: "alice_user"}, userprovisioning.Spec{
				Login:      "alice",
				Groups:     []string{"foo", "bar"},
				Uid:        "abcd",
				Gid:        "1234",
				NodeLabels: types.Labels{"foo": {"bar"}},
			}),
			assert: require.Error,
		},
		{
			name: "non-numeric gid",
			hostUser: userprovisioning.NewStaticHostUser(&headerv1.Metadata{Name: "alice_user"}, userprovisioning.Spec{
				Login:      "alice",
				Groups:     []string{"foo", "bar"},
				Uid:        "1234",
				Gid:        "abcd",
				NodeLabels: types.Labels{"foo": {"bar"}},
			}),
			assert: require.Error,
		},
		{
			name: "ok",
			hostUser: userprovisioning.NewStaticHostUser(&headerv1.Metadata{Name: "alice_user"}, userprovisioning.Spec{
				Login:                "alice",
				Groups:               []string{"foo", "bar"},
				Uid:                  "1234",
				Gid:                  "5678",
				NodeLabels:           types.Labels{"foo": {"bar"}},
				NodeLabelsExpression: `labels["env"] == "staging" || labels["env"] == "test"`,
			}),
			assert: require.NoError,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.assert(t, ValidateStaticHostUser(tc.hostUser))
		})
	}
}
