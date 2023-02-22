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
	"os/exec"
	"runtime"

	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
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
		Version:               hello.Version,
		HostID:                hello.ServerID,
		Services:              services,
		OS:                    runtime.GOOS,
		OSVersion:             fetchOSVersion(),
		HostArchitecture:      fetchHostArchitecture(),
		GLibCVersion:          fetchGlibcVersion(),
		InstallMethods:        fetchInstallMethods(),
		ContainerRuntime:      fetchContainerRuntime(),
		ContainerOrchestrator: fetchContainerOrchestrator(),
		CloudEnvironment:      fetchCloudEnvironment(),
	}
	// TODO(vitorenesduarte): fetch remaining metadata
	return metadata
}

func fetchOSVersion() string {
	return ""
}

// fetchHostArchitecture computes the host architecture using the arch
// command-line utility.
func fetchHostArchitecture() string {
	out, err := exec.Command("arch").Output()
	if err != nil {
		log.Debugf("Failed to execute 'arch' command: %s", err)
		return ""
	}
	return string(out)
}

func fetchGlibcVersion() string {
	return ""
}

func fetchInstallMethods() []string {
	return []string{}
}

func fetchContainerRuntime() string {
	return ""
}

func fetchContainerOrchestrator() string {
	return ""
}

func fetchCloudEnvironment() string {
	return ""
}
