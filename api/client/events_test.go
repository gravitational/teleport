// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	authpb "github.com/gravitational/teleport/api/client/proto"
	discoveryservicev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryservice/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	accesslistv1conv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	"github.com/gravitational/teleport/api/types/header"
)

// TestEventDiscoveryServiceRoundTrip pins the watcher event codec for the
// discovery_service resource: EventToGRPC must have a oneof case for the
// wrapped resource (a missing case returns BadParameter, which closes remote
// watcher streams), the gRPC event must survive binary proto encoding with
// the legacy gogoproto matcher types embedded in the spec, and EventFromGRPC
// must hand back the same heartbeat.
func TestEventDiscoveryServiceRoundTrip(t *testing.T) {
	hb := discoveryservicev1.DiscoveryService_builder{
		Kind:     types.KindDiscoveryService,
		Version:  types.V1,
		Metadata: headerv1.Metadata_builder{Name: "host-1"}.Build(),
		Spec: discoveryservicev1.DiscoveryServiceSpec_builder{
			DiscoveryGroup: "demo",
			StaticMatchers: discoveryservicev1.StaticMatchers_builder{
				Aws: []*types.AWSMatcher{{
					Types:   []string{"ec2"},
					Regions: []string{"us-east-1"},
					Tags:    types.Labels{"env": []string{"prod"}},
				}},
			}.Build(),
		}.Build(),
	}.Build()

	grpcEvent, err := EventToGRPC(types.Event{
		Type:     types.OpPut,
		Resource: types.Resource153ToLegacy(hb),
	})
	require.NoError(t, err)

	encoded, err := proto.Marshal(grpcEvent)
	require.NoError(t, err)
	decoded := &authpb.Event{}
	require.NoError(t, proto.Unmarshal(encoded, decoded))

	out, err := EventFromGRPC(decoded)
	require.NoError(t, err)
	got, ok := out.Resource.(types.Resource153UnwrapperT[*discoveryservicev1.DiscoveryService])
	require.True(t, ok, "expected a discovery_service event resource, got %T", out.Resource)
	require.Empty(t, cmp.Diff(hb, got.UnwrapT(), protocmp.Transform()))
}

// TestEventEqual will test an event object against a google proto.Equal. This is
// primarily to catch potential issues with using our "mixed" gogo + regular protobuf
// strategy.
func TestEventEqual(t *testing.T) {
	clock := clockwork.NewFakeClock()
	app1, err := types.NewAppV3(types.Metadata{
		Name: "app1",
	}, types.AppSpecV3{
		URI:        "https://uri.com",
		PublicAddr: "https://public-addr.com",
	})
	require.NoError(t, err)

	app1Dupe, err := types.NewAppV3(types.Metadata{
		Name: "app1",
	}, types.AppSpecV3{
		URI:        "https://uri.com",
		PublicAddr: "https://public-addr.com",
	})
	require.NoError(t, err)

	app2, err := types.NewAppV3(types.Metadata{
		Name: "app2",
	}, types.AppSpecV3{
		URI:        "https://uri.com",
		PublicAddr: "https://public-addr.com",
	})
	require.NoError(t, err)

	accessList1 := newAccessList(t, "1", clock)
	accessList2 := newAccessList(t, "2", clock)

	tests := []struct {
		name     string
		event1   *authpb.Event
		event2   *authpb.Event
		expected bool
	}{
		{
			name:     "empty equal",
			event1:   &authpb.Event{},
			event2:   &authpb.Event{},
			expected: true,
		},
		{
			name:   "empty not equal",
			event1: &authpb.Event{},
			event2: &authpb.Event{
				Type: authpb.Operation_PUT,
				Resource: &authpb.Event_App{
					App: app1,
				},
			},
			expected: false,
		},
		{
			name: "gogo oneof equal",
			event1: &authpb.Event{
				Type: authpb.Operation_PUT,
				Resource: &authpb.Event_App{
					App: app1,
				},
			},
			event2: &authpb.Event{
				Type: authpb.Operation_PUT,
				Resource: &authpb.Event_App{
					App: app1Dupe,
				},
			},
			expected: true,
		},
		{
			name: "gogo oneof not equal",
			event1: &authpb.Event{
				Type: authpb.Operation_PUT,
				Resource: &authpb.Event_App{
					App: app1,
				},
			},
			event2: &authpb.Event{
				Type: authpb.Operation_PUT,
				Resource: &authpb.Event_App{
					App: app2,
				},
			},
			expected: false,
		},
		{
			name: "regular protobuf oneof equal",
			event1: &authpb.Event{
				Type: authpb.Operation_PUT,
				Resource: &authpb.Event_AccessList{
					AccessList: accesslistv1conv.ToProto(accessList1),
				},
			},
			event2: &authpb.Event{
				Type: authpb.Operation_PUT,
				Resource: &authpb.Event_AccessList{
					AccessList: accesslistv1conv.ToProto(accessList1),
				},
			},
			expected: true,
		},
		{
			name: "regular protobuf oneof not equal",
			event1: &authpb.Event{
				Type: authpb.Operation_PUT,
				Resource: &authpb.Event_AccessList{
					AccessList: accesslistv1conv.ToProto(accessList1),
				},
			},
			event2: &authpb.Event{
				Type: authpb.Operation_PUT,
				Resource: &authpb.Event_AccessList{
					AccessList: accesslistv1conv.ToProto(accessList2),
				},
			},
			expected: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			event1 := test.event1
			event2 := test.event2
			require.Equal(t, test.expected, proto.Equal(event1, event2))
		})
	}
}

func newAccessList(t *testing.T, name string, clock clockwork.Clock) *accesslist.AccessList {
	t.Helper()

	accessList, err := accesslist.NewAccessList(
		header.Metadata{
			Name: name,
		},
		accesslist.Spec{
			Title:       "title",
			Description: "test access list",
			Owners: []accesslist.Owner{
				{
					Name:        "test-user1",
					Description: "test user 1",
				},
				{
					Name:        "test-user2",
					Description: "test user 2",
				},
			},
			Audit: accesslist.Audit{
				NextAuditDate: clock.Now(),
			},
			MembershipRequires: accesslist.Requires{
				Roles: []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			OwnershipRequires: accesslist.Requires{
				Roles: []string{"orole1", "orole2"},
				Traits: map[string][]string{
					"otrait1": {"ovalue1", "ovalue2"},
					"otrait2": {"ovalue3", "ovalue4"},
				},
			},
			Grants: accesslist.Grants{
				Roles: []string{"grole1", "grole2"},
				Traits: map[string][]string{
					"gtrait1": {"gvalue1", "gvalue2"},
					"gtrait2": {"gvalue3", "gvalue4"},
				},
			},
		},
	)
	require.NoError(t, err)

	return accessList
}
