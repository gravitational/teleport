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

package services

import (
	"context"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
)

// VnetConfigGetter is an interface for getting the cluster singleton VnetConfig.
type VnetConfigGetter interface {
	// GetVnetConfig returns the singleton VnetConfig resource.
	GetVnetConfig(context.Context) (*vnet.VnetConfig, error)
}

// VnetConfigService is an interface for the VnetConfig service.
type VnetConfigService interface {
	VnetConfigGetter

	// CreateVnetConfig does basic validation and creates a VnetConfig resource.
	CreateVnetConfig(ctx context.Context, vnetConfig *vnet.VnetConfig) (*vnet.VnetConfig, error)

	// UpdateVnetConfig does basic validation and updates a VnetConfig resource.
	UpdateVnetConfig(ctx context.Context, vnetConfig *vnet.VnetConfig) (*vnet.VnetConfig, error)

	// UpsertVnetConfig does basic validation and upserts a VnetConfig resource.
	UpsertVnetConfig(ctx context.Context, vnetConfig *vnet.VnetConfig) (*vnet.VnetConfig, error)

	// DeleteVnetConfig deletes the singleton VnetConfig resource.
	DeleteVnetConfig(ctx context.Context) error
}
