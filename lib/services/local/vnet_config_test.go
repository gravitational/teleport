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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestVnetConfigService(t *testing.T) {
	t.Parallel()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	service, err := NewVnetConfigService(bk)
	require.NoError(t, err)

	ctx := context.Background()
	vnetConfig := &vnet.VnetConfig{
		Kind:     types.KindVnetConfig,
		Version:  types.V1,
		Metadata: &headerv1.Metadata{Name: vnetConfigSingletonName},
		Spec:     &vnet.VnetConfigSpec{Ipv4CidrRange: "192.168.1.0/24"},
	}

	// The following are not subtests because they depend on each other.

	// Create
	created, err := service.CreateVnetConfig(ctx, vnetConfig)
	require.NoError(t, err)
	diff := cmp.Diff(vnetConfig, created,
		cmpopts.IgnoreFields(headerv1.Metadata{}, "Revision"),
		cmpopts.IgnoreUnexported(vnet.VnetConfig{}, vnet.VnetConfigSpec{}, vnet.CustomDNSZone{}, headerv1.Metadata{}),
	)
	require.Empty(t, diff)
	require.NotEmpty(t, created.GetMetadata().GetRevision())

	// Get
	got, err := service.GetVnetConfig(ctx)
	require.NoError(t, err)
	diff = cmp.Diff(vnetConfig, got,
		cmpopts.IgnoreFields(headerv1.Metadata{}, "Revision"),
		cmpopts.IgnoreUnexported(vnet.VnetConfig{}, vnet.VnetConfigSpec{}, vnet.CustomDNSZone{}, headerv1.Metadata{}),
	)
	require.Empty(t, diff)
	require.Equal(t, created.GetMetadata().GetRevision(), got.GetMetadata().GetRevision())

	// Update
	vnetConfig.Spec.CustomDnsZones = append(vnetConfig.Spec.CustomDnsZones, &vnet.CustomDNSZone{Suffix: "example.com"})
	updated, err := service.UpdateVnetConfig(ctx, vnetConfig)
	require.NoError(t, err)
	require.NotEqual(t, got.GetSpec().GetCustomDnsZones(), updated.GetSpec().GetCustomDnsZones())

	// Upsert
	_, err = service.UpsertVnetConfig(ctx, vnetConfig)
	require.NoError(t, err)

	// Delete
	err = service.DeleteVnetConfig(ctx)
	require.NoError(t, err)

	// Get none
	_, err = service.GetVnetConfig(ctx)
	var notFoundError *trace.NotFoundError
	require.ErrorAs(t, err, &notFoundError)

	// Update none
	_, err = service.UpdateVnetConfig(ctx, vnetConfig)
	var compareFailedError *trace.CompareFailedError
	require.ErrorAs(t, err, &compareFailedError)
}

func TestVnetConfigValidation(t *testing.T) {
	t.Parallel()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	service, err := NewVnetConfigService(bk)
	require.NoError(t, err)

	ctx := context.Background()

	for _, tc := range []struct {
		name    string
		config  *vnet.VnetConfig
		wantErr bool
	}{
		{
			name: "invalid kind",
			config: &vnet.VnetConfig{
				Kind:     "invalidKind",
				Version:  types.V1,
				Metadata: &headerv1.Metadata{Name: vnetConfigSingletonName},
				Spec:     &vnet.VnetConfigSpec{Ipv4CidrRange: "192.168.1.0/24"},
			},
			wantErr: true,
		},
		{
			name: "invalid version",
			config: &vnet.VnetConfig{
				Kind:     types.KindVnetConfig,
				Version:  "v2",
				Metadata: &headerv1.Metadata{Name: vnetConfigSingletonName},
				Spec:     &vnet.VnetConfigSpec{Ipv4CidrRange: "192.168.1.0/24"},
			},
			wantErr: true,
		},
		{
			name: "invalid name",
			config: &vnet.VnetConfig{
				Kind:     types.KindVnetConfig,
				Version:  types.V1,
				Metadata: &headerv1.Metadata{Name: "wrong-name"},
				Spec:     &vnet.VnetConfigSpec{Ipv4CidrRange: "192.168.1.0/24"},
			},
			wantErr: true,
		},
		{
			name: "invalid CIDR",
			config: &vnet.VnetConfig{
				Kind:     types.KindVnetConfig,
				Version:  types.V1,
				Metadata: &headerv1.Metadata{Name: vnetConfigSingletonName},
				Spec:     &vnet.VnetConfigSpec{Ipv4CidrRange: "192.168.300.0/24"},
			},
			wantErr: true,
		},
		{
			name: "empty zone suffix",
			config: &vnet.VnetConfig{
				Kind:     types.KindVnetConfig,
				Version:  types.V1,
				Metadata: &headerv1.Metadata{Name: vnetConfigSingletonName},
				Spec: &vnet.VnetConfigSpec{
					Ipv4CidrRange: "192.168.1.0/24",
					CustomDnsZones: []*vnet.CustomDNSZone{
						&vnet.CustomDNSZone{
							Suffix: "",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid zone suffix",
			config: &vnet.VnetConfig{
				Kind:     types.KindVnetConfig,
				Version:  types.V1,
				Metadata: &headerv1.Metadata{Name: vnetConfigSingletonName},
				Spec: &vnet.VnetConfigSpec{
					Ipv4CidrRange: "192.168.1.0/24",
					CustomDnsZones: []*vnet.CustomDNSZone{
						&vnet.CustomDNSZone{
							Suffix: "invalid.character$",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid",
			config: &vnet.VnetConfig{
				Kind:     types.KindVnetConfig,
				Version:  types.V1,
				Metadata: &headerv1.Metadata{Name: vnetConfigSingletonName},
				Spec: &vnet.VnetConfigSpec{
					Ipv4CidrRange: "192.168.1.0/24",
					CustomDnsZones: []*vnet.CustomDNSZone{
						&vnet.CustomDNSZone{
							Suffix: "teleport.example.com",
						},
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Attempt to create with invalid config to test validation
			_, err := service.CreateVnetConfig(ctx, tc.config)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
