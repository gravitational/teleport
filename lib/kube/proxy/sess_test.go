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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/remotecommand"

	kubewaitingcontainerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/moderation"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	testingkubemock "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestSessionEndError(t *testing.T) {
	t.Parallel()
	const (
		errorMessage = "request denied"
		errorCode    = http.StatusForbidden
	)
	recordingErr := errors.New("recording err")

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

	tests := []struct {
		name         string
		recordingErr error
		interactive  bool
	}{
		{
			name:        "interactive without recording error",
			interactive: true,
		},
		{
			name:        "non-interactive without recording error",
			interactive: false,
		},
		{
			name:         "interactive with recording error",
			interactive:  true,
			recordingErr: recordingErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var (
				eventsResult      []apievents.AuditEvent
				eventsResultMutex sync.Mutex
			)
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
					CreateAuditStreamErr: tt.recordingErr,
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
				stdout         = &bytes.Buffer{}
				stdinReader, _ = io.Pipe()
			)

			t.Cleanup(func() { stdinReader.Close() })
			streamOpts := remotecommand.StreamOptions{
				Stdin:  stdinReader,
				Stdout: stdout,
				Stderr: nil,
				Tty:    tt.interactive,
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

			if tt.recordingErr == nil {
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
						require.True(t, ok)
						require.Equal(t, events.ExecFailureCode, execEvent.GetCode())
						if tt.recordingErr == nil {
							require.Equal(t, strconv.Itoa(errorCode), execEvent.ExitCode)
							require.Equal(t, errorMessage, execEvent.Error)
						} else {
							require.Empty(t, execEvent.ExitCode)
							require.Equal(t, tt.recordingErr.Error(), execEvent.Error)
						}
						hasSessionExecEvent = true
					}
					require.Truef(t, hasSessionEndEvent, "session end event not found in audit log")
					require.Truef(t, hasSessionExecEvent, "session exec event not found in audit log")
				}, 10*time.Second, 1*time.Second)
			} else {
				require.Never(t, func() bool {
					eventsResultMutex.Lock()
					defer eventsResultMutex.Unlock()
					return len(eventsResult) > 0
				}, 1*time.Second, 100*time.Millisecond)
			}
		})
	}
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
		authClient authclient.ClientI
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
				log: logtest.NewLogger(),
				id:  uuid.New(),
				req: &http.Request{
					URL: &url.URL{
						RawQuery: "command=command&command=arg1&command=arg2",
					},
				},
				podName:         "podName",
				podNamespace:    "podNamespace",
				accessEvaluator: moderation.NewSessionAccessEvaluator(tt.args.policies, types.KubernetesSessionKind, "username"),
				ctx: authContext{
					ScopedContext: authz.ScopedContextFromUnscopedContext(&authz.Context{
						User: &types.UserV2{
							Metadata: types.Metadata{
								Name: "username",
							},
						},
					}),
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
			if err != nil {
				return
			}
			tracker := tt.args.authClient.(*mockSessionTrackerService).tracker
			require.Equal(t, "username", tracker.GetHostUser())
			require.Equal(t, "name", tracker.GetClusterName())
			require.Equal(t, "kubeClusterName", tracker.GetKubeCluster())
			require.Equal(t, sess.id.String(), tracker.GetSessionID())
			require.Equal(t, []string{"command", "arg1", "arg2"}, tracker.GetCommand())
			require.Equal(t, "podNamespace/podName", tracker.GetHostname())
			require.Equal(t, types.KubernetesSessionKind, tracker.GetSessionKind())

		})
	}
}

type mockSessionTrackerService struct {
	authclient.ClientI
	returnErr bool
	tracker   types.SessionTracker
}

func (m *mockSessionTrackerService) CreateSessionTracker(_ context.Context, tracker types.SessionTracker) (types.SessionTracker, error) {
	m.tracker = tracker
	if m.returnErr {
		return nil, trace.ConnectionProblem(nil, "mock error")
	}
	return tracker, nil
}

func (m *mockSessionTrackerService) ListKubernetesWaitingContainers(ctx context.Context, pageSize int, pageToken string) ([]*kubewaitingcontainerpb.KubernetesWaitingContainer, string, error) {
	return nil, "", nil
}

// TestMultiResizeQueueNextEvictsClosedChannel is a regression test for the CPU hot-spin in (*multiResizeQueue).Next():
// a closed party channel must be dropped from the select set, not re-selected forever, and must not wedge other parties.
// See https://github.com/gravitational/teleport/issues/68140.
func TestMultiResizeQueueNextEvictsClosedChannel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		q := newMultiResizeQueue(t.Context())
		q.callback = func(terminalResizeMessage) {}

		// A disconnected party: its resize channel is closed but never removed.
		closedCh := make(chan terminalResizeMessage)
		close(closedCh)
		q.add("disconnected-party", closedCh)

		// A live party's resize must still come through with the dead channel present.
		live := make(chan terminalResizeMessage, 1)
		q.add("live-party", live)
		size := &remotecommand.TerminalSize{Width: 80, Height: 24}
		live <- terminalResizeMessage{size: size, source: uuid.New()}
		require.Equal(t, size, q.Next())

		// The closed channel's forwarder cleaned itself up; only the live party remains.
		synctest.Wait()
		q.mutex.Lock()
		defer q.mutex.Unlock()
		require.Len(t, q.cancels, 1)
	})
}

// TestMultiResizeQueueDeliversResize verifies a resize sent by a party is
// returned by Next(), recorded as the last size, and passed to the callback.
func TestMultiResizeQueueDeliversResize(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		q := newMultiResizeQueue(ctx)
		var got terminalResizeMessage
		q.callback = func(m terminalResizeMessage) { got = m }

		require.Nil(t, q.getLastSize())

		ch := make(chan terminalResizeMessage, 1)
		q.add("party", ch)

		source := uuid.New()
		size := &remotecommand.TerminalSize{Width: 80, Height: 24}
		ch <- terminalResizeMessage{size: size, source: source}

		require.Equal(t, size, q.Next())
		require.Equal(t, size, q.getLastSize())
		require.Equal(t, size, got.size)
		require.Equal(t, source, got.source)
	})
}

// TestMultiResizeQueueMultipleParties verifies resizes from several parties are
// all delivered.
func TestMultiResizeQueueMultipleParties(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		q := newMultiResizeQueue(ctx)
		q.callback = func(terminalResizeMessage) {}

		chA := make(chan terminalResizeMessage, 1)
		chB := make(chan terminalResizeMessage, 1)
		q.add("a", chA)
		q.add("b", chB)

		sizeA := &remotecommand.TerminalSize{Width: 80, Height: 24}
		sizeB := &remotecommand.TerminalSize{Width: 120, Height: 40}
		chA <- terminalResizeMessage{size: sizeA, source: uuid.New()}
		chB <- terminalResizeMessage{size: sizeB, source: uuid.New()}

		got := []*remotecommand.TerminalSize{q.Next(), q.Next()}
		require.ElementsMatch(t, []*remotecommand.TerminalSize{sizeA, sizeB}, got)
	})
}

// TestMultiResizeQueueDynamicAdd verifies Next() picks up a channel added while
// it is already blocked waiting.
func TestMultiResizeQueueDynamicAdd(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		q := newMultiResizeQueue(ctx)
		q.callback = func(terminalResizeMessage) {}

		// An existing party with no pending resize: Next() blocks on it.
		q.add("existing", make(chan terminalResizeMessage))

		result := make(chan *remotecommand.TerminalSize, 1)
		go func() { result <- q.Next() }()
		synctest.Wait()

		// A second party joins mid-session and sends a resize.
		ch := make(chan terminalResizeMessage, 1)
		size := &remotecommand.TerminalSize{Width: 100, Height: 40}
		ch <- terminalResizeMessage{size: size, source: uuid.New()}
		q.add("late", ch)

		require.Equal(t, size, <-result)
	})
}

// TestMultiResizeQueueRemove verifies a removed party leaves the queue while the
// remaining parties keep delivering.
func TestMultiResizeQueueRemove(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		q := newMultiResizeQueue(ctx)
		q.callback = func(terminalResizeMessage) {}

		chA := make(chan terminalResizeMessage, 1)
		chB := make(chan terminalResizeMessage, 1)
		q.add("a", chA)
		q.add("b", chB)
		q.remove("a")

		sizeB := &remotecommand.TerminalSize{Width: 120, Height: 40}
		chB <- terminalResizeMessage{size: sizeB, source: uuid.New()}

		require.Equal(t, sizeB, q.Next())
	})
}

// TestMultiResizeQueueCancelReturnsNil verifies Next() returns nil once the
// parent context is canceled.
func TestMultiResizeQueueCancelReturnsNil(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		q := newMultiResizeQueue(ctx)
		q.callback = func(terminalResizeMessage) {}
		q.add("party", make(chan terminalResizeMessage))

		result := make(chan *remotecommand.TerminalSize, 1)
		go func() { result <- q.Next() }()
		synctest.Wait()

		cancel()
		require.Nil(t, <-result)
	})
}
