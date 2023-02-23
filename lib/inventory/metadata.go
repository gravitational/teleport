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
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

const (
	nodeService           = "node"
	kubeService           = "kube"
	appService            = "app"
	dbService             = "db"
	windowsDesktopService = "windows_desktop"
)

// This regexp is used to validate if the host architecture fetched
// has the expected format.
var matchHostArchitecture = regexp.MustCompile(`^\w+$`)

// fetchConfig contains the configuration used by the fetchAgentMetadata method.
type fetchConfig struct {
	ctx context.Context
	// hello is the initial upstream hello message.
	hello proto.UpstreamInventoryHello
	// readFile is the method called to read a file.
	// It is configurable so that it can be mocked in tests.
	readFile func(name string) ([]byte, error)
	// execCommand is the method called to execute a command.
	// It is configurable so that it can be mocked in tests.
	execCommand func(name string, args ...string) ([]byte, error)
}

// setDefaults sets the values of readFile and execCommand to the ones in the
// standard library. Having these two methods configurable allows us to mock
// them in tests.
func (cfg *fetchConfig) setDefaults() {
	if cfg.readFile == nil {
		cfg.readFile = os.ReadFile
	}
	if cfg.execCommand == nil {
		cfg.execCommand = func(name string, args ...string) ([]byte, error) {
			return exec.Command(name, args...).Output()
		}
	}
}

// fetchAgentMetadata fetches and calculates all agent metadata we are interested
// in tracking.
func fetchAgentMetadata(c *fetchConfig) proto.UpstreamInventoryAgentMetadata {
	c.setDefaults()
	return proto.UpstreamInventoryAgentMetadata{
		Version:               c.fetchVersion(),
		HostID:                c.fetchHostID(),
		Services:              c.fetchServices(),
		OS:                    c.fetchOS(),
		OSVersion:             c.fetchOSVersion(),
		HostArchitecture:      c.fetchHostArchitecture(),
		GLibCVersion:          c.fetchGlibcVersion(),
		InstallMethods:        c.fetchInstallMethods(),
		ContainerRuntime:      c.fetchContainerRuntime(),
		ContainerOrchestrator: c.fetchContainerOrchestrator(),
		CloudEnvironment:      c.fetchCloudEnvironment(),
	}
}

// fetchVersion returns the Teleport version present in the hello message.
func (c *fetchConfig) fetchVersion() string {
	return c.hello.Version
}

// fetchHostID returns the agent ID present in the hello message.
func (c *fetchConfig) fetchHostID() string {
	return c.hello.ServerID
}

// fetchOS returns the value of GOOS.
func (c *fetchConfig) fetchOS() string {
	return runtime.GOOS
}

// fetchServices computes the list of access protocols enabled at the agent from
// the list of system roles present in the hello message.
func (c *fetchConfig) fetchServices() []string {
	var services []string
	for _, svc := range c.hello.Services {
		switch svc {
		case types.RoleNode:
			services = append(services, nodeService)
		case types.RoleKube:
			services = append(services, kubeService)
		case types.RoleApp:
			services = append(services, appService)
		case types.RoleDatabase:
			services = append(services, dbService)
		case types.RoleWindowsDesktop:
			services = append(services, windowsDesktopService)
		}
	}
	return services
}

// fetchHostArchitecture computes the host architecture using the arch
// command-line utility.
func (c *fetchConfig) fetchHostArchitecture() string {
	cmd := "arch"
	arch, err := c.exec(cmd)
	if err != nil {
		return ""
	}

	if !matchHostArchitecture.MatchString(arch) {
		return invalid(cmd, arch)
	}

	return arch
}

func (c *fetchConfig) fetchInstallMethods() []string {
	// TODO(vitorenesduarte): fetch install methods
	return []string{}

}

// fetchContainerRuntime returns "docker" if the file "/.dockerenv" exists.
func (c *fetchConfig) fetchContainerRuntime() string {
	_, err := c.read("/.dockerenv")
	if err != nil {
		return ""
	}

	// If the file exists, we should be running on Docker.
	return "docker"
}

func (c *fetchConfig) fetchContainerOrchestrator() string {
	// TODO(vitorenesduarte): fetch container orchestrator
	return ""
}

func (c *fetchConfig) fetchCloudEnvironment() string {
	// TODO(vitorenesduarte): fetch cloud environment
	return ""
}

// exec runs a command and validates its output using the parse function.
func (cfg fetchConfig) exec(name string, args ...string) (string, error) {
	out, err := cfg.execCommand(name, args...)
	if err != nil {
		log.Debugf("Failed to execute command '%s': %s", name, err)
		return "", err
	}
	return string(out), nil
}

// read reads a read and validates its content using the parse function.
func (cfg fetchConfig) read(name string) (string, error) {
	out, err := cfg.readFile(name)
	if err != nil {
		log.Debugf("Failed to read file '%s': %s", name, err)
		return "", err
	}
	return string(out), nil
}

// invalid logs the unexpected output/content and sanitizes it by quoting it.
func invalid(in string, out string) string {
	log.Debugf("Unexpected '%q' format: %s", in, out)
	return sanitize(out)
}

// sanitize sanitizes some output/content by quoting it.
func sanitize(s string) string {
	return strconv.Quote(s)
}
