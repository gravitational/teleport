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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	discoveryservicev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryservice/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
)

// TestDiscoveryServiceRoundTrip proves that the discovery_service resource —
// a protov2 message embedding legacy gogoproto matcher types — survives the
// encoding/json marshal/unmarshal used by the generic backend service.
func TestDiscoveryServiceRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	mem, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mem.Close()) })

	svc, err := NewDiscoveryServiceService(mem)
	require.NoError(t, err)

	original := discoveryservicev1.DiscoveryService_builder{
		Kind:    types.KindDiscoveryService,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: "aaaaaaaa-1111-2222-3333-444444444444",
			// The round trip is the subject here, not liveness; a distant
			// expiry keeps the field exercised without letting a stalled
			// CI worker expire the resource mid-test.
			Expires: timestamppb.New(time.Now().UTC().Add(24 * time.Hour)),
		}.Build(),
		Spec: discoveryservicev1.DiscoveryServiceSpec_builder{
			Hostname:        "disc-1.example.com",
			TeleportVersion: "19.0.0-dev",
			DiscoveryGroup:  "demo",
			PollInterval:    durationpb.New(5 * time.Minute),
			StaticMatchers: discoveryservicev1.StaticMatchers_builder{
				Aws: []*types.AWSMatcher{{
					Types:       []string{"ec2"},
					Regions:     []string{"us-east-1"},
					Integration: "my-oidc",
					Tags:        types.Labels{"env": []string{"prod"}},
				}},
				Azure: []*types.AzureMatcher{{
					Types:          []string{"vm"},
					Regions:        []string{"eastus"},
					Subscriptions:  []string{"sub-1"},
					ResourceGroups: []string{"rg-1"},
				}},
				Gcp: []*types.GCPMatcher{{
					Types:      []string{"gce"},
					ProjectIDs: []string{"project-1"},
					Locations:  []string{"us-central1"},
				}},
				AccessGraph: &types.AccessGraphSync{
					AWS: []*types.AccessGraphAWSSync{{
						Regions: []string{"us-east-1"},
					}},
				},
			}.Build(),
		}.Build(),
		Status: &discoveryservicev1.DiscoveryServiceStatus{},
	}.Build()

	upserted, err := svc.UpsertDiscoveryService(ctx, original)
	require.NoError(t, err)
	require.NotEmpty(t, upserted.GetMetadata().GetRevision())

	got, err := svc.GetDiscoveryService(ctx, original.GetMetadata().GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(original, got, protocmp.Transform(), protocmp.IgnoreFields(&headerv1.Metadata{}, "revision")))

	items, next, err := svc.ListDiscoveryServices(ctx, 10, "")
	require.NoError(t, err)
	require.Empty(t, next)
	require.Len(t, items, 1)

	require.NoError(t, svc.DeleteDiscoveryService(ctx, original.GetMetadata().GetName()))
	_, err = svc.GetDiscoveryService(ctx, original.GetMetadata().GetName())
	require.True(t, trace.IsNotFound(err),
		"deleted heartbeat must read as NotFound, got %v", err)
}
