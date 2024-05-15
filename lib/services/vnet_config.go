package services

import (
	"context"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
)

// VnetConfigService is an interface for the VnetConfig service.
type VnetConfigService interface {
	// GetVnetConfig returns the singleton VnetConfig resource.
	GetVnetConfig(context.Context) (*vnet.VnetConfig, error)

	// CreateVnetConfig does basic validation and creates a VnetConfig resource.
	CreateVnetConfig(ctx context.Context, vnetConfig *vnet.VnetConfig) (*vnet.VnetConfig, error)

	// UpdateVnetConfig does basic validation and updates a VnetConfig resource.
	UpdateVnetConfig(ctx context.Context, vnetConfig *vnet.VnetConfig) (*vnet.VnetConfig, error)

	// UpsertVnetConfig does basic validation and upserts a VnetConfig resource.
	UpsertVnetConfig(ctx context.Context, vnetConfig *vnet.VnetConfig) (*vnet.VnetConfig, error)

	// DeleteVnetConfig deletes the singleton VnetConfig resource.
	DeleteVnetConfig(ctx context.Context) error
}
