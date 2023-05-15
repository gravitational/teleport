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

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetTunnelType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		appServer AppServer
		expected  TunnelType
	}{
		{
			name:      "default",
			appServer: &AppServerV3{},
			expected:  AppTunnel,
		},
		{
			name: "okta",
			appServer: &AppServerV3{
				Metadata: Metadata{
					Labels: map[string]string{
						OriginLabel: OriginOkta,
					},
				},
			},
			expected: OktaTunnel,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, test.appServer.GetTunnelType())
		})
	}
}
