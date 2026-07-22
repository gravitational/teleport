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
	"strings"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
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
// SPIFFEFederations
type SPIFFEFederationService struct {
	service *generic.ServiceWrapper[*machineidv1.SPIFFEFederation]
}

// NewSPIFFEFederationService creates a new SPIFFEFederationService.
func NewSPIFFEFederationService(b backend.Backend) (*SPIFFEFederationService, error) {
	service, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*machineidv1.SPIFFEFederation]{
			Backend:       b,
			ResourceKind:  types.KindSPIFFEFederation,
			BackendPrefix: backend.NewKey(spiffeFederationPrefix),
			MarshalFunc:   services.MarshalSPIFFEFederation,
			UnmarshalFunc: services.UnmarshalSPIFFEFederation,
			ValidateFunc:  services.ValidateSPIFFEFederation,
		})
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

// DeleteSPIFFEFederation deletes a specific SPIFFEFederations.
func (b *SPIFFEFederationService) DeleteSPIFFEFederation(
	ctx context.Context, name string,
) error {
	return trace.Wrap(b.service.DeleteResource(ctx, name))
}

// DeleteAllSPIFFEFederations deletes all SPIFFE federations, this is typically
// only meant to be used by the cache.
func (b *SPIFFEFederationService) DeleteAllSPIFFEFederations(
	ctx context.Context,
) error {
	return trace.Wrap(b.service.DeleteAllResources(ctx))
}

// UpsertSPIFFEFederation upserts a SPIFFEFederations. Prefer using
// CreateSPIFFEFederation. This is only designed for usage by the cache.
func (b *SPIFFEFederationService) UpsertSPIFFEFederation(
	ctx context.Context, federation *machineidv1.SPIFFEFederation,
) (*machineidv1.SPIFFEFederation, error) {
	upserted, err := b.service.UpsertResource(ctx, federation)
	return upserted, trace.Wrap(err)
}

// UpdateSPIFFEFederation updates a specific SPIFFEFederation.
func (b *SPIFFEFederationService) UpdateSPIFFEFederation(
	ctx context.Context, federation *machineidv1.SPIFFEFederation,
) (*machineidv1.SPIFFEFederation, error) {
	updated, err := b.service.ConditionalUpdateResource(ctx, federation)
	return updated, trace.Wrap(err)
}

func newSPIFFEFederationParser() *spiffeFederationParser {
	return &spiffeFederationParser{
		baseParser: newBaseParser(backend.NewKey(spiffeFederationPrefix)),
	}
}

type spiffeFederationParser struct {
	baseParser
}

func (p *spiffeFederationParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpDelete:
		name := event.Item.Key.TrimPrefix(backend.NewKey(spiffeFederationPrefix)).String()
		if name == "" {
			return nil, trace.NotFound("failed parsing %v", event.Item.Key.String())
		}

		return &types.ResourceHeader{
			Kind:    types.KindSPIFFEFederation,
			Version: types.V1,
			Metadata: types.Metadata{
				Name:      strings.TrimPrefix(name, backend.SeparatorString),
				Namespace: apidefaults.Namespace,
			},
		}, nil
	case types.OpPut:
		federation, err := services.UnmarshalSPIFFEFederation(
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision))
		if err != nil {
			return nil, trace.Wrap(err, "unmarshalling resource from event")
		}
		return types.Resource153ToLegacy(federation), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}
