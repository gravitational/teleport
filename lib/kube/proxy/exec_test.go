/*
Copyright 2022 Gravitational, Inc.

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

package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"

	testingkubemock "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
)

var (
	kubeCluster              = "test_cluster"
	username                 = "test_user"
	roleName                 = "kube_role"
	roleKubeGroups           = []string{"kube"}
	roleKubeUsers            = []string{"kube"}
	podName                  = "teleport"
	podNamespace             = "default"
	podContainerName         = "teleportContainer"
	containerCommmandExecute = []string{"sh"}
	stdinContent             = []byte("stdin_data")
)

func TestExecKubeService(t *testing.T) {
	kubeMock, err := testingkubemock.NewKubeAPIMock()
	require.NoError(t, err)
	t.Cleanup(func() { kubeMock.Close() })

	// creates a Kubernetes service with a configured cluster pointing to mock api server
	testCtx := setupTestContext(
		context.Background(),
		t,
		testConfig{
			clusters: []kubeClusterConfig{{name: kubeCluster, apiEndpoint: kubeMock.URL}},
		},
	)

	t.Cleanup(func() { require.NoError(t, testCtx.Close()) })

	// create a user with access to kubernetes (kubernetes_user and kubernetes_groups specified)
	user, _ := testCtx.createUserAndRole(
		testCtx.ctx,
		t,
		username,
		roleSpec{
			name:       roleName,
			kubeUsers:  roleKubeUsers,
			kubeGroups: roleKubeGroups,
		})

	// generate a kube client with user certs for auth
	_, config := testCtx.genTestKubeClientTLSCert(
		t,
		user.GetName(),
		kubeCluster,
	)
	require.NoError(t, err)

	type args struct {
		executorBuilder func(*rest.Config, string, *url.URL) (remotecommand.Executor, error)
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "SPDY protocol",
			args: args{
				executorBuilder: remotecommand.NewSPDYExecutor,
			},
		},
		{
			name: "Websocket protocol",
			args: args{
				// We can delete the dummy client once https://github.com/kubernetes/kubernetes/pull/110142
				// is merged into k8s go-client.
				// For now go-client does not support connections over websockets.
				executorBuilder: func(c *rest.Config, s string, u *url.URL) (remotecommand.Executor, error) {
					return newWebSocketExecutor(c, s, u)
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				stdinWrite = &bytes.Buffer{}
				stdout     = &bytes.Buffer{}
				stderr     = &bytes.Buffer{}
			)

			_, err = stdinWrite.Write(stdinContent)
			require.NoError(t, err)

			streamOpts := remotecommand.StreamOptions{
				Stdin:  io.NopCloser(stdinWrite),
				Stdout: stdout,
				Stderr: stderr,
				Tty:    false,
			}

			req, err := generateExecRequest(
				testCtx.KubeServiceAddress(),
				podName,
				podNamespace,
				podContainerName,
				containerCommmandExecute, // placeholder for commands to execute in the dummy pod
				streamOpts,
			)
			require.NoError(t, err)

			exec, err := tt.args.executorBuilder(config, http.MethodPost, req.URL())
			require.NoError(t, err)

			err = exec.Stream(streamOpts)
			require.NoError(t, err)

			require.Equal(t, fmt.Sprintf("%s\n%s", podContainerName, string(stdinContent)), stdout.String())
			require.Equal(t, fmt.Sprintf("%s\n%s", podContainerName, string(stdinContent)), stderr.String())
		})
	}
}

// generateExecRequest generates a Kube API url for executing commands in pods.
// The url format is the following:
// "/api/v1/namespaces/{podNamespace}/pods/{podName}/exec?stderr={stdout}&stdout={stdout}&tty={tty}.
func generateExecRequest(addr, podName, podNamespace, containerName string, cmd []string, options remotecommand.StreamOptions) (*rest.Request, error) {
	restClient, err := rest.RESTClientFor(&rest.Config{
		Host:    addr,
		APIPath: "/api",
		ContentConfig: rest.ContentConfig{
			GroupVersion:         &corev1.SchemeGroupVersion,
			NegotiatedSerializer: runtime.NewSimpleNegotiatedSerializer(runtime.SerializerInfo{}),
		},
		TLSClientConfig: rest.TLSClientConfig{Insecure: true},
	})
	if err != nil {
		return nil, err
	}

	req := restClient.Post().
		Resource("pods").
		Name(podName).
		Namespace(podNamespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   cmd,
			Stdin:     options.Stdin != nil,
			Stdout:    options.Stdout != nil,
			Stderr:    options.Stderr != nil,
			TTY:       options.Tty,
		}, scheme.ParameterCodec)

	return req, nil
}
