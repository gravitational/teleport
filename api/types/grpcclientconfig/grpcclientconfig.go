// Copyright 2025 Gravitational, Inc
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

package grpcclientconfig

import (
	grpcv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/grpcclientconfig/v1"
)

// DefaultServiceConfig constructs the default service config the server will
// present to clients.
func DefaultServiceConfig() *grpcv1.ServiceConfig {
	return &grpcv1.ServiceConfig{
		LoadBalancingConfig: []*grpcv1.LoadBalancerConfig{{
			Config: &grpcv1.LoadBalancerConfig_TeleportPickHealthy{
				TeleportPickHealthy: defaultTeleportPickHealthy(),
			},
		}},
	}
}

func defaultTeleportPickHealthy() *grpcv1.TeleportPickHealthyConfig {
	return &grpcv1.TeleportPickHealthyConfig{
		Mode: grpcv1.Mode_MODE_PICK_FIRST,
	}
}

// TeleportPickHealthy searches for a [grpcv1.TeleportPickHealthConfig] in the
// service config or returns the default configuration.
func TeleportPickHealthy(config *grpcv1.ServiceConfig) *grpcv1.TeleportPickHealthyConfig {
	for _, c := range config.GetLoadBalancingConfig() {
		if tc := c.GetTeleportPickHealthy(); tc != nil {
			return tc
		}
	}
	return defaultTeleportPickHealthy()
}

// HealthCheckingEnabled checks if healthchecking is enabled in the service
// config, defaulting to false.
func HealthCheckingEnabled(config *grpcv1.ServiceConfig) bool {
	if config == nil || config.HealthCheckConfig == nil || config.HealthCheckConfig.ServiceName == nil {
		return false
	}
	return true
}
