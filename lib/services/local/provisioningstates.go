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

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/teleport/lib/utils/pagination"
)

const (
	provisioningStatePrefix   = "provisioning_principal_state"
	provisioningStatePageSize = 100
)

// ProvisioningStateService handles low-level CRUD operations for the provisioning status
type ProvisioningStateService struct {
	provisioningv1.UnimplementedProvisioningServiceServer
	service *generic.ServiceWrapper[*provisioningv1.PrincipalState]
}

var _ services.ProvisioningStates = (*ProvisioningStateService)(nil)

// NewProvisioningStateService creates a new ProvisioningStateService backed by
// the supplied backend.
func NewProvisioningStateService(backendInstance backend.Backend) (*ProvisioningStateService, error) {
	userStatusSvc, err := generic.NewServiceWrapper(generic.ServiceWrapperConfig[*provisioningv1.PrincipalState]{
		Backend:       backendInstance,
		ResourceKind:  types.KindProvisioningPrincipalState,
		BackendPrefix: backend.NewKey(provisioningStatePrefix),
		MarshalFunc:   services.MarshalProtoResource[*provisioningv1.PrincipalState],
		UnmarshalFunc: services.UnmarshalProtoResource[*provisioningv1.PrincipalState],
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	svc := &ProvisioningStateService{
		service: userStatusSvc,
	}

	return svc, nil
}

func validatePrincipalState(state *provisioningv1.PrincipalState) error {
	if state.GetSpec().GetDownstreamId() == "" {
		return trace.BadParameter("principal state missing downstream id")
	}

	return nil
}

// CreateProvisioningState creates a new backend PrincipalState record bound to
// the supplied downstream.
func (ss *ProvisioningStateService) CreateProvisioningState(ctx context.Context, state *provisioningv1.PrincipalState) (*provisioningv1.PrincipalState, error) {
	if err := validatePrincipalState(state); err != nil {
		return nil, trace.Wrap(err, "creating provisioning state record")
	}

	createdState, err := ss.service.WithPrefix(state.Spec.DownstreamId).CreateResource(ctx, state)
	if err != nil {
		return nil, trace.Wrap(err, "creating provisioning state record")
	}
	return createdState, nil
}

// UpdateProvisioningState performs a conditional update of the supplied
// provisioning state
func (ss *ProvisioningStateService) UpdateProvisioningState(ctx context.Context, state *provisioningv1.PrincipalState) (*provisioningv1.PrincipalState, error) {
	if err := validatePrincipalState(state); err != nil {
		return nil, trace.Wrap(err, "updating provisioning state record")
	}

	updatedState, err := ss.service.WithPrefix(state.Spec.DownstreamId).ConditionalUpdateResource(ctx, state)
	if err != nil {
		return nil, trace.Wrap(err, "updating provisioning state record")
	}
	return updatedState, nil
}

// UpsertProvisioningState performs an unconditional update of the supplied
// provisioning state
func (ss *ProvisioningStateService) UpsertProvisioningState(ctx context.Context, state *provisioningv1.PrincipalState) (*provisioningv1.PrincipalState, error) {
	if err := validatePrincipalState(state); err != nil {
		return nil, trace.Wrap(err, "upserting provisioning state record")
	}

	updatedState, err := ss.service.WithPrefix(state.Spec.DownstreamId).UpsertResource(ctx, state)
	if err != nil {
		return nil, trace.Wrap(err, "upserting provisioning state record")
	}
	return updatedState, nil
}

// GetProvisioningState fetches a provisioning state record from the supplied
// downstream
func (ss *ProvisioningStateService) GetProvisioningState(ctx context.Context, downstreamID services.DownstreamID, name services.ProvisioningStateID) (*provisioningv1.PrincipalState, error) {
	state, err := ss.service.WithPrefix(string(downstreamID)).GetResource(ctx, string(name))
	if err != nil {
		return nil, trace.Wrap(err, "fetching provisioning state record")
	}
	return state, nil
}

// GetProvisioningState fetches a provisioning state record from the supplied
// downstream
func (ss *ProvisioningStateService) ListProvisioningStates(ctx context.Context, downstreamID services.DownstreamID, pageSize int, page *pagination.PageRequestToken) ([]*provisioningv1.PrincipalState, pagination.NextPageToken, error) {
	if pageSize == 0 {
		pageSize = provisioningStatePageSize
	}

	token, err := page.Consume()
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	resp, nextPage, err := ss.service.WithPrefix(string(downstreamID)).ListResources(ctx, pageSize, token)
	if err != nil {
		return nil, "", trace.Wrap(err, "listing provisioning state records")
	}
	return resp, pagination.NextPageToken(nextPage), nil
}

// ListProvisioningStatesForAllDownstreams lists all provisioning state records for all
// downstream receivers. Note that the returned record names may not be unique
// across all downstream receivers. Check the records' `DownstreamID` field
// to disambiguate.
func (ss *ProvisioningStateService) ListProvisioningStatesForAllDownstreams(
	ctx context.Context,
	pageSize int,
	page *pagination.PageRequestToken,
) ([]*provisioningv1.PrincipalState, pagination.NextPageToken, error) {
	if pageSize == 0 {
		pageSize = provisioningStatePageSize
	}

	token, err := page.Consume()
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	resp, nextPage, err := ss.service.ListResources(ctx, pageSize, token)
	if err != nil {
		return nil, "", trace.Wrap(err, "listing provisioning state records")
	}
	return resp, pagination.NextPageToken(nextPage), nil
}

// DeleteProvisioningState deletes a given principal's provisioning state
// record
func (ss *ProvisioningStateService) DeleteProvisioningState(ctx context.Context, downstreamID services.DownstreamID, name services.ProvisioningStateID) error {
	return trace.Wrap(ss.service.WithPrefix(string(downstreamID)).DeleteResource(ctx, string(name)))
}

// DeleteDownstreamProvisioningStates deletes *all* provisioning records for
// a given downstream
func (ss *ProvisioningStateService) DeleteDownstreamProvisioningStates(ctx context.Context, req *provisioningv1.DeleteDownstreamProvisioningStatesRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, trace.Wrap(ss.service.WithPrefix(req.GetDownstreamId()).DeleteAllResources(ctx))
}

// DeleteAllProvisioningStates deletes *all* provisioning records for a *all*
// downstream receivers
func (ss *ProvisioningStateService) DeleteAllProvisioningStates(ctx context.Context) error {
	return trace.Wrap(ss.service.DeleteAllResources(ctx))
}
