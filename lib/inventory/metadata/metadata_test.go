/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package metadata

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestFetchInstallMethods(t *testing.T) {
	testCases := []struct {
		desc        string
		setupEnv    func(t *testing.T)
		getenv      func(string) string
		execCommand func(string, ...string) ([]byte, error)
		expected    []string
	}{
		{
			desc: "dockerfile if dockerfile",
			getenv: func(name string) string {
				if name == "TELEPORT_INSTALL_METHOD_DOCKERFILE" {
					return "true"
				}
				return ""
			},
			execCommand: func(name string, args ...string) ([]byte, error) {
				return nil, trace.NotFound("command does not exist")
			},
			expected: []string{
				"dockerfile",
			},
		},
		{
			desc: "helm_kube_agent if helm",
			getenv: func(name string) string {
				if name == "TELEPORT_INSTALL_METHOD_HELM_KUBE_AGENT" {
					return "true"
				}
				return ""
			},
			execCommand: func(name string, args ...string) ([]byte, error) {
				return nil, trace.NotFound("command does not exist")
			},
			expected: []string{
				"helm_kube_agent",
			},
		},
		{
			desc: "node_script if node script",
			getenv: func(name string) string {
				if name == "TELEPORT_INSTALL_METHOD_NODE_SCRIPT" {
					return "true"
				}
				return ""
			},
			execCommand: func(name string, args ...string) ([]byte, error) {
				return nil, trace.NotFound("command does not exist")
			},
			expected: []string{
				"node_script",
			},
		},
		{
			desc: "awsoidc_deployservice if env var is present",
			getenv: func(name string) string {
				return ""
			},
			setupEnv: func(t *testing.T) {
				t.Setenv(types.InstallMethodAWSOIDCDeployServiceEnvVar, "yes")
			},
			execCommand: func(name string, args ...string) ([]byte, error) {
				return nil, trace.NotFound("command does not exist")
			},
			expected: []string{
				"awsoidc_deployservice",
			},
		},
		{
			desc: "systemctl if systemctl",
			getenv: func(name string) string {
				return ""
			},
			execCommand: func(name string, args ...string) ([]byte, error) {
				if name != "systemctl" {
					return nil, trace.NotFound("command does not exist")
				}
				if len(args) != 2 {
					return nil, trace.NotFound("command does not exist")
				}
				if args[0] != "status" || args[1] != "teleport.service" {
					return nil, trace.NotFound("command does not exist")
				}
				output := `
● teleport.service - Teleport Service
Loaded: loaded (/lib/systemd/system/teleport.service; enabled; vendor preset: enabled)
Active: active (running) since Wed 2022-11-09 10:52:49 UTC; 3 months 22 days ago
Main PID: 1815 (teleport)
	Tasks: 12 (limit: 1143)
Memory: 55.6M
	CPU: 2h 2min 27.181s
CGroup: /system.slice/teleport.service
		└─1815 /usr/local/bin/teleport start --pid-file=/run/teleport.pid
`
				return []byte(output), nil
			},
			expected: []string{
				"systemctl",
			},
		},
		{
			desc: "dockerfile and helm_kube_agent if dockerfile and helm",
			getenv: func(name string) string {
				if name == "TELEPORT_INSTALL_METHOD_DOCKERFILE" {
					return "true"
				}
				if name == "TELEPORT_INSTALL_METHOD_HELM_KUBE_AGENT" {
					return "true"
				}
				return ""
			},
			execCommand: func(name string, args ...string) ([]byte, error) {
				return nil, trace.NotFound("command does not exist")
			},
			expected: []string{
				"dockerfile",
				"helm_kube_agent",
			},
		},
		{
			desc: "empty if none",
			getenv: func(name string) string {
				return ""
			},
			execCommand: func(name string, args ...string) ([]byte, error) {
				return nil, trace.NotFound("command does not exist")
			},
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.setupEnv != nil {
				tc.setupEnv(t)
			}
			c := &fetchConfig{
				getenv:      tc.getenv,
				execCommand: tc.execCommand,
			}
			require.Equal(t, tc.expected, c.fetchInstallMethods())
		})
	}
}

func TestFetchContainerRuntime(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc     string
		readFile func(string) ([]byte, error)
		expected string
	}{
		{
			desc: "docker if /.dockerenv exists",
			readFile: func(name string) ([]byte, error) {
				if name != "/.dockerenv" {
					return nil, trace.NotFound("file does not exist")
				}
				return []byte{}, nil
			},
			expected: "docker",
		},
		{
			desc: "empty if /.dockerenv does not exist",
			readFile: func(name string) ([]byte, error) {
				return nil, trace.NotFound("file does not exist")
			},
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			c := &fetchConfig{
				readFile: tc.readFile,
			}
			require.Equal(t, tc.expected, c.fetchContainerRuntime())
		})
	}
}

func TestFetchContainerOrchestrator(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc     string
		getenv   func(string) string
		httpDo   func(*http.Request, bool) (*http.Response, error)
		expected string
	}{
		{
			desc: "kubernetes with git version if on kubernetes",
			getenv: func(name string) string {
				if name == "KUBERNETES_SERVICE_HOST" {
					return "172.20.0.1"
				}
				if name == "KUBERNETES_SERVICE_PORT" {
					return "443"
				}
				return ""
			},
			httpDo: func(req *http.Request, insecureSkipVerify bool) (*http.Response, error) {
				if !insecureSkipVerify {
					return nil, trace.BadParameter("insecureSkipVerify should be true")
				}
				if req.URL.String() != "https://172.20.0.1:443/version" {
					return nil, trace.NotFound("not found")
				}

				body := `
				{
					"major": "1",
					"minor": "23+",
					"gitVersion": "v1.23.14-eks-ffeb93d",
					"gitCommit": "96e7d52c98a32f2b296ca7f19dc9346cf79915ba",
					"gitTreeState": "clean",
					"buildDate": "2022-11-29T18:43:31Z",
					"goVersion": "go1.17.13",
					"compiler": "gc",
					"platform": "linux/amd64"
				}
				`
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			},
			expected: "kubernetes-v1.23.14-eks-ffeb93d",
		},
		{
			desc: "empty if not on kubernetes",
			getenv: func(name string) string {
				return ""
			},
			httpDo: func(req *http.Request, insecureSkipVerify bool) (*http.Response, error) {
				return nil, trace.NotFound("not found")
			},
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			c := &fetchConfig{
				getenv: tc.getenv,
				httpDo: tc.httpDo,
			}
			require.Equal(t, tc.expected, c.fetchContainerOrchestrator(context.Background()))
		})
	}
}

func TestFetchCloudEnvironment(t *testing.T) {
	t.Parallel()

	success := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("")),
	}

	testCases := []struct {
		desc     string
		httpDo   func(*http.Request, bool) (*http.Response, error)
		expected string
	}{
		{
			desc: "aws if on aws",
			httpDo: func(req *http.Request, insecureSkipVerify bool) (*http.Response, error) {
				if insecureSkipVerify {
					return nil, trace.BadParameter("insecureSkipVerify should be false")
				}

				if req.URL.String() == "http://169.254.169.254/latest/api/token" {
					if req.Method != http.MethodPut {
						return nil, trace.NotFound("not found")
					}
					if len(req.Header) != 1 {
						return nil, trace.NotFound("not found")
					}
					if len(req.Header["X-Aws-Ec2-Metadata-Token-Ttl-Seconds"]) != 1 {
						return nil, trace.NotFound("not found")
					}
					if req.Header["X-Aws-Ec2-Metadata-Token-Ttl-Seconds"][0] != "300" {
						return nil, trace.NotFound("not found")
					}
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(strings.NewReader("thisIsAFakeTestToken")),
					}, nil
				}

				if req.URL.String() == "http://169.254.169.254/latest/meta-data/" {
					if len(req.Header) != 1 {
						return nil, trace.NotFound("not found")
					}
					if len(req.Header["X-Aws-Ec2-Metadata-Token"]) != 1 {
						return nil, trace.NotFound("not found")
					}
					if req.Header["X-Aws-Ec2-Metadata-Token"][0] != "thisIsAFakeTestToken" {
						return nil, trace.NotFound("not found")
					}
					return success, nil
				}

				return nil, trace.NotFound("not found")
			},
			expected: "aws",
		},
		{
			desc: "gcp if on gcp",
			httpDo: func(req *http.Request, insecureSkipVerify bool) (*http.Response, error) {
				if insecureSkipVerify {
					return nil, trace.BadParameter("insecureSkipVerify should be false")
				}
				if req.URL.String() != "http://metadata.google.internal/computeMetadata/v1" {
					return nil, trace.NotFound("not found")
				}
				if len(req.Header) != 1 {
					return nil, trace.NotFound("not found")
				}
				if len(req.Header["Metadata-Flavor"]) != 1 {
					return nil, trace.NotFound("not found")
				}
				if req.Header["Metadata-Flavor"][0] != "Google" {
					return nil, trace.NotFound("not found")
				}
				return success, nil
			},
			expected: "gcp",
		},
		{
			desc: "azure if on azure",
			httpDo: func(req *http.Request, insecureSkipVerify bool) (*http.Response, error) {
				if insecureSkipVerify {
					return nil, trace.BadParameter("insecureSkipVerify should be false")
				}
				if req.URL.String() != "http://169.254.169.254/metadata/instance?api-version=2021-02-01" {
					return nil, trace.NotFound("not found")
				}
				if len(req.Header) != 1 {
					return nil, trace.NotFound("not found")
				}
				if len(req.Header["Metadata"]) != 1 {
					return nil, trace.NotFound("not found")
				}
				if req.Header["Metadata"][0] != "true" {
					return nil, trace.NotFound("not found")
				}
				return success, nil
			},
			expected: "azure",
		},
		{
			desc: "empty if not aws, gcp nor azure",
			httpDo: func(req *http.Request, insecureSkipVerify bool) (*http.Response, error) {
				if insecureSkipVerify {
					return nil, trace.BadParameter("insecureSkipVerify should be false")
				}
				return nil, trace.NotFound("not found")
			},
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			c := &fetchConfig{
				httpDo: tc.httpDo,
			}
			require.Equal(t, tc.expected, c.fetchCloudEnvironment(context.Background()))
		})
	}
}
