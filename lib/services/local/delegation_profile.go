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

const delegationProfilePrefix = "delegation_profile"

// DelegationProfileService exposes backend functionality for storing
// DelegationProfile resources.
type DelegationProfileService struct {
	service *generic.ServiceWrapper[*delegationv1.DelegationProfile]
}

var _ services.DelegationProfiles = (*DelegationProfileService)(nil)

// NewDelegationProfileService creates a new DelegationProfileService.
func NewDelegationProfileService(b backend.Backend) (*DelegationProfileService, error) {
	service, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*delegationv1.DelegationProfile]{
			Backend:       b,
			ResourceKind:  types.KindDelegationProfile,
			BackendPrefix: backend.NewKey(delegationProfilePrefix),
			MarshalFunc:   services.MarshalDelegationProfile,
			UnmarshalFunc: services.UnmarshalDelegationProfile,
			ValidateFunc:  services.ValidateDelegationProfile,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &DelegationProfileService{
		service: service,
	}, nil
}

// GetDelegationProfile gets a DelegationProfile by name.
func (s *DelegationProfileService) GetDelegationProfile(
	ctx context.Context,
	name string,
) (*delegationv1.DelegationProfile, error) {
	profile, err := s.service.GetResource(ctx, name)
	return profile, trace.Wrap(err)
}

// ListDelegationProfiles lists all DelegationProfile resources using Google
// style pagination.
func (s *DelegationProfileService) ListDelegationProfiles(
	ctx context.Context, pageSize int, lastToken string,
) ([]*delegationv1.DelegationProfile, string, error) {
	profiles, token, err := s.service.ListResources(ctx, pageSize, lastToken)
	return profiles, token, trace.Wrap(err)
}

// CreateDelegationProfile creates a new DelegationProfile.
func (s *DelegationProfileService) CreateDelegationProfile(
	ctx context.Context,
	delegationProfile *delegationv1.DelegationProfile,
) (*delegationv1.DelegationProfile, error) {
	profile, err := s.service.CreateResource(ctx, delegationProfile)
	return profile, trace.Wrap(err)
}

// DeleteDelegationProfile deletes a DelegationProfile by name.
func (s *DelegationProfileService) DeleteDelegationProfile(ctx context.Context, name string) error {
	return trace.Wrap(s.service.DeleteResource(ctx, name))
}

// DeleteAllDelegationProfiles deletes all DelegationProfile resources.
func (s *DelegationProfileService) DeleteAllDelegationProfiles(ctx context.Context) error {
	return trace.Wrap(s.service.DeleteAllResources(ctx))
}

// UpdateDelegationProfile updates a specific DelegationProfile.
//
// The resource must already exist, and, conditional update semantics are
// used - e.g the submitted resource must have a revision matching the
// revision of the resource in the backend.
func (s *DelegationProfileService) UpdateDelegationProfile(
	ctx context.Context,
	delegationProfile *delegationv1.DelegationProfile,
) (*delegationv1.DelegationProfile, error) {
	profile, err := s.service.ConditionalUpdateResource(ctx, delegationProfile)
	return profile, trace.Wrap(err)
}

// UpsertDelegationProfile creates or updates a DelegationProfile.
func (s *DelegationProfileService) UpsertDelegationProfile(
	ctx context.Context,
	delegationProfile *delegationv1.DelegationProfile,
) (*delegationv1.DelegationProfile, error) {
	profile, err := s.service.UpsertResource(ctx, delegationProfile)
	return profile, trace.Wrap(err)
}
