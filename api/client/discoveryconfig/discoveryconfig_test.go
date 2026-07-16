/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package discoveryconfig

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	typesdiscoveryconfig "github.com/gravitational/teleport/api/types/discoveryconfig"
	conv "github.com/gravitational/teleport/api/types/discoveryconfig/convert/v1"
	"github.com/gravitational/teleport/api/types/header"
)

type listDiscoveryConfigsClient struct {
	discoveryconfigv1.DiscoveryConfigServiceClient
	request  *discoveryconfigv1.ListDiscoveryConfigsRequest
	response *discoveryconfigv1.ListDiscoveryConfigsResponse
}

func (c *listDiscoveryConfigsClient) ListDiscoveryConfigs(_ context.Context, req *discoveryconfigv1.ListDiscoveryConfigsRequest, _ ...grpc.CallOption) (*discoveryconfigv1.ListDiscoveryConfigsResponse, error) {
	c.request = req
	return c.response, nil
}

func (c *listDiscoveryConfigsClient) ListSyntheticDiscoveryConfigs(_ context.Context, req *discoveryconfigv1.ListDiscoveryConfigsRequest, _ ...grpc.CallOption) (*discoveryconfigv1.ListDiscoveryConfigsResponse, error) {
	c.request = req
	return c.response, nil
}

type getDiscoveryConfigClient struct {
	discoveryconfigv1.DiscoveryConfigServiceClient
	regular       *discoveryconfigv1.DiscoveryConfig
	regularErr    error
	synthetic     *discoveryconfigv1.DiscoveryConfig
	syntheticErr  error
	syntheticGets int
}

func (c *getDiscoveryConfigClient) GetDiscoveryConfig(_ context.Context, req *discoveryconfigv1.GetDiscoveryConfigRequest, _ ...grpc.CallOption) (*discoveryconfigv1.DiscoveryConfig, error) {
	if c.regularErr != nil {
		return nil, c.regularErr
	}
	return c.regular, nil
}

func (c *getDiscoveryConfigClient) GetSyntheticDiscoveryConfig(_ context.Context, req *discoveryconfigv1.GetDiscoveryConfigRequest, _ ...grpc.CallOption) (*discoveryconfigv1.DiscoveryConfig, error) {
	c.syntheticGets++
	if c.syntheticErr != nil {
		return nil, c.syntheticErr
	}
	return c.synthetic, nil
}

// TestGetDiscoveryConfigSyntheticFallback covers the client-side combined
// lookup: the legacy Get RPC is regular-only on the server, so regular-first,
// synthetic-second resolution for reserved synthetic names lives in the
// client.
func TestGetDiscoveryConfigSyntheticFallback(t *testing.T) {
	const serverID = "00000000-0000-0000-0000-000000000001"
	syntheticName := typesdiscoveryconfig.SyntheticName(serverID)

	regular, err := typesdiscoveryconfig.NewDiscoveryConfig(header.Metadata{Name: "regular"}, typesdiscoveryconfig.Spec{DiscoveryGroup: "group"})
	require.NoError(t, err)
	synthetic, err := typesdiscoveryconfig.NewSyntheticDiscoveryConfig(serverID, typesdiscoveryconfig.SyntheticStatus{
		DiscoveryGroup: "group",
		Matchers:       &typesdiscoveryconfig.Spec{},
	})
	require.NoError(t, err)

	t.Run("regular resources resolve without the synthetic RPC", func(t *testing.T) {
		grpcClient := &getDiscoveryConfigClient{regular: conv.ToProto(regular)}
		got, err := NewClient(grpcClient).GetDiscoveryConfig(t.Context(), "regular")
		require.NoError(t, err)
		require.Equal(t, "regular", got.GetName())
		require.Zero(t, grpcClient.syntheticGets)
	})

	t.Run("reserved names fall back to the synthetic RPC on NotFound", func(t *testing.T) {
		grpcClient := &getDiscoveryConfigClient{
			regularErr: trace.NotFound("not found"),
			synthetic:  conv.ToProto(synthetic),
		}
		got, err := NewClient(grpcClient).GetDiscoveryConfig(t.Context(), syntheticName)
		require.NoError(t, err)
		require.True(t, got.IsSynthetic())
		require.Equal(t, 1, grpcClient.syntheticGets)
	})

	t.Run("non-reserved names do not trigger the fallback", func(t *testing.T) {
		grpcClient := &getDiscoveryConfigClient{regularErr: trace.NotFound("not found")}
		_, err := NewClient(grpcClient).GetDiscoveryConfig(t.Context(), "regular")
		require.True(t, trace.IsNotFound(err), "got %v", err)
		require.Zero(t, grpcClient.syntheticGets)
	})

	t.Run("NotImplemented is preserved for mixed-version Auth clusters", func(t *testing.T) {
		grpcClient := &getDiscoveryConfigClient{
			regularErr:   trace.NotFound("not found"),
			syntheticErr: trace.NotImplemented("unknown method"),
		}
		_, err := NewClient(grpcClient).GetDiscoveryConfig(t.Context(), syntheticName)
		require.True(t, trace.IsNotImplemented(err), "got %v", err)
	})
}

func TestListSyntheticDiscoveryConfigs(t *testing.T) {
	regular, err := typesdiscoveryconfig.NewDiscoveryConfig(header.Metadata{Name: "regular"}, typesdiscoveryconfig.Spec{DiscoveryGroup: "group"})
	require.NoError(t, err)
	synthetic := regular.Clone()
	synthetic.SetName("synthetic-owner")
	synthetic.SetSubKind(typesdiscoveryconfig.SubKindSynthetic)

	grpcClient := &listDiscoveryConfigsClient{
		response: discoveryconfigv1.ListDiscoveryConfigsResponse_builder{
			DiscoveryConfigs: []*discoveryconfigv1.DiscoveryConfig{conv.ToProto(synthetic), conv.ToProto(regular)},
			NextKey:          "raw-next-key",
		}.Build(),
	}
	client := NewClient(grpcClient)

	configs, nextToken, err := client.ListSyntheticDiscoveryConfigs(t.Context(), 2, "start")
	require.NoError(t, err)
	require.Equal(t, int32(2), grpcClient.request.GetPageSize())
	require.Equal(t, "start", grpcClient.request.GetNextToken())
	require.Equal(t, "raw-next-key", nextToken)
	require.Len(t, configs, 2)
}
