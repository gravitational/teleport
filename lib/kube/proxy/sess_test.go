/*
Copyright 2021 Gravitational, Inc.

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
	"io"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/remotecommand"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	testingkubemock "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
)

func TestSessionEndError(t *testing.T) {
	t.Parallel()
	var (
		eventsResult      []apievents.AuditEvent
		eventsResultMutex sync.Mutex
	)
	const (
		errorMessage = "request denied"
		errorCode    = http.StatusForbidden
	)
	kubeMock, err := testingkubemock.NewKubeAPIMock(
		testingkubemock.WithExecError(
			metav1.Status{
				Status:  metav1.StatusFailure,
				Message: errorMessage,
				Reason:  metav1.StatusReasonForbidden,
				Code:    errorCode,
			},
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { kubeMock.Close() })

	// creates a Kubernetes service with a configured cluster pointing to mock api server
	testCtx := SetupTestContext(
		context.Background(),
		t,
		TestConfig{
			Clusters: []KubeClusterConfig{{Name: kubeCluster, APIEndpoint: kubeMock.URL}},
			// collect all audit events
			OnEvent: func(event apievents.AuditEvent) {
				eventsResultMutex.Lock()
				defer eventsResultMutex.Unlock()
				eventsResult = append(eventsResult, event)
			},
		},
	)

	t.Cleanup(func() { require.NoError(t, testCtx.Close()) })

	// create a user with access to kubernetes (kubernetes_user and kubernetes_groups specified)
	user, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		username,
		RoleSpec{
			Name:       roleName,
			KubeUsers:  roleKubeUsers,
			KubeGroups: roleKubeGroups,
		})

	// generate a kube client with user certs for auth
	_, userRestConfig := testCtx.GenTestKubeClientTLSCert(
		t,
		user.GetName(),
		kubeCluster,
	)
	require.NoError(t, err)

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

	exec, err := remotecommand.NewSPDYExecutor(userRestConfig, http.MethodPost, req.URL())
	require.NoError(t, err)
	err = exec.StreamWithContext(testCtx.Context, streamOpts)
	require.Error(t, err)

	// check that the session is ended with an error in audit log.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		eventsResultMutex.Lock()
		defer eventsResultMutex.Unlock()
		hasSessionEndEvent := false
		hasSessionExecEvent := false
		for _, event := range eventsResult {
			if event.GetType() == events.SessionEndEvent {
				hasSessionEndEvent = true
			}
			if event.GetType() != events.ExecEvent {
				continue
			}

			execEvent, ok := event.(*apievents.Exec)
			assert.True(t, ok)
			assert.Equal(t, events.ExecFailureCode, execEvent.GetCode())
			assert.Equal(t, strconv.Itoa(errorCode), execEvent.ExitCode)
			assert.Equal(t, errorMessage, execEvent.Error)
			hasSessionExecEvent = true
		}
		assert.Truef(t, hasSessionEndEvent, "session end event not found in audit log")
		assert.Truef(t, hasSessionExecEvent, "session exec event not found in audit log")
	}, 10*time.Second, 1*time.Second)
}
