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

package local

import (
	"context"

	"github.com/gravitational/trace"

	discoveryservicev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryservice/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const discoveryServicesPrefix = "discovery_services"

// DiscoveryServiceService manages discovery_service resources
// (Discovery Service configuration heartbeats) in the backend.
type DiscoveryServiceService struct {
	svc *generic.ServiceWrapper[*discoveryservicev1.DiscoveryService]
}

var _ services.DiscoveryServices = (*DiscoveryServiceService)(nil)

// NewDiscoveryServiceService creates a new DiscoveryServiceService.
func NewDiscoveryServiceService(b backend.Backend) (*DiscoveryServiceService, error) {
	svc, err := generic.NewServiceWrapper(generic.ServiceConfig[*discoveryservicev1.DiscoveryService]{
		Backend:       b,
		ResourceKind:  types.KindDiscoveryService,
		BackendPrefix: backend.NewKey(discoveryServicesPrefix),
		MarshalFunc:   services.MarshalDiscoveryService,
		UnmarshalFunc: services.UnmarshalDiscoveryService,
		ValidateFunc:  services.ValidateDiscoveryService,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &DiscoveryServiceService{svc: svc}, nil
}

// GetDiscoveryService implements [services.DiscoveryServices].
func (s *DiscoveryServiceService) GetDiscoveryService(ctx context.Context, name string) (*discoveryservicev1.DiscoveryService, error) {
	return s.svc.GetResource(ctx, name)
}

// ListDiscoveryServices implements [services.DiscoveryServices].
func (s *DiscoveryServiceService) ListDiscoveryServices(ctx context.Context, pageSize int, pageToken string) (_ []*discoveryservicev1.DiscoveryService, nextPageToken string, _ error) {
	return s.svc.ListResources(ctx, pageSize, pageToken)
}

// UpsertDiscoveryService implements [services.DiscoveryServices].
func (s *DiscoveryServiceService) UpsertDiscoveryService(ctx context.Context, svc *discoveryservicev1.DiscoveryService) (*discoveryservicev1.DiscoveryService, error) {
	return s.svc.UpsertResource(ctx, svc)
}

// DeleteDiscoveryService implements [services.DiscoveryServices].
func (s *DiscoveryServiceService) DeleteDiscoveryService(ctx context.Context, name string) error {
	return s.svc.DeleteResource(ctx, name)
}
