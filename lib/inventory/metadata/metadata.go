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

package metadata

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/defaults"
)

// AgentMetadataFetchConfig contains the configuration used by the FetchAgentMetadata method.
type AgentMetadataFetchConfig struct {
	Context context.Context
	// getenv is the method called to retrieve an environment
	// variable.
	// It is configurable so that it can be mocked in tests.
	getenv func(name string) string
	// readFile is the method called to read a file.
	// It is configurable so that it can be mocked in tests.
	readFile func(name string) ([]byte, error)
	// execCommand is the method called to execute a command.
	// It is configurable so that it can be mocked in tests.
	execCommand func(name string, args ...string) ([]byte, error)
	// httpDo is the method called to perform an http request.
	// It is configurable so that it can be mocked in tests.
	httpDo func(req *http.Request) (*http.Response, error)
	// kubeClient is a kubernetes client used to retrieve the
	// server version.
	// It is configurable so that it can be mocked in tests.
	kubeClient kubernetes.Interface
}

// setDefaults sets the values of several methods used to read files, execute
// commands, performing http requests, etc.
// Having these methods configurable allows us to mock them in tests.
func (c *AgentMetadataFetchConfig) setDefaults() {
	if c.Context == nil {
		c.Context = context.Background()
	}
	if c.getenv == nil {
		c.getenv = os.Getenv
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
			client, err := defaults.HTTPClient()
			if err != nil {
				return nil, err
			}
			client.Timeout = 5 * time.Second
			return client.Do(req)
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

// FetchAgentMetadata fetches and calculates all agent metadata we are
// interested in tracking. Note that the resulting metadata is not cached.
func FetchAgentMetadata(c *AgentMetadataFetchConfig) proto.UpstreamInventoryAgentMetadata {
	c.setDefaults()
	return *c.fetchMetadata()
}

// fetchMetadata fetches all agent metadata.
func (c *AgentMetadataFetchConfig) fetchMetadata() *proto.UpstreamInventoryAgentMetadata {
	return &proto.UpstreamInventoryAgentMetadata{
		OS:                    c.fetchOS(),
		OSVersion:             c.fetchOSVersion(),
		HostArchitecture:      c.fetchHostArchitecture(),
		GlibcVersion:          c.fetchGlibcVersion(),
		InstallMethods:        c.fetchInstallMethods(),
		ContainerRuntime:      c.fetchContainerRuntime(),
		ContainerOrchestrator: c.fetchContainerOrchestrator(),
		CloudEnvironment:      c.fetchCloudEnvironment(),
	}
}

// fetchOS returns the value of GOOS.
func (c *AgentMetadataFetchConfig) fetchOS() string {
	return runtime.GOOS
}

// fetchHostArchitecture returns the output of arch.
func (c *AgentMetadataFetchConfig) fetchHostArchitecture() string {
	arch, err := c.exec("arch")
	if err != nil {
		return ""
	}

	return arch
}

// fetchInstallMethods returns the list of methods used to install the agent.
func (c *AgentMetadataFetchConfig) fetchInstallMethods() []string {
	var installMethods []string
	if c.dockerfileInstallMethod() {
		installMethods = append(installMethods, "dockerfile")
	}
	if c.helmKubeAgentInstallMethod() {
		installMethods = append(installMethods, "helm_kube_agent")
	}
	if c.nodeScriptInstallMethod() {
		installMethods = append(installMethods, "node_script")
	}
	if c.systemctlInstallMethod() {
		installMethods = append(installMethods, "systemctl")
	}
	return installMethods
}

// dockerfileInstallMethod returns true if the agent was installed using our
// Dockerfile.
func (c *AgentMetadataFetchConfig) dockerfileInstallMethod() bool {
	return c.getenv("TELEPORT_INSTALL_METHOD_DOCKERFILE") == "true"
}

// helmKubeAgentInstallMethod returns true if the agent was installed using our
// Helm chart.
func (c *AgentMetadataFetchConfig) helmKubeAgentInstallMethod() bool {
	return c.getenv("TELEPORT_INSTALL_METHOD_HELM_KUBE_AGENT") == "true"
}

// nodeScriptInstallMethod returns true if the agent was installed using our
// install-node.sh script.
func (c *AgentMetadataFetchConfig) nodeScriptInstallMethod() bool {
	return c.getenv("TELEPORT_INSTALL_METHOD_NODE_SCRIPT") == "true"
}

// systemctlInstallMethod returns true if the agent is running using systemctl.
func (c *AgentMetadataFetchConfig) systemctlInstallMethod() bool {
	out, err := c.exec("systemctl", "status", "teleport.service")
	if err != nil {
		return false
	}

	return strings.Contains(out, "active (running)")
}

// fetchContainerRuntime returns "docker" if the file "/.dockerenv" exists.
func (c *AgentMetadataFetchConfig) fetchContainerRuntime() string {
	_, err := c.read("/.dockerenv")
	if err != nil {
		return ""
	}

	// If the file exists, we should be running on Docker.
	return "docker"
}

// fetchContainerOrchestrator returns kubernetes-${GIT_VERSION} if the agent is
// running on kubernetes.
func (c *AgentMetadataFetchConfig) fetchContainerOrchestrator() string {
	if c.kubeClient == nil {
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
func (c *AgentMetadataFetchConfig) fetchCloudEnvironment() string {
	if c.awsHTTPGetSuccess() {
		return "aws"
	}
	if c.gcpHTTPGetSuccess() {
		return "gcp"
	}
	if c.azureHTTPGetSuccess() {
		return "azure"
	}
	return ""
}

// awsHTTPGetSuccess hits the AWS metadata endpoint in order to detect whether
// the agent is running on AWS.
// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-retrieval.html
func (c *AgentMetadataFetchConfig) awsHTTPGetSuccess() bool {
	url := "http://169.254.169.254/latest/meta-data/"
	req, err := http.NewRequestWithContext(c.Context, http.MethodGet, url, nil)
	if err != nil {
		log.Debugf("Failed to create AWS http GET request '%s': %s", url, err)
		return false
	}

	return c.httpReqSuccess(req)
}

// gcpHTTPGetSuccess hits the GCP metadata endpoint in order to detect whether
// the agent is running on GCP.
// https://cloud.google.com/compute/docs/metadata/overview#parts-of-a-request
func (c *AgentMetadataFetchConfig) gcpHTTPGetSuccess() bool {
	url := "http://metadata.google.internal/computeMetadata/v1"
	req, err := http.NewRequestWithContext(c.Context, http.MethodGet, url, nil)
	if err != nil {
		log.Debugf("Failed to create GCP http GET request '%s': %s", url, err)
		return false
	}

	req.Header.Add("Metadata-Flavor", "Google")
	return c.httpReqSuccess(req)
}

// azureHTTPGetSuccess hits the Azure metadata endpoint in order to detect whether
// the agent is running on Azure.
// https://learn.microsoft.com/en-us/azure/virtual-machines/instance-metadata-service
func (c *AgentMetadataFetchConfig) azureHTTPGetSuccess() bool {
	url := "http://169.254.169.254/metadata/instance?api-version=2021-02-01"
	req, err := http.NewRequestWithContext(c.Context, http.MethodGet, url, nil)
	if err != nil {
		log.Debugf("Failed to create Azure http GET request '%s': %s", url, err)
		return false
	}

	req.Header.Add("Metadata", "true")
	return c.httpReqSuccess(req)
}

// exec runs a command and returns its output.
func (c *AgentMetadataFetchConfig) exec(name string, args ...string) (string, error) {
	out, err := c.execCommand(name, args...)
	if err != nil {
		log.Debugf("Failed to execute command '%s': %s", name, err)
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

// read reads a file and returns its content.
func (c *AgentMetadataFetchConfig) read(name string) (string, error) {
	out, err := c.readFile(name)
	if err != nil {
		log.Debugf("Failed to read file '%s': %s", name, err)
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

// httpReqSuccess performs an http request, returning true if the status code
// is 200.
func (c *AgentMetadataFetchConfig) httpReqSuccess(req *http.Request) bool {
	resp, err := c.httpDo(req)
	if err != nil {
		log.Debugf("Failed to perform http GET request: %s", err)
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
