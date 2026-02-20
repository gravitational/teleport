// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package grpcclientconfigv1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	grpcv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/grpcclientconfig/v1"
	"github.com/gravitational/teleport/api/types/grpcclientconfig"
)

func TestGetServiceConfigEnvVar(t *testing.T) {
	ptr := func(s string) *string {
		return &s
	}

	for _, tt := range []struct {
		name     string
		env      *string
		expected *grpcv1.ServiceConfig
	}{
		{
			name:     "default service config",
			env:      nil,
			expected: grpcclientconfig.DefaultServiceConfig(),
		},
		{
			name: "custom service config envvar",
			env:  ptr(`{"loadBalancingConfig": [{"teleport_pick_healthy": {"mode": "MODE_RECONNECT"}}], "healthCheckConfig":{"serviceName": "test"}}`),
			expected: &grpcv1.ServiceConfig{
				LoadBalancingConfig: []*grpcv1.LoadBalancerConfig{{
					Config: &grpcv1.LoadBalancerConfig_TeleportPickHealthy{
						TeleportPickHealthy: &grpcv1.TeleportPickHealthyConfig{
							Mode: grpcv1.Mode_MODE_RECONNECT,
						},
					},
				}},
				HealthCheckConfig: &grpcv1.HealthCheckConfig{
					ServiceName: ptr("test"),
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != nil {
				t.Setenv(serviceConfigEnvVar, *tt.env)
			}
			service, err := NewService()
			require.NoError(t, err)

			resp, err := service.GetServiceConfig(t.Context(), &grpcv1.GetServiceConfigRequest{})
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(tt.expected, resp.GetConfig(), protocmp.Transform()))
		})
	}
}
