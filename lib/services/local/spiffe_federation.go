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

	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	spiffeFederationPrefix = "spiffe_federation"
)

// SPIFFEFederationService exposes backend functionality for storing
// SPIFFEFederation
type SPIFFEFederationService struct {
	service *generic.ServiceWrapper[*machineidv1.SPIFFEFederation]
}

// NewSPIFFEFederationService creates a new SPIFFEFederationService.
func NewSPIFFEFederationService(
	backend backend.Backend,
) (*SPIFFEFederationService, error) {
	service, err := generic.NewServiceWrapper(backend,
		types.KindSPIFFEFederation,
		spiffeFederationPrefix,
		services.MarshalSPIFFEFederation,
		services.UnmarshalSPIFFEFederation,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &SPIFFEFederationService{
		service: service,
	}, nil
}

// CreateSPIFFEFederation inserts a new SPIFFEFederation into the backend.
func (b *SPIFFEFederationService) CreateSPIFFEFederation(
	ctx context.Context, federation *machineidv1.SPIFFEFederation,
) (*machineidv1.SPIFFEFederation, error) {
	if err := services.ValidateSPIFFEFederation(federation); err != nil {
		return nil, trace.Wrap(err)
	}
	created, err := b.service.CreateResource(ctx, federation)
	return created, trace.Wrap(err)
}

// GetSPIFFEFederation retrieves a specific SPIFFEFederation given a name
func (b *SPIFFEFederationService) GetSPIFFEFederation(
	ctx context.Context, name string,
) (*machineidv1.SPIFFEFederation, error) {
	federation, err := b.service.GetResource(ctx, name)
	return federation, trace.Wrap(err)
}

// ListSPIFFEFederations lists all SPIFFEFederations using a given page size
// and last key.
func (b *SPIFFEFederationService) ListSPIFFEFederations(
	ctx context.Context, pageSize int, currentToken string,
) ([]*machineidv1.SPIFFEFederation, string, error) {
	r, nextToken, err := b.service.ListResources(ctx, pageSize, currentToken)
	return r, nextToken, trace.Wrap(err)
}

// DeleteSPIFFEFederation deletes a specific SPIFFEFederation.
func (b *SPIFFEFederationService) DeleteSPIFFEFederation(
	ctx context.Context, name string,
) error {
	return trace.Wrap(b.service.DeleteResource(ctx, name))
}
