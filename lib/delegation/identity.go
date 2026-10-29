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

package delegation

import (
	"slices"

	"github.com/gravitational/trace"

	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/api/types"
)

// UserChain returns the given user's delegation chain (if it is part of one) or
// creates a new chain with the user at the root.
func UserChain(user User) *delegationv1.Delegation {
	if user.GetDelegation() != nil {
		return user.GetDelegation()
	}
	return delegationv1.Delegation_builder{
		Chain: []*delegationv1.DelegationChainLink{
			ChainLinkFromUser(user),
		},
	}.Build()
}

// BuildChain adds the delegate to the delegator's chain (if it is part of one)
// or creates a new chain with the delegator at the root.
func BuildChain(delegator, delegate User) (*delegationv1.Delegation, error) {
	if len(delegate.GetDelegation().GetChain()) != 0 {
		return nil, trace.BadParameter("user is already part of a delegation chain")
	}

	chain := slices.Clone(UserChain(delegator).GetChain())
	chain = append(chain, ChainLinkFromUser(delegate))

	delegation := delegationv1.Delegation_builder{Chain: chain}.Build()
	if err := Validate(delegation); err != nil {
		return nil, trace.Wrap(err)
	}
	return delegation, nil
}

// ChainLinkFromUser constructs a delegation chain link for the given user.
func ChainLinkFromUser(user User) *delegationv1.DelegationChainLink {
	var link delegationv1.DelegationChainLink_builder

	switch {
	case user.IsBot():
		name, _ := user.GetLabel(types.BotLabel)
		bot := delegationv1.BotDelegator_builder{
			Name: name,
		}
		if scope, ok := user.GetLabel(types.BotScopeLabel); ok {
			bot.Scope = &scope
		}
		if beam := beamInfoFromUser(user); beam != nil {
			bot.Beam = beam
		}
		link.Bot = bot.Build()

	default:
		link.User = delegationv1.UserDelegator_builder{
			Name: user.GetName(),
		}.Build()
	}

	return link.Build()
}

func beamInfoFromUser(user User) *delegationv1.BeamInfo {
	id, hasID := user.GetLabel(types.BeamIDLabel)
	alias, hasAlias := user.GetLabel(types.BeamAliasLabel)
	if !hasID && !hasAlias {
		return nil
	}
	return delegationv1.BeamInfo_builder{
		Id:    id,
		Alias: alias,
	}.Build()
}

type User interface {
	GetDelegation() *delegationv1.Delegation
	GetLabel(string) (string, bool)
	GetName() string
	IsBot() bool
}
