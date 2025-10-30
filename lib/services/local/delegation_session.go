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

package local

import (
	"context"

	"github.com/gravitational/trace"

	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const delegationSessionPrefix = "delegation_session"

// DelegationSessionService exposes backend functionality for storing
// DelegationSession resources.
type DelegationSessionService struct {
	service *generic.ServiceWrapper[*delegationv1.DelegationSession]
}

var _ services.DelegationSessions = (*DelegationSessionService)(nil)

// NewDelegationSessionService creates a new DelegationSessionService.
func NewDelegationSessionService(b backend.Backend) (*DelegationSessionService, error) {
	service, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*delegationv1.DelegationSession]{
			Backend:       b,
			ResourceKind:  types.KindDelegationSession,
			BackendPrefix: backend.NewKey(delegationSessionPrefix),
			MarshalFunc:   services.MarshalProtoResource[*delegationv1.DelegationSession],
			UnmarshalFunc: services.UnmarshalProtoResource[*delegationv1.DelegationSession],
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &DelegationSessionService{
		service: service,
	}, nil
}

// CreateDelegationSession creates a new delegation session.
func (s *DelegationSessionService) CreateDelegationSession(
	ctx context.Context,
	session *delegationv1.DelegationSession,
) (*delegationv1.DelegationSession, error) {
	session, err := s.service.CreateResource(ctx, session)
	return session, trace.Wrap(err)
}

// GetDelegationSession reads a delegation session using its ID.
func (s *DelegationSessionService) GetDelegationSession(
	ctx context.Context,
	id string,
) (*delegationv1.DelegationSession, error) {
	session, err := s.service.GetResource(ctx, id)
	return session, trace.Wrap(err)
}

// DeleteDelegationSession deletes a delegation session using its ID.
func (s *DelegationSessionService) DeleteDelegationSession(ctx context.Context, id string) error {
	return trace.Wrap(s.service.DeleteResource(ctx, id))
}
