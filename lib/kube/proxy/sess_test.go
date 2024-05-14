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
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/gravitational/teleport"
	kubewaitingcontainerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
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

func Test_session_trackSession(t *testing.T) {
	t.Parallel()
	moderatedPolicy := &types.SessionTrackerPolicySet{
		Version: types.V3,
		Name:    "name",
		RequireSessionJoin: []*types.SessionRequirePolicy{
			{
				Name:   "Auditor oversight",
				Filter: fmt.Sprintf("contains(user.spec.roles, %q)", "test"),
				Kinds:  []string{"k8s"},
				Modes:  []string{string(types.SessionModeratorMode)},
				Count:  1,
			},
		},
	}
	nonModeratedPolicy := &types.SessionTrackerPolicySet{
		Version: types.V3,
		Name:    "name",
	}
	type args struct {
		authClient auth.ClientI
		policies   []*types.SessionTrackerPolicySet
	}
	tests := []struct {
		name      string
		args      args
		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "ok with moderated session and healthy auth service",
			args: args{
				authClient: &mockSessionTrackerService{},
				policies: []*types.SessionTrackerPolicySet{
					moderatedPolicy,
				},
			},
			assertErr: require.NoError,
		},
		{
			name: "ok with non-moderated session session and healthy auth service",
			args: args{
				authClient: &mockSessionTrackerService{},
				policies: []*types.SessionTrackerPolicySet{
					nonModeratedPolicy,
				},
			},
			assertErr: require.NoError,
		},
		{
			name: "fail with moderated session and unhealthy auth service",
			args: args{
				authClient: &mockSessionTrackerService{
					returnErr: true,
				},
				policies: []*types.SessionTrackerPolicySet{
					moderatedPolicy,
				},
			},
			assertErr: require.Error,
		},
		{
			name: "ok with non-moderated session session and unhealthy auth service",
			args: args{
				authClient: &mockSessionTrackerService{
					returnErr: true,
				},
				policies: []*types.SessionTrackerPolicySet{
					nonModeratedPolicy,
				},
			},
			assertErr: require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sess := &session{
				log: logrus.New().WithField(teleport.ComponentKey, "test"),
				id:  uuid.New(),
				req: &http.Request{
					URL: &url.URL{},
				},
				podName:         "podName",
				accessEvaluator: auth.NewSessionAccessEvaluator(tt.args.policies, types.KubernetesSessionKind, "username"),
				ctx: authContext{
					Context: authz.Context{
						User: &types.UserV2{
							Metadata: types.Metadata{
								Name: "username",
							},
						},
					},
					teleportCluster: teleportClusterClient{
						name: "name",
					},
					kubeClusterName: "kubeClusterName",
				},
				forwarder: &Forwarder{
					cfg: ForwarderConfig{
						Clock:             clockwork.NewFakeClock(),
						AuthClient:        tt.args.authClient,
						CachingAuthClient: tt.args.authClient,
					},
					ctx: context.Background(),
				},
			}
			p := &party{
				Ctx: sess.ctx,
			}
			err := sess.trackSession(p, tt.args.policies)
			tt.assertErr(t, err)
		})
	}
}

type mockSessionTrackerService struct {
	auth.ClientI
	returnErr bool
}

func (m *mockSessionTrackerService) CreateSessionTracker(ctx context.Context, tracker types.SessionTracker) (types.SessionTracker, error) {
	if m.returnErr {
		return nil, trace.ConnectionProblem(nil, "mock error")
	}
	return tracker, nil
}

func (m *mockSessionTrackerService) ListKubernetesWaitingContainers(ctx context.Context, pageSize int, pageToken string) ([]*kubewaitingcontainerpb.KubernetesWaitingContainer, string, error) {
	return nil, "", nil
}
