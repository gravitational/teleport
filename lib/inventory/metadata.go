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
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

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
	// httpDo is the method called to perform an httpDo request.
	// It is configurable so that it can be mocked in tests.
	httpDo func(req *http.Request) (*http.Response, error)
	// kubeClient is a kubernetes client used to retrieve the
	// server version.
	// It is configurable so that it can be mocked in tests.
	kubeClient kubernetes.Interface
}

// setDefaults sets the values of readFile and execCommand to the ones in the
// standard library. Having these two methods configurable allows us to mock
// them in tests.
func (c *fetchConfig) setDefaults() {
	if c.ctx == nil {
		c.ctx = context.Background()
	}
	if c.readFile == nil {
		c.readFile = os.ReadFile
	}
	if c.execCommand == nil {
		c.execCommand = func(name string, args ...string) ([]byte, error) {
			return exec.Command(name, args...).Output()
		}
	}
	if c.httpDo == nil {
		c.httpDo = func(req *http.Request) (*http.Response, error) {
			return http.DefaultClient.Do(req)
		}
	}
	if c.kubeClient == nil {
		c.kubeClient = getKubeClient()
	}
}

// getKubeClient returns a kubernetes client in case the agent is running on
// kubernetes. It returns nil otherwise.
func getKubeClient() kubernetes.Interface {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Debugf("Failed to get kubernetes cluster config: %s", err)
		return nil
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Debugf("Failed to create kubernetes client: %s", err)
		return nil
	}
	return client
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
	command := "arch"
	out, err := c.exec(command)
	if err != nil {
		return ""
	}

	if !matchHostArchitecture.MatchString(out) {
		return invalid(command, out)
	}

	return out
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

// fetchContainerOrchestrator returns kubernetes-${GIT_VERSION} if the agent is
// running on kubernetes.
func (c *fetchConfig) fetchContainerOrchestrator() string {
	if c.kubeClient == nil {
		// Return empty if not running on kubernetes.
		return ""
	}

	version, err := c.kubeClient.Discovery().ServerVersion()
	if err != nil {
		log.Debugf("Failed to retrieve kubernetes server version: %s", err)
	}

	return fmt.Sprintf("kubernetes-%s", version.GitVersion)
}

// fetchCloudEnvironment returns aws, gpc or azure if the agent is running on
// such cloud environments.
func (c *fetchConfig) fetchCloudEnvironment() string {
	if c.awsHttpGetSuccess() {
		return "aws"
	}
	if c.gcpHttpGetSuccess() {
		return "gcp"
	}
	if c.azureHttpGetSuccess() {
		return "azure"
	}
	return ""
}

// awsHttpGetSuccess hits the AWS metadata endpoint in order to detect whether
// the agent is running on AWS.
// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-retrieval.html
func (c *fetchConfig) awsHttpGetSuccess() bool {
	url := "http://169.254.169.254/latest/meta-data/"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Debugf("Failed to create AWS http GET request '%s': %s", url, err)
		return false
	}

	return c.httpReqSuccess(req)
}

// gcpHttpGetSuccess hits the GCP metadata endpoint in order to detect whether
// the agent is running on GCP.
// https://cloud.google.com/compute/docs/metadata/overview#parts-of-a-request
func (c *fetchConfig) gcpHttpGetSuccess() bool {
	url := "http://metadata.google.internal/computeMetadata/v1"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Debugf("Failed to create GCP http GET request '%s': %s", url, err)
		return false
	}

	req.Header.Add("Metadata-Flavor", "Google")
	return c.httpReqSuccess(req)
}

// azureHttpGetSuccess hits the Azure metadata endpoint in order to detect whether
// the agent is running on Azure.
// https://learn.microsoft.com/en-us/azure/virtual-machines/instance-metadata-service
func (c *fetchConfig) azureHttpGetSuccess() bool {
	url := "http://169.254.169.254/metadata/instance?api-version=2021-02-01"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Debugf("Failed to create Azure http GET request '%s': %s", url, err)
		return false
	}

	req.Header.Add("Metadata", "true")
	return c.httpReqSuccess(req)
}

// azureGetSuccess performs an http request, returning true if the status code
// is 200.
func (c *fetchConfig) httpReqSuccess(req *http.Request) bool {
	resp, err := c.httpDo(req.WithContext(c.ctx))
	if err != nil {
		log.Debugf("Failed to perform http GET request: %s", err)
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// exec runs a command and validates its output using the parse function.
func (c *fetchConfig) exec(name string, args ...string) (string, error) {
	out, err := c.execCommand(name, args...)
	if err != nil {
		log.Debugf("Failed to execute command '%s': %s", name, err)
		return "", err
	}
	return string(out), nil
}

// read reads a read and validates its content using the parse function.
func (c *fetchConfig) read(name string) (string, error) {
	out, err := c.readFile(name)
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
