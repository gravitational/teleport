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
	"github.com/gravitational/teleport/api/types/events"
)

// MaxChainDepth is the maximum number of links in a delegation chain.
const MaxChainDepth = 100

// EventsFrom flattens a recursive delegation linked-list into a
// repeated list of DelegationChainEntry for use in audit events. The returned
// slice is ordered from most recent delegator (index 0) to original delegator
// (last element). Returns nil if delegation is nil.
func EventsFrom(d *delegationv1.Delegation) []events.DelegationChainEntry {
	if d == nil {
		return nil
	}

	var chain []events.DelegationChainEntry
	for current := d; current != nil; current = current.GetPrevious() {
		if len(chain) >= MaxChainDepth {
			break
		}
		var entry events.DelegationChainEntry
		switch {
		case current.GetUser() != nil:
			entry.Username = current.GetUser().GetUsername()
		case current.GetBot() != nil:
			entry.BotName = current.GetBot().GetName()
			entry.BotScope = current.GetBot().GetScope()
		}
		chain = append(chain, entry)
	}
	return chain
}
