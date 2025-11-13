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

package services

import (
	"context"

	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
)

// DelegationSessions is an interface over the DelegationSessions service. This
// interface may also be implemented by a client to allow remote and local
// consumers to access the resource in a similar way.
type DelegationSessions interface {
	// CreateDelegationSession creates a new delegation session.
	CreateDelegationSession(ctx context.Context, session *delegationv1.DelegationSession) (*delegationv1.DelegationSession, error)

	// GetDelegationSession reads a delegation session using its ID.
	GetDelegationSession(ctx context.Context, id string) (*delegationv1.DelegationSession, error)

	// DeleteDelegationSession deletes a delegation session using its ID.
	DeleteDelegationSession(ctx context.Context, id string) error
}
