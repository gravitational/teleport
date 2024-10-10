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
	"github.com/gravitational/teleport/lib/backend"
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

func NewProvisioningStateService(backendInstance backend.Backend, mode ProvisioningStateServiceMode) (*ProvisioningStateService, error) {
	userStatusSvc, err := generic.NewServiceWrapper(generic.ServiceWrapperConfig[*provisioningv1.PrincipalState]{
		Backend:       backendInstance,
		ResourceKind:  types.KindProvisioningState,
		BackendPrefix: backend.NewKey(provisioningStatePrefix),
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

func (ss *ProvisioningStateService) CreateProvisioningState(ctx context.Context, downstreamID services.DownstreamID, state *provisioningv1.PrincipalState) (*provisioningv1.PrincipalState, error) {
	createdState, err := ss.service.WithPrefix(string(downstreamID)).CreateResource(ctx, state)
	if err != nil {
		return nil, trace.Wrap(err, "creating provisioning state record")
	}
	return createdState, nil
}

func (ss *ProvisioningStateService) UpdateProvisioningState(ctx context.Context, downstreamID services.DownstreamID, state *provisioningv1.PrincipalState) (*provisioningv1.PrincipalState, error) {
	scopedService := ss.service.WithPrefix(string(downstreamID))

	var updateFn func(context.Context, *provisioningv1.PrincipalState) (*provisioningv1.PrincipalState, error)
	switch ss.mode {
	case ProvisioningStateServiceModeStrict:
		updateFn = scopedService.ConditionalUpdateResource

	case ProvisioningStateServiceModeRelaxed:
		updateFn = scopedService.CreateResource

	default:
		return nil, trace.BadParameter("invalid service mode: %v", ss.mode)
	}

	updatedState, err := updateFn(ctx, state)
	if err != nil {
		return nil, trace.Wrap(err, "updating provisioning state record")
	}
	return updatedState, nil
}

func (ss *ProvisioningStateService) GetProvisioningState(ctx context.Context, downstreamID services.DownstreamID, name services.ProvisioningStateID) (*provisioningv1.PrincipalState, error) {
	state, err := ss.service.WithPrefix(string(downstreamID)).GetResource(ctx, string(name))
	if err != nil {
		return nil, trace.Wrap(err, "fetching provisioning state record")
	}
	return state, nil
}

func (ss *ProvisioningStateService) ListProvisioningStates(ctx context.Context, downstreamID services.DownstreamID, page *pagination.PageRequestToken) ([]*provisioningv1.PrincipalState, pagination.NextPageToken, error) {
	token, err := page.Consume()
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	resp, nextPage, err := ss.service.WithPrefix(string(downstreamID)).ListResources(ctx, provisioningStatePageSize, token)
	if err != nil {
		return nil, "", trace.Wrap(err, "listing provisioning state records")
	}
	return resp, pagination.NextPageToken(nextPage), nil
}

func (ss *ProvisioningStateService) DeleteProvisioningState(ctx context.Context, downstreamID services.DownstreamID, name services.ProvisioningStateID) error {
	return trace.Wrap(ss.service.WithPrefix(string(downstreamID)).DeleteResource(ctx, string(name)))
}

func (ss *ProvisioningStateService) DeleteAllProvisioningStates(ctx context.Context, downstreamID services.DownstreamID) error {
	return trace.Wrap(ss.service.WithPrefix(string(downstreamID)).DeleteAllResources(ctx))
}
