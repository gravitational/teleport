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

	"github.com/gravitational/teleport/api"
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

func TestNewAppServerForAWSOIDCIntegration(t *testing.T) {
	for _, tt := range []struct {
		name           string
		integratioName string
		hostID         string
		publicAddr     string
		labels         map[string]string
		expectedApp    *AppServerV3
		errCheck       require.ErrorAssertionFunc
	}{
		{
			name:           "valid",
			integratioName: "valid",
			hostID:         "my-host-id",
			publicAddr:     "valid.proxy.example.com",
			labels:         map[string]string{"account_id": "123456789012"},
			expectedApp: &AppServerV3{
				Kind:    KindAppServer,
				Version: V3,
				Metadata: Metadata{
					Name:      "valid",
					Namespace: "default",
					Labels:    map[string]string{"account_id": "123456789012"},
				},
				Spec: AppServerSpecV3{
					Version: api.Version,
					HostID:  "my-host-id",
					App: &AppV3{
						Kind:    KindApp,
						Version: V3,
						Metadata: Metadata{
							Name:      "valid",
							Namespace: "default",
							Labels:    map[string]string{"account_id": "123456789012"},
						},
						Spec: AppSpecV3{
							URI:         "https://console.aws.amazon.com",
							Cloud:       "AWS",
							Integration: "valid",
							PublicAddr:  "valid.proxy.example.com",
						},
					},
				},
			},
			errCheck: require.NoError,
		},
		{
			name:           "error when HostID is missing",
			integratioName: "invalid-missing-hostid",
			errCheck:       require.Error,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			app, err := NewAppServerForAWSOIDCIntegration(tt.integratioName, tt.hostID, tt.publicAddr, tt.labels)
			if tt.errCheck != nil {
				tt.errCheck(t, err)
			}
			if tt.expectedApp != nil {
				require.Equal(t, tt.expectedApp, app)
			}
		})
	}
}
