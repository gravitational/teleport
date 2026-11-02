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
	"github.com/gravitational/trace"

	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/api/scopes"
)

// MaxChainLength is the maximum number of links allowed in a delegation chain.
//
// This limit was chosen to allow us to include the entire chain in audit events
// without trimming, which have a maximum total size of 64KiB. We also use it to
// avoid generating enormous X.509 and SSH certificates.
const MaxChainLength = 8

// Validate the given delegation chain.
func Validate(d *delegationv1.Delegation) error {
	if len(d.GetChain()) > MaxChainLength {
		return trace.LimitExceeded("chain cannot contain more than %d links", MaxChainLength)
	}
	for i, l := range d.GetChain() {
		if err := ValidateChainLink(l); err != nil {
			return trace.Wrap(err, "validating chain[%d]", i)
		}
	}
	return nil
}

// ValidateChainLink validates the given delegation chain link.
func ValidateChainLink(link *delegationv1.DelegationChainLink) error {
	if link == nil {
		return trace.BadParameter("must not be nil")
	}

	switch link.WhichIdentity() {
	case delegationv1.DelegationChainLink_Identity_not_set_case:
		return trace.BadParameter("user or bot is required")
	case delegationv1.DelegationChainLink_User_case:
		if link.GetUser().GetName() == "" {
			return trace.BadParameter("user.name is required")
		}
	case delegationv1.DelegationChainLink_Bot_case:
		bot := link.GetBot()
		if bot.GetName() == "" {
			return trace.BadParameter("bot.name is required")
		}
		if bot.HasScope() {
			if err := scopes.StrongValidate(bot.GetScope()); err != nil {
				return trace.Wrap(err)
			}
		}
	default:
		return trace.BadParameter("unknown identity type: %s", link.WhichIdentity())
	}
	return nil
}
