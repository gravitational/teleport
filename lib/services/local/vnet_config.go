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
	"log/slog"

	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/api/types"
	typesvnet "github.com/gravitational/teleport/api/types/vnet"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

// VnetConfigService implements the storage layer for the VnetConfig resource.
type VnetConfigService struct {
	slog *slog.Logger
	svc  *generic.ServiceWrapper[*vnet.VnetConfig]
}

// NewVnetConfigService returns a new VnetConfig storage service.
func NewVnetConfigService(b backend.Backend) (*VnetConfigService, error) {
	svc, err := generic.NewServiceWrapper(
		generic.ServiceConfig[*vnet.VnetConfig]{
			Backend:       b,
			ResourceKind:  types.KindVnetConfig,
			BackendPrefix: backend.NewKey(types.KindVnetConfig),
			MarshalFunc:   services.MarshalProtoResource[*vnet.VnetConfig],
			UnmarshalFunc: services.UnmarshalProtoResource[*vnet.VnetConfig],
			ValidateFunc:  validateVnetConfig,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &VnetConfigService{
		svc:  svc,
		slog: slog.With(teleport.ComponentKey, "VnetConfig.local"),
	}, nil
}

// GetVnetConfig returns the singleton VnetConfig resource.
func (s *VnetConfigService) GetVnetConfig(ctx context.Context) (*vnet.VnetConfig, error) {
	vnetConfig, err := s.svc.GetResource(ctx, types.MetaNameVnetConfig)
	return vnetConfig, trace.Wrap(err)
}

// CreateVnetConfig does basic validation and creates a VnetConfig resource.
func (s *VnetConfigService) CreateVnetConfig(ctx context.Context, vnetConfig *vnet.VnetConfig) (*vnet.VnetConfig, error) {
	vnetConfig, err := s.svc.CreateResource(ctx, vnetConfig)
	return vnetConfig, trace.Wrap(err)
}

// UpdateVnetConfig does basic validation and updates a VnetConfig resource.
func (s *VnetConfigService) UpdateVnetConfig(ctx context.Context, vnetConfig *vnet.VnetConfig) (*vnet.VnetConfig, error) {
	vnetConfig, err := s.svc.ConditionalUpdateResource(ctx, vnetConfig)
	return vnetConfig, trace.Wrap(err)
}

// UpsertVnetConfig does basic validation and upserts a VnetConfig resource.
func (s *VnetConfigService) UpsertVnetConfig(ctx context.Context, vnetConfig *vnet.VnetConfig) (*vnet.VnetConfig, error) {
	vnetConfig, err := s.svc.UpsertResource(ctx, vnetConfig)
	return vnetConfig, trace.Wrap(err)
}

// DeleteVnetConfig deletes the singleton VnetConfig resource.
func (s *VnetConfigService) DeleteVnetConfig(ctx context.Context) error {
	return trace.Wrap(s.svc.DeleteResource(ctx, types.MetaNameVnetConfig))
}

func validateVnetConfig(vnetConfig *vnet.VnetConfig) error {
	if err := typesvnet.ValidateVnetConfig(vnetConfig); err != nil {
		return trace.Wrap(err)
	}

	// This validation is not present in typesvnet.ValidateVnetConfig because the api package does not
	// have k8s.io/apimachinery in its deps.
	for _, zone := range vnetConfig.GetSpec().GetCustomDnsZones() {
		suffix := zone.GetSuffix()
		errs := validation.IsDNS1123Subdomain(suffix)
		if len(errs) > 0 {
			return trace.BadParameter("validating custom_dns_zone.suffix %q: %s", suffix, errs)
		}
	}

	return nil
}
