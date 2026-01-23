package grpcclientconfigv1

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

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
				require.NoError(t, os.Setenv(serviceConfigEnvVar, *tt.env))
			}
			service, err := NewService()
			require.NoError(t, err)

			resp, err := service.GetServiceConfig(context.Background(), &grpcv1.GetServiceConfigRequest{})
			require.NoError(t, err)
			require.EqualExportedValues(t, tt.expected, resp.GetConfig())
		})
	}
}
