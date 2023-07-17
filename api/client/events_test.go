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
	app, err := types.NewAppV3(types.Metadata{
		Name: "app",
	}, types.AppSpecV3{
		URI:        "https://uri.com",
		PublicAddr: "https://public-addr.com",
	})
	require.NoError(t, err)

	tests := []struct {
		name  string
		event *authpb.Event
	}{
		{
			name:  "empty",
			event: &authpb.Event{},
		},
		{
			name: "gogo oneof",
			event: &authpb.Event{
				Type: authpb.Operation_PUT,
				Resource: &authpb.Event_App{
					App: app,
				},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			require.True(t, proto.Equal(test.event, test.event))
		})
	}
}
