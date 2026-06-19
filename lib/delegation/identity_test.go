// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package delegation_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/delegation"
	"github.com/stretchr/testify/require"
)

func TestFromUser(t *testing.T) {
	testCases := map[string]struct {
		in   types.User
		want *types.Delegation
	}{
		"user": {
			in: &types.UserV2{
				Metadata: types.Metadata{
					Name: "alice",
				},
			},
			want: &types.Delegation{
				Delegator: &types.Delegation_User{
					User: &types.UserDelegator{
						Username: "alice",
					},
				},
			},
		},
		"bot": {
			in: &types.UserV2{
				Metadata: types.Metadata{
					Name: "bot-claude",
					Labels: map[string]string{
						types.BotLabel:      "claude",
						types.BotScopeLabel: "/production",
					},
				},
			},
			want: &types.Delegation{
				Delegator: &types.Delegation_Bot{
					Bot: &types.BotDelegator{
						Name:  "claude",
						Scope: "/production",
					},
				},
			},
		},
		"chain": {
			in: &types.UserV2{
				Metadata: types.Metadata{
					Name: "bot-a",
					Labels: map[string]string{
						types.BotLabel: "a",
					},
				},
				Spec: types.UserSpecV2{
					Delegation: &types.Delegation{
						Delegator: &types.Delegation_Bot{
							Bot: &types.BotDelegator{
								Name: "b",
							},
						},
						Previous: &types.Delegation{
							Delegator: &types.Delegation_User{
								User: &types.UserDelegator{
									Username: "c",
								},
							},
						},
					},
				},
			},
			want: &types.Delegation{
				Delegator: &types.Delegation_Bot{
					Bot: &types.BotDelegator{
						Name: "a",
					},
				},
				Previous: &types.Delegation{
					Delegator: &types.Delegation_Bot{
						Bot: &types.BotDelegator{
							Name: "b",
						},
					},
					Previous: &types.Delegation{
						Delegator: &types.Delegation_User{
							User: &types.UserDelegator{
								Username: "c",
							},
						},
					},
				},
			},
		},
	}
	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			require.Empty(t, cmp.Diff(tc.want, delegation.FromUser(tc.in)))
		})
	}
}
