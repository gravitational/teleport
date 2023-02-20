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

package inventory

import (
	"context"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/client/proto"
)

// fetchAgentMetadata fetches and calculates all agent metadata we are interested
// in tracking.
func fetchAgentMetadata(ctx context.Context, hello proto.UpstreamInventoryHello) proto.UpstreamInventoryAgentMetadata {
	var services []string
	for _, svc := range hello.Services {
		switch svc {
		case types.RoleNode:
			services = append(services, "node")
		case types.RoleKube:
			services = append(services, "kube")
		case types.RoleApp:
			services = append(services, "app")
		case types.RoleDatabase:
			services = append(services, "db")
		case types.RoleWindowsDesktop:
			services = append(services, "windows_desktop")
		}
	}
	metadata := proto.UpstreamInventoryAgentMetadata{
		Version:  hello.Version,
		ServerID: hello.ServerID,
		Services: services,
	}
	// TODO(vitorenesduarte): fetch remaining metadata
	return metadata
}
