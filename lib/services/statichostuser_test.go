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

	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/userprovisioning"
)

func TestValidateStaticHostUser(t *testing.T) {
	t.Parallel()

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
				Matchers: []*userprovisioningpb.Matcher{
					{
						NodeLabels: []*labelv1.Label{
							{
								Name:   "foo",
								Values: []string{"bar"},
							},
						},
					},
				},
			}),
			assert: require.Error,
		},
		{
			name:     "no spec",
			hostUser: userprovisioning.NewStaticHostUser("alice_user", nil),
			assert:   require.Error,
		},
		{
			name:     "no matchers",
			hostUser: userprovisioning.NewStaticHostUser("alice", &userprovisioningpb.StaticHostUserSpec{}),
			assert:   require.Error,
		},
		{
			name: "invalid node labels",
			hostUser: userprovisioning.NewStaticHostUser("alice_user", &userprovisioningpb.StaticHostUserSpec{
				Matchers: []*userprovisioningpb.Matcher{
					{
						NodeLabels: []*labelv1.Label{
							{
								Name:   types.Wildcard,
								Values: []string{"bar"},
							},
						},
					},
				},
			}),
			assert: require.Error,
		},
		{
			name: "invalid node labels expression",
			hostUser: userprovisioning.NewStaticHostUser("alice_user", &userprovisioningpb.StaticHostUserSpec{
				Matchers: []*userprovisioningpb.Matcher{
					{
						NodeLabelsExpression: "foo bar xyz",
					},
				},
			}),
			assert: require.Error,
		},
		{
			name: "valid wildcard labels",
			hostUser: userprovisioning.NewStaticHostUser("alice_user", &userprovisioningpb.StaticHostUserSpec{
				Matchers: []*userprovisioningpb.Matcher{
					{
						NodeLabels: []*labelv1.Label{
							{
								Name:   "foo",
								Values: []string{types.Wildcard},
							},
						},
					},
					{
						NodeLabels: []*labelv1.Label{
							{
								Name:   types.Wildcard,
								Values: []string{types.Wildcard},
							},
						},
					},
				},
			}),
			assert: require.NoError,
		},
		{
			name: "ok",
			hostUser: userprovisioning.NewStaticHostUser("alice_user", &userprovisioningpb.StaticHostUserSpec{
				Matchers: []*userprovisioningpb.Matcher{
					{
						NodeLabels: []*labelv1.Label{
							{
								Name:   "foo",
								Values: []string{"bar"},
							},
						},
						Groups:               []string{"foo", "bar"},
						NodeLabelsExpression: `labels["env"] == "staging" || labels["env"] == "test"`,
						Uid:                  1234,
						Gid:                  1234,
					},
				},
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
