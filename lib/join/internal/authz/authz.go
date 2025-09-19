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

package authz

import (
	"github.com/gravitational/teleport/api/types"
)

// Context represents a distilled authenticated context for a join request.
type Context struct {
	// IsForwardedByProxy is true if the join request was actively forwarded by
	// a proxy. As in, the proxy terminated the gRPC request, added
	// proxy-supplied parameters, and forwarded it to the auth service.
	IsForwardedByProxy bool
	// IsInstance is true if the client authenticated as the Instance system role.
	IsInstance bool
	// IsBot is true if the client authenticated as the Bot system role.
	IsBot bool
	// SystemRoles is a list of additional system roles that an Instance
	// authenticated as having.
	SystemRoles types.SystemRoles
	// HostID is an authenticated HostID.
	HostID string
	// BotGeneration is the current generation of an authenticated Bot.
	BotGeneration uint64
	// BotInstanceID is an authenticated Bot ID.
	BotInstanceID string
}
