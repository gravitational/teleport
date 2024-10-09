// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	libbackend "github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/teleport/lib/utils/pagination"
	"github.com/gravitational/trace"
)

const (
	provisioningStatePrefix   = "provisioning_state"
	provisioningStatePageSize = 100
)

type ProvisioningStateServiceMode int

const (
	// ProvisioningStateServiceModeStrict is the default service mode, with
	// strict validation enabled.
	ProvisioningStateServiceModeStrict ProvisioningStateServiceMode = 0

	// ProvisioningStateServiceModeRelaxed indicates that the service should do
	// no validation and just write to the provided backend. This is generally
	// for use with caches
	ProvisioningStateServiceModeRelaxed ProvisioningStateServiceMode = 1
)

// ProvisioningStateService handles low-level CRUD operations for the provisioning status
type ProvisioningStateService struct {
	service *generic.ServiceWrapper[*provisioningv1.PrincipalState]
	mode    ProvisioningStateServiceMode
}

var _ services.ProvisioningStates = (*ProvisioningStateService)(nil)

func NewProvisioningStateService(backend libbackend.Backend, mode ProvisioningStateServiceMode) (*ProvisioningStateService, error) {
	userStatusSvc, err := generic.NewServiceWrapper(generic.ServiceWrapperConfig[*provisioningv1.PrincipalState]{
		Backend:       backend,
		ResourceKind:  types.KindProvisioningState,
		BackendPrefix: libbackend.NewKey(provisioningStatePrefix),
		MarshalFunc:   services.MarshalProtoResource[*provisioningv1.PrincipalState],
		UnmarshalFunc: services.UnmarshalProtoResource[*provisioningv1.PrincipalState],
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	svc := &ProvisioningStateService{
		mode:    mode,
		service: userStatusSvc,
	}

	return svc, nil
}

func (ss *ProvisioningStateService) CreateProvisioningState(ctx context.Context, state *provisioningv1.PrincipalState) (*provisioningv1.PrincipalState, error) {
	createdState, err := ss.service.CreateResource(ctx, state)
	if err != nil {
		return nil, trace.Wrap(err, "creating provisioning state record")
	}
	return createdState, nil
}

func (ss *ProvisioningStateService) UpdateProvisioningState(ctx context.Context, state *provisioningv1.PrincipalState) (*provisioningv1.PrincipalState, error) {
	var updatedState *provisioningv1.PrincipalState
	var err error

	switch ss.mode {
	case ProvisioningStateServiceModeStrict:
		updatedState, err = ss.service.ConditionalUpdateResource(ctx, state)

	case ProvisioningStateServiceModeRelaxed:
		updatedState, err = ss.service.UpdateResource(ctx, state)

	default:
		return nil, trace.BadParameter("invalid service mode: %v", ss.mode)
	}

	if err != nil {
		return nil, trace.Wrap(err, "updating provisioning state record")
	}
	return updatedState, nil
}

func (ss *ProvisioningStateService) GetProvisioningState(ctx context.Context, name services.ProvisioningStateID) (*provisioningv1.PrincipalState, error) {
	state, err := ss.service.GetResource(ctx, string(name))
	if err != nil {
		return nil, trace.Wrap(err, "fetching provisioning state record")
	}
	return state, nil
}

func (ss *ProvisioningStateService) ListProvisioningStates(ctx context.Context, page pagination.PageRequestToken) ([]*provisioningv1.PrincipalState, pagination.NextPageToken, error) {
	resp, nextPage, err := ss.service.ListResources(ctx, provisioningStatePageSize, string(page))
	if err != nil {
		return nil, "", trace.Wrap(err, "listing provisioning state records")
	}
	return resp, pagination.NextPageToken(nextPage), nil
}

func (ss *ProvisioningStateService) DeleteProvisioningState(ctx context.Context, name services.ProvisioningStateID) error {
	return trace.Wrap(ss.service.DeleteResource(ctx, string(name)))
}

func (ss *ProvisioningStateService) DeleteAllProvisioningStates(ctx context.Context) error {
	return trace.Wrap(ss.service.DeleteAllResources(ctx))
}
