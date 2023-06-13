/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestFriendlyName(t *testing.T) {
	appNoFriendly, err := types.NewAppV3(types.Metadata{
		Name: "no friendly",
	}, types.AppSpecV3{
		URI: "https://some-uri.com",
	},
	)
	require.NoError(t, err)

	appFriendly, err := types.NewAppV3(types.Metadata{
		Name:        "no friendly",
		Description: "friendly name",
		Labels: map[string]string{
			types.OriginLabel: types.OriginOkta,
		},
	}, types.AppSpecV3{
		URI: "https://some-uri.com",
	},
	)
	require.NoError(t, err)

	node, err := types.NewServer("node", types.KindNode, types.ServerSpecV2{
		Hostname: "friendly hostname",
	})
	require.NoError(t, err)

	tests := []struct {
		name     string
		resource types.ResourceWithLabels
		expected string
	}{
		{
			name:     "no friendly name",
			resource: appNoFriendly,
			expected: "",
		},
		{
			name:     "friendly app name",
			resource: appFriendly,
			expected: "friendly name",
		},
		{
			name:     "friendly node name",
			resource: node,
			expected: "friendly hostname",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, FriendlyName(test.resource))
		})
	}
}
