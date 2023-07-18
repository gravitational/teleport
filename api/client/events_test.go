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

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	authpb "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
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

	app2, err := types.NewAppV3(types.Metadata{
		Name: "app2",
	}, types.AppSpecV3{
		URI:        "https://uri.com",
		PublicAddr: "https://public-addr.com",
	})
	require.NoError(t, err)

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
					App: app1,
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
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.expected, proto.Equal(test.event1, test.event2))
		})
	}
}
