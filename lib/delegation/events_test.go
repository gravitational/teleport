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

	"github.com/stretchr/testify/require"

	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/delegation"
)

func TestToEventsDelegationChain(t *testing.T) {
	tests := []struct {
		name       string
		delegation *delegationv1.Delegation
		want       []events.DelegationChainEntry
	}{
		{
			name:       "nil delegation",
			delegation: nil,
			want:       nil,
		},
		{
			name: "single user delegator",
			delegation: delegationv1.Delegation_builder{
				User: delegationv1.UserDelegator_builder{
					Username: "alice",
				}.Build(),
			}.Build(),
			want: []events.DelegationChainEntry{
				{Username: "alice"},
			},
		},
		{
			name: "single bot delegator",
			delegation: delegationv1.Delegation_builder{
				Bot: delegationv1.BotDelegator_builder{
					Name:  "deploy-bot",
					Scope: "/prod",
				}.Build(),
			}.Build(),
			want: []events.DelegationChainEntry{
				{BotName: "deploy-bot", BotScope: "/prod"},
			},
		},
		{
			name: "bot with no scope",
			delegation: delegationv1.Delegation_builder{
				Bot: delegationv1.BotDelegator_builder{
					Name: "simple-bot",
				}.Build(),
			}.Build(),
			want: []events.DelegationChainEntry{
				{BotName: "simple-bot"},
			},
		},
		{
			name: "chain: bot -> bot -> user",
			delegation: delegationv1.Delegation_builder{
				Bot: delegationv1.BotDelegator_builder{
					Name:  "inner-bot",
					Scope: "/staging",
				}.Build(),
				Previous: delegationv1.Delegation_builder{
					Bot: delegationv1.BotDelegator_builder{
						Name:  "outer-bot",
						Scope: "/dev",
					}.Build(),
					Previous: delegationv1.Delegation_builder{
						User: delegationv1.UserDelegator_builder{
							Username: "bob",
						}.Build(),
					}.Build(),
				}.Build(),
			}.Build(),
			want: []events.DelegationChainEntry{
				{BotName: "inner-bot", BotScope: "/staging"},
				{BotName: "outer-bot", BotScope: "/dev"},
				{Username: "bob"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := delegation.EventsFrom(tt.delegation)
			require.Equal(t, tt.want, got)
		})
	}
}
