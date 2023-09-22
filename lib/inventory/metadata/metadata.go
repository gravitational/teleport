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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/defaults"
)

// Metadata contains the instance "system" metadata.
// We expect each of these values to not change for the lifetime of the instance.
type Metadata struct {
	// OS advertises the instance OS ("darwin" or "linux").
	OS string
	// OSVersion advertises the instance OS version (e.g. "ubuntu 22.04").
	OSVersion string
	// HostArchitecture advertises the instance host architecture (e.g. "x86_64" or "arm64").
	HostArchitecture string
	// GlibcVersion advertises the instance glibc version of linux instances (e.g. "2.35").
	GlibcVersion string
	// InstallMethods advertises the install methods used for the instance (e.g. "dockerfile").
	InstallMethods []string
	// ContainerRuntime advertises the container runtime for the instance, if any (e.g. "docker").
	ContainerRuntime string
	// ContainerOrchestrator advertises the container orchestrator for the instance, if any
	// (e.g. "kubernetes-v1.24.8-eks-ffeb93d").
	ContainerOrchestrator string
	// CloudEnvironment advertises the cloud environment for the instance, if any (e.g. "aws").
	CloudEnvironment string
}

// fetchConfig contains the configuration used by the FetchMetadata method.
type fetchConfig struct {
	context context.Context
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
	httpDo func(req *http.Request, insecureSkipVerify bool) (*http.Response, error)
}

// setDefaults sets the values of several methods used to read files, execute
// commands, performing http requests, etc.
// Having these methods configurable allows us to mock them in tests.
func (c *fetchConfig) setDefaults() {
	if c.context == nil {
		c.context = context.Background()
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
		c.httpDo = func(req *http.Request, insecureSkipVerify bool) (*http.Response, error) {
			transport, err := defaults.Transport()
			if err != nil {
				return nil, trace.Wrap(err)
			}

			// Initialize transport.TLSClientConfig if defaults.Transport() returns a nil one
			if transport.TLSClientConfig == nil {
				transport.TLSClientConfig = &tls.Config{}
			}

			transport.TLSClientConfig.InsecureSkipVerify = insecureSkipVerify
			client := &http.Client{
				Transport: transport,
				Timeout:   5 * time.Second,
			}
			return client.Do(req)
		}
	}
}

// fetch fetches all metadata.
func (c *fetchConfig) fetch() *Metadata {
	return &Metadata{
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
func (c *fetchConfig) fetchOS() string {
	return runtime.GOOS
}

// fetchHostArchitecture returns the value of GOARCH.
func (c *fetchConfig) fetchHostArchitecture() string {
	return runtime.GOARCH
}

// fetchInstallMethods returns the list of methods used to install the instance.
func (c *fetchConfig) fetchInstallMethods() []string {
	installMethods := []string{}
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

// dockerfileInstallMethod returns true if the instance was installed using our
// Dockerfile.
func (c *fetchConfig) dockerfileInstallMethod() bool {
	return c.boolEnvIsTrue("TELEPORT_INSTALL_METHOD_DOCKERFILE")
}

// helmKubeAgentInstallMethod returns true if the instance was installed using our
// Helm chart.
func (c *fetchConfig) helmKubeAgentInstallMethod() bool {
	return c.boolEnvIsTrue("TELEPORT_INSTALL_METHOD_HELM_KUBE_AGENT")
}

// nodeScriptInstallMethod returns true if the instance was installed using our
// install-node.sh script.
func (c *fetchConfig) nodeScriptInstallMethod() bool {
	return c.boolEnvIsTrue("TELEPORT_INSTALL_METHOD_NODE_SCRIPT")
}

// systemctlInstallMethod returns true if the instance is running using systemctl.
func (c *fetchConfig) systemctlInstallMethod() bool {
	out, err := c.exec("systemctl", "status", "teleport.service")
	if err != nil {
		return false
	}

	return strings.Contains(out, "active (running)")
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

// fetchContainerOrchestrator returns kubernetes-${GIT_VERSION} if the instance is
// running on kubernetes.
// This function performs the equivalent of the following:
// curl -k https://$KUBERNETES_SERVICE_HOST:$KUBERNETES_SERVICE_PORT/version | jq .gitVersion
func (c *fetchConfig) fetchContainerOrchestrator() string {
	host := c.getenv("KUBERNETES_SERVICE_HOST")
	port := c.getenv("KUBERNETES_SERVICE_PORT")
	if host == "" || port == "" {
		return ""
	}

	url := fmt.Sprintf("https://%s:%s/version", host, port)
	req, err := http.NewRequestWithContext(c.context, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}

	const insecureSkipVerify = true
	resp, err := c.httpDo(req, insecureSkipVerify)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	var version struct {
		GitVersion string `json:"gitVersion"`
	}
	if err := json.Unmarshal(body, &version); err != nil {
		return ""
	}
	if version.GitVersion == "" {
		return ""
	}

	return fmt.Sprintf("kubernetes-%s", version.GitVersion)
}

// fetchCloudEnvironment returns aws, gpc or azure if the instance is running on
// such cloud environments.
func (c *fetchConfig) fetchCloudEnvironment() string {
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
// the instance is running on AWS.
// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-retrieval.html
func (c *fetchConfig) awsHTTPGetSuccess() bool {
	url := "http://169.254.169.254/latest/meta-data/"
	req, err := http.NewRequestWithContext(c.context, http.MethodGet, url, nil)
	if err != nil {
		return false
	}

	return c.httpReqSuccess(req)
}

// gcpHTTPGetSuccess hits the GCP metadata endpoint in order to detect whether
// the instance is running on GCP.
// https://cloud.google.com/compute/docs/metadata/overview#parts-of-a-request
func (c *fetchConfig) gcpHTTPGetSuccess() bool {
	url := "http://metadata.google.internal/computeMetadata/v1"
	req, err := http.NewRequestWithContext(c.context, http.MethodGet, url, nil)
	if err != nil {
		return false
	}

	req.Header.Add("Metadata-Flavor", "Google")
	return c.httpReqSuccess(req)
}

// azureHTTPGetSuccess hits the Azure metadata endpoint in order to detect whether
// the instance is running on Azure.
// https://learn.microsoft.com/en-us/azure/virtual-machines/instance-metadata-service
func (c *fetchConfig) azureHTTPGetSuccess() bool {
	url := "http://169.254.169.254/metadata/instance?api-version=2021-02-01"
	req, err := http.NewRequestWithContext(c.context, http.MethodGet, url, nil)
	if err != nil {
		return false
	}

	req.Header.Add("Metadata", "true")
	return c.httpReqSuccess(req)
}

// exec runs a command and returns its output.
func (c *fetchConfig) exec(name string, args ...string) (string, error) {
	out, err := c.execCommand(name, args...)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

// read reads a file and returns its content.
func (c *fetchConfig) read(name string) (string, error) {
	out, err := c.readFile(name)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

// httpReqSuccess performs an http request, returning true if the status code
// is 200.
func (c *fetchConfig) httpReqSuccess(req *http.Request) bool {
	const insecureSkipVerify = false
	resp, err := c.httpDo(req, insecureSkipVerify)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// boolEnvIsTrue returns true if the environment variable is set to a value
// that represent true (e.g. true, yes, y, ...).
func (c *fetchConfig) boolEnvIsTrue(name string) bool {
	b, err := utils.ParseBool(c.getenv(name))
	if err != nil {
		return false
	}
	return b
}
