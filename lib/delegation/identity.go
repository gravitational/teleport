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
	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/api/types"
)

// FromUser constructs a delegation from the given user, preserving its existing
// delegation chain.
func FromUser(user User) *delegationv1.Delegation {
	d := &delegationv1.Delegation_builder{Previous: user.GetDelegation()}

	switch {
	case user.IsBot():
		name, _ := user.GetLabel(types.BotLabel)
		scope, _ := user.GetLabel(types.BotScopeLabel)

		d.Bot = delegationv1.BotDelegator_builder{
			Name:  name,
			Scope: scope,
		}.Build()

	default:
		d.User = delegationv1.UserDelegator_builder{
			Username: user.GetName(),
		}.Build()
	}

	return d.Build()
}

type User interface {
	GetDelegation() *delegationv1.Delegation
	GetLabel(string) (string, bool)
	GetName() string
	IsBot() bool
}
