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

package services

import (
	"github.com/gravitational/trace"

	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/api/scopes"
)

// DelegatorUsername returns the Teleport username of the identity in the given
// delegation chain link.
func DelegatorUsername(link *delegationv1.DelegationChainLink) (string, error) {
	if user := link.GetUser(); user != nil {
		return user.GetName(), nil
	}
	if bot := link.GetBot(); bot != nil {
		name, err := BotResourceName(scopes.QualifiedName{
			Name:  bot.GetName(),
			Scope: bot.GetScope(),
		})
		return name, trace.Wrap(err)
	}
	return "", trace.BadParameter("unknown identity type: %s", link.WhichIdentity())
}
