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
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	authpb "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	accesslistv1conv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	"github.com/gravitational/teleport/api/types/header"
)

// TestEventEqual will test an event object against a google proto.Equal. This is
// primarily to catch potential issues with using our "mixed" gogo + regular protobuf
// strategy.
func TestEventEqual(t *testing.T) {
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

	accessList1 := newAccessList(t, "1")
	accessList2 := newAccessList(t, "2")

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

func newAccessList(t *testing.T, name string) *accesslist.AccessList {
	t.Helper()

	accessList, err := accesslist.NewAccessList(
		header.Metadata{
			Name: name,
		},
		accesslist.Spec{
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
				Frequency: time.Hour,
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
			Members: []accesslist.Member{
				{
					Name:    "member1",
					Joined:  time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
					Expires: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					Reason:  "because",
					AddedBy: "test-user1",
				},
				{
					Name:    "member2",
					Joined:  time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
					Expires: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
					Reason:  "because again",
					AddedBy: "test-user2",
				},
			},
		},
	)
	require.NoError(t, err)

	return accessList
}
