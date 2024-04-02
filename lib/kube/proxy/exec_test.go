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

package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"

	"github.com/gravitational/teleport"
	testingkubemock "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
)

var (
	kubeCluster              = "test_cluster"
	username                 = "test_user"
	roleName                 = "kube_role"
	usernameMultiUsers       = "test_user_multi_users"
	roleNameMultiUsers       = "kube_role_multi_users"
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
	testCtx := SetupTestContext(
		context.Background(),
		t,
		TestConfig{
			Clusters: []KubeClusterConfig{{Name: kubeCluster, APIEndpoint: kubeMock.URL}},
		},
	)

	t.Cleanup(func() { require.NoError(t, testCtx.Close()) })

	// create a user with access to kubernetes (kubernetes_user and kubernetes_groups specified)
	userWithSingleKubeUser, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		username,
		RoleSpec{
			Name:       roleName,
			KubeUsers:  roleKubeUsers,
			KubeGroups: roleKubeGroups,
		})

	// generate a kube client with user certs for auth
	_, configWithSingleKubeUser := testCtx.GenTestKubeClientTLSCert(
		t,
		userWithSingleKubeUser.GetName(),
		kubeCluster,
	)
	require.NoError(t, err)

	// create a user with access to kubernetes (kubernetes_user and kubernetes_groups specified)
	userMultiKubeUsers, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		usernameMultiUsers,
		RoleSpec{
			Name:       roleNameMultiUsers,
			KubeUsers:  append(roleKubeUsers, "admin"),
			KubeGroups: roleKubeGroups,
		})

	// generate a kube client with user certs for auth
	_, configMultiKubeUsers := testCtx.GenTestKubeClientTLSCert(
		t,
		userMultiKubeUsers.GetName(),
		kubeCluster,
	)
	require.NoError(t, err)

	type args struct {
		executorBuilder func(*rest.Config, string, *url.URL) (remotecommand.Executor, error)
		impersonateUser string
		config          *rest.Config
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "SPDY protocol",
			args: args{
				executorBuilder: remotecommand.NewSPDYExecutor,
				config:          configWithSingleKubeUser,
			},
		},
		{
			name: "Websocket protocol v4",
			args: args{
				// We can delete the dummy client once https://github.com/kubernetes/kubernetes/pull/110142
				// is merged into k8s go-client.
				// For now go-client does not support connections over websockets.
				executorBuilder: func(c *rest.Config, s string, u *url.URL) (remotecommand.Executor, error) {
					return newWebSocketClient(c, s, u)
				},
				config: configWithSingleKubeUser,
			},
		},
		{
			name: "Websocket protocol v5",
			args: args{
				executorBuilder: func(c *rest.Config, s string, u *url.URL) (remotecommand.Executor, error) {
					return remotecommand.NewWebSocketExecutor(c, s, u.String())
				},
				config: configWithSingleKubeUser,
			},
		},
		{
			name: "SPDY protocol for user with multiple kubernetes users",
			args: args{
				executorBuilder: remotecommand.NewSPDYExecutor,
				config:          configMultiKubeUsers,
				impersonateUser: "admin",
			},
		},
		{
			name: "Websocket protocol v4 for user with multiple kubernetes users",
			args: args{
				// We can delete the dummy client once https://github.com/kubernetes/kubernetes/pull/110142
				// is merged into k8s go-client.
				// For now go-client does not support connections over websockets.
				executorBuilder: func(c *rest.Config, s string, u *url.URL) (remotecommand.Executor, error) {
					return newWebSocketClient(c, s, u)
				},
				config:          configMultiKubeUsers,
				impersonateUser: "admin",
			},
		},
		{
			name: "Websocket protocol v5 for user with multiple kubernetes users",
			args: args{
				executorBuilder: func(c *rest.Config, s string, u *url.URL) (remotecommand.Executor, error) {
					return remotecommand.NewWebSocketExecutor(c, s, u.String())
				},
				config:          configMultiKubeUsers,
				impersonateUser: "admin",
			},
		},
		{
			name: "SPDY protocol for user with multiple kubernetes users without specifying impersonate user",
			args: args{
				executorBuilder: remotecommand.NewSPDYExecutor,
				config:          configMultiKubeUsers,
			},
			wantErr: true,
		},
		{
			name: "Websocket protocol v5 for user with multiple kubernetes users without specifying impersonate user",
			args: args{
				executorBuilder: func(c *rest.Config, s string, u *url.URL) (remotecommand.Executor, error) {
					return remotecommand.NewWebSocketExecutor(c, s, u.String())
				},
				config: configMultiKubeUsers,
			},
			wantErr: true,
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
				generateExecRequestConfig{
					addr:          testCtx.KubeProxyAddress(),
					podName:       podName,
					podNamespace:  podNamespace,
					containerName: podContainerName,
					cmd:           containerCommmandExecute, // placeholder for commands to execute in the dummy pod
					options:       streamOpts,
				},
			)
			require.NoError(t, err)
			// configure the client to impersonate the user.
			// If empty, the client ignores it.
			tt.args.config.Impersonate.UserName = tt.args.impersonateUser
			exec, err := tt.args.executorBuilder(tt.args.config, http.MethodPost, req.URL())
			require.NoError(t, err)

			err = exec.StreamWithContext(testCtx.Context, streamOpts)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, fmt.Sprintf("%s\n%s", podContainerName, string(stdinContent)), stdout.String())
			require.Equal(t, fmt.Sprintf("%s\n%s", podContainerName, string(stdinContent)), stderr.String())
		})
	}
}

type generateExecRequestConfig struct {
	// addr is the address of the Kube API server.
	addr string
	// podName is the name of the pod to execute the command in.
	podName string
	// podNamespace is the namespace of the pod to execute the command in.
	podNamespace string
	// containerName is the name of the container to execute the command in.
	containerName string
	// cmd is the command to execute in the container.
	cmd []string
	// options are the options for the command execution.
	options remotecommand.StreamOptions
	// reason is the reason for the command execution.
	reason string
	// invite is the list of users to invite.
	invite []string
}

// generateExecRequest generates a Kube API url for executing commands in pods.
// The url format is the following:
// "/api/v1/namespaces/{podNamespace}/pods/{podName}/exec?stderr={stdout}&stdout={stdout}&tty={tty}&reason={reason}&container={containerName}&command={command}"
func generateExecRequest(cfg generateExecRequestConfig) (*rest.Request, error) {
	restClient, err := rest.RESTClientFor(&rest.Config{
		Host:    cfg.addr,
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
			Container: cfg.containerName,
			Command:   cfg.cmd,
			Stdin:     cfg.options.Stdin != nil,
			Stdout:    cfg.options.Stdout != nil,
			Stderr:    cfg.options.Stderr != nil,
			TTY:       cfg.options.Tty,
		}, scheme.ParameterCodec).
		Param(teleport.KubeSessionInvitedQueryParam, strings.Join(cfg.invite, ",")).
		Param(teleport.KubeSessionReasonQueryParam, cfg.reason)

	return req, nil
}
