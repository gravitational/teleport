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

	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/userprovisioning"
	"github.com/gravitational/teleport/api/types/wrappers"
)

func TestValidateStaticHostUser(t *testing.T) {
	t.Parallel()

	nodeLabels := func(labels map[string]string) *wrappers.LabelValues {
		if len(labels) == 0 {
			return nil
		}
		values := &wrappers.LabelValues{
			Values: make(map[string]wrappers.StringValues, len(labels)),
		}
		for k, v := range labels {
			values.Values[k] = wrappers.StringValues{
				Values: []string{v},
			}
		}
		return values
	}

	tests := []struct {
		name     string
		hostUser *userprovisioningpb.StaticHostUser
		assert   require.ErrorAssertionFunc
	}{
		{
			name:   "nil user",
			assert: require.Error,
		},
		{
			name: "no name",
			hostUser: userprovisioning.NewStaticHostUser("", &userprovisioningpb.StaticHostUserSpec{
				Login: "alice",
			}),
			assert: require.Error,
		},
		{
			name:     "no spec",
			hostUser: userprovisioning.NewStaticHostUser("alice_user", nil),
			assert:   require.Error,
		},
		{
			name:     "missing login",
			hostUser: userprovisioning.NewStaticHostUser("alice_user", &userprovisioningpb.StaticHostUserSpec{}),
			assert:   require.Error,
		},
		{
			name: "invalid node labels",
			hostUser: userprovisioning.NewStaticHostUser("alice_user", &userprovisioningpb.StaticHostUserSpec{
				Login:      "alice",
				NodeLabels: nodeLabels(map[string]string{types.Wildcard: "bar"}),
			}),
			assert: require.Error,
		},
		{
			name: "invalid node labels expression",
			hostUser: userprovisioning.NewStaticHostUser("alice_user", &userprovisioningpb.StaticHostUserSpec{
				Login:                "alice",
				NodeLabelsExpression: "foo bar xyz",
			}),
			assert: require.Error,
		},
		{
			name: "valid wildcard labels",
			hostUser: userprovisioning.NewStaticHostUser("alice_user", &userprovisioningpb.StaticHostUserSpec{
				Login: "alice",
				NodeLabels: nodeLabels(map[string]string{
					"foo":          types.Wildcard,
					types.Wildcard: types.Wildcard,
				}),
			}),
			assert: require.NoError,
		},
		{
			name: "non-numeric uid",
			hostUser: userprovisioning.NewStaticHostUser("alice_user", &userprovisioningpb.StaticHostUserSpec{
				Login:      "alice",
				Groups:     []string{"foo", "bar"},
				Uid:        "abcd",
				Gid:        "1234",
				NodeLabels: nodeLabels(map[string]string{"foo": "bar"}),
			}),
			assert: require.Error,
		},
		{
			name: "non-numeric gid",
			hostUser: userprovisioning.NewStaticHostUser("alice_user", &userprovisioningpb.StaticHostUserSpec{
				Login:      "alice",
				Groups:     []string{"foo", "bar"},
				Uid:        "1234",
				Gid:        "abcd",
				NodeLabels: nodeLabels(map[string]string{"foo": "bar"}),
			}),
			assert: require.Error,
		},
		{
			name: "ok",
			hostUser: userprovisioning.NewStaticHostUser("alice_user", &userprovisioningpb.StaticHostUserSpec{
				Login:                "alice",
				Groups:               []string{"foo", "bar"},
				Uid:                  "1234",
				Gid:                  "5678",
				NodeLabels:           nodeLabels(map[string]string{"foo": "bar"}),
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
