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

const (
	// ModePickFirst configures the client balancer to behave like the default grpc
	// pick_first balancer.
	ModePickFirst string = "pick_first"
	// ModeReconnect configures the client balancer to reconnect when the service
	// becomes unhealthy.
	ModeReconnect string = "reconnect"
)

// DefaultServiceConfig constructs the default service config the server will
// present to clients.
func DefaultServiceConfig() *grpcv1.ServiceConfig {
	return &grpcv1.ServiceConfig{
		LoadBalancingConfig: []*grpcv1.LoadBalancerConfig{{
			Config: &grpcv1.LoadBalancerConfig_TeleportPickHealthy{
				TeleportPickHealthy: &grpcv1.TeleportPickHealthyConfig{
					Mode: ModePickFirst,
				},
			},
		}},
	}
}
