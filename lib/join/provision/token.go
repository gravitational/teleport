// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package provision

import (
	"time"

	"github.com/gravitational/teleport/api/types"
)

// A Token is used in the join service to facilitate provisioning.
type Token interface {
	// GetName returns the name of the token.
	GetName() string
	// GetSafeName returns the name of the token, sanitized appropriately for
	// join methods where the name is secret. This should be used when logging
	// the token name.
	GetSafeName() string
	// GetJoinMethod returns joining method that must be used with this token.
	GetJoinMethod() types.JoinMethod
	// GetRoles returns a list of teleport roles that will be granted to the
	// resources provisioned with this token.
	GetRoles() types.SystemRoles
	// Expiry returns the token's expiration time.
	Expiry() time.Time
	// GetBotName returns the BotName field which must be set for joining bots.
	GetBotName() string
	// GetAssignedScope returns the scope that will be assigned to provisioned resources
	// provisioned using the wrapped [joiningv1.ScopedToken].
	GetAssignedScope() string
	// GetAllowRules returns the list of allow rules.
	GetAllowRules() []*types.TokenRule
	// GetAWSIIDTTL returns the TTL of EC2 IIDs
	GetAWSIIDTTL() types.Duration
}
