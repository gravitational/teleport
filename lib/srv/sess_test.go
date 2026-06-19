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

package srv

import (
	"context"
	"crypto/ed25519"
	"io"
	"maps"
	"net"
	"os/user"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	apissh "github.com/gravitational/teleport/api/ssh"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/approval"
	"github.com/gravitational/teleport/lib/sshutils/sftp"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/session/reexec/reexecsftp"
)

func TestIsApprovedFileTransfer(t *testing.T) {
	// set enterprise for tests
	modulestest.SetTestModules(t, modulestest.Modules{TestBuildType: modules.BuildEnterprise})
	srv := newMockServer(t)
	srv.component = teleport.ComponentNode

	// init a session registry
	reg, _ := NewSessionRegistry(SessionRegistryConfig{
		Srv:                   srv,
		SessionTrackerService: srv.auth,
	})
	t.Cleanup(func() { reg.Close() })

	// Create the auditorRole and moderator Party
	auditorRole, _ := types.NewRole("auditor", types.RoleSpecV6{
		Allow: types.RoleConditions{
			JoinSessions: []*types.SessionJoinPolicy{{
				Name:  "foo",
				Roles: []string{"access"},
				Kinds: []string{string(types.SSHSessionKind)},
				Modes: []string{string(types.SessionModeratorMode)},
			}},
		},
	})
	auditorRoleSet := services.NewRoleSet(auditorRole)
	auditScx := newTestServerContext(t, reg.Srv, auditorRoleSet, &decisionpb.SSHAccessPermit{})
	// change the teleport user so we don't match the user in the test cases
	auditScx.Identity.TeleportUser = "mod"
	auditSess, _ := testOpenSession(t, reg, auditorRoleSet, &decisionpb.SSHAccessPermit{})
	approvers := make(map[string]*party)
	clientChan, serverChan := newMockSSHChannel(t)
	clientChan.Drain()

	approvers["mod"] = newParty(auditSess, types.SessionModeratorMode, serverChan, auditScx)

	// create the accessRole to be used for the requester
	accessRole, _ := types.NewRole("access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			RequireSessionJoin: []*types.SessionRequirePolicy{{
				Name:   "foo",
				Filter: "contains(user.roles, \"auditor\")", // escape to avoid illegal rune
				Kinds:  []string{string(types.SSHSessionKind)},
				Modes:  []string{string(types.SessionModeratorMode)},
				Count:  1,
			}},
		},
	})
	accessRoleSet := services.NewRoleSet(accessRole)

	cases := []struct {
		name           string
		expectedResult bool
		expectedError  string
		req            *fileTransferRequestWithApprovers
		reqID          string
		location       string
	}{
		{
			name:           "no pending file request",
			expectedResult: false,
			expectedError:  "Session does not have a pending file transfer request",
			reqID:          "",
			req:            nil,
		},
		{
			name:           "current requester does not match original requester",
			expectedResult: false,
			expectedError:  "Teleport user does not match original requester",
			reqID:          "123",
			req: &fileTransferRequestWithApprovers{
				FileTransferRequest: reexecsftp.FileTransferRequest{
					ID:        "123",
					Requester: "michael",
				},
				approvers: make(map[string]*party),
			},
		},
		{
			name:           "current request location does not match original location",
			expectedResult: false,
			expectedError:  "requested destination path does not match the current request",
			reqID:          "123",
			location:       "~/Downloads",
			req: &fileTransferRequestWithApprovers{
				FileTransferRequest: reexecsftp.FileTransferRequest{
					ID:        "123",
					Requester: "teleportUser",
					Location:  "~/badlocation",
				},
				approvers: make(map[string]*party),
			},
		},
		{
			name:           "approved request",
			expectedResult: true,
			expectedError:  "",
			reqID:          "123",
			location:       "~/Downloads",
			req: &fileTransferRequestWithApprovers{
				FileTransferRequest: reexecsftp.FileTransferRequest{
					ID:        "123",
					Requester: "teleportUser",
					Location:  "~/Downloads",
				},
				approvers: approvers,
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			// create and add a session to the registry
			sess, _ := testOpenSession(t, reg, accessRoleSet, &decisionpb.SSHAccessPermit{})

			// create a FileTransferRequest. can be nil
			sess.fileTransferReq = tt.req

			// new exec request context
			scx := newTestServerContext(t, reg.Srv, accessRoleSet, &decisionpb.SSHAccessPermit{})
			scx.SetEnv(sftp.EnvModeratedSessionID, sess.ID())
			result, err := reg.isApprovedFileTransfer(scx)
			if err != nil {
				require.Equal(t, tt.expectedError, err.Error())
			}

			require.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestSession_newRecorder(t *testing.T) {
	t.Parallel()

	proxyRecording, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
		Mode: types.RecordAtProxy,
	})
	require.NoError(t, err)

	proxyRecordingSync, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
		Mode: types.RecordAtProxySync,
	})
	require.NoError(t, err)

	nodeRecordingSync, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
		Mode: types.RecordAtNodeSync,
	})
	require.NoError(t, err)

	logger := logtest.NewLogger()

	isNotSessionWriter := func(t require.TestingT, i any, i2 ...any) {
		require.NotNil(t, i)
		_, ok := i.(*events.SessionWriter)
		require.False(t, ok)
	}

	cases := []struct {
		desc           string
		sctx           *ServerContext
		noninteractive bool
		errAssertion   require.ErrorAssertionFunc
		recAssertion   require.ValueAssertionFunc
	}{
		{
			desc: "discard-stream-when-proxy-recording",

			sctx: &ServerContext{
				SessionRecordingConfig: proxyRecording,
				srv: &mockServer{
					component: teleport.ComponentNode,
				},
				Identity: IdentityContext{
					AccessPermit: &decisionpb.SSHAccessPermit{},
				},
			},
			errAssertion: require.NoError,
			recAssertion: isNotSessionWriter,
		},
		{
			desc: "discard-stream-when-proxy-sync-recording",
			sctx: &ServerContext{
				SessionRecordingConfig: proxyRecordingSync,
				srv: &mockServer{
					component: teleport.ComponentNode,
				},
				Identity: IdentityContext{
					AccessPermit: &decisionpb.SSHAccessPermit{},
				},
			},
			errAssertion: require.NoError,
			recAssertion: isNotSessionWriter,
		},
		{
			desc:           "discard-stream-when-non-interactive-non-bpf",
			noninteractive: true,
			sctx: &ServerContext{
				SessionRecordingConfig: proxyRecordingSync,
				srv: &mockServer{
					component: teleport.ComponentNode,
				},
				Identity: IdentityContext{
					AccessPermit: &decisionpb.SSHAccessPermit{},
				},
			},
			errAssertion: require.NoError,
			recAssertion: isNotSessionWriter,
		},
		{
			desc: "strict-err-new-audit-writer-fails",
			sctx: &ServerContext{
				SessionRecordingConfig: nodeRecordingSync,
				srv: &mockServer{
					component: teleport.ComponentNode,
				},
				Identity: IdentityContext{
					AccessPermit: decisionpb.SSHAccessPermit_builder{
						SessionRecordingMode: string(constants.SessionRecordingModeStrict),
					}.Build(),
				},
			},
			errAssertion: require.Error,
			recAssertion: require.Nil,
		},
		{
			desc: "best-effort-err-new-audit-writer-succeeds",
			sctx: &ServerContext{
				ClusterName:            "test",
				SessionRecordingConfig: nodeRecordingSync,
				srv: &mockServer{
					component: teleport.ComponentNode,
					datadir:   t.TempDir(),
				},
				Identity: IdentityContext{
					AccessPermit: decisionpb.SSHAccessPermit_builder{
						SessionRecordingMode: string(constants.SessionRecordingModeBestEffort),
					}.Build(),
				},
			},
			errAssertion: require.NoError,
			recAssertion: func(t require.TestingT, i any, _ ...any) {
				require.NotNil(t, i)
				sw, ok := i.(apievents.Stream)
				require.True(t, ok)
				require.NoError(t, sw.Close(context.Background()))
			},
		},
		{
			desc: "audit-writer",
			sctx: &ServerContext{
				ClusterName:            "test",
				SessionRecordingConfig: nodeRecordingSync,
				srv: &mockServer{
					MockRecorderEmitter: &eventstest.MockRecorderEmitter{},
					datadir:             t.TempDir(),
					component:           teleport.ComponentNode,
				},
				Identity: IdentityContext{
					AccessPermit: &decisionpb.SSHAccessPermit{},
				},
			},
			errAssertion: require.NoError,
			recAssertion: func(t require.TestingT, i any, i2 ...any) {
				require.NotNil(t, i)
				sw, ok := i.(apievents.Stream)
				require.True(t, ok)
				require.NoError(t, sw.Close(context.Background()))
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.desc, func(t *testing.T) {
			sess := &session{
				id:     "test",
				logger: logger,
				registry: &SessionRegistry{
					logger: logtest.NewLogger(),
					SessionRegistryConfig: SessionRegistryConfig{
						Srv: tt.sctx.srv,
					},
				},
				scx: tt.sctx,
			}

			sessType := sessionTypeInteractive
			if tt.noninteractive {
				sessType = sessionTypeNonInteractive
			}

			rec, err := newRecorder(sess, tt.sctx, sessType)
			tt.errAssertion(t, err)
			tt.recAssertion(t, rec)
		})
	}
}

func TestSessionRegistrySetupFailureCleanup(t *testing.T) {
	moderatedRole, err := types.NewRole("access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			RequireSessionJoin: []*types.SessionRequirePolicy{{
				Name:   "foo",
				Filter: "contains(user.roles, 'auditor')",
				Kinds:  []string{string(types.SSHSessionKind)},
				Modes:  []string{string(types.SessionModeratorMode)},
				Count:  1,
			}},
		},
	})
	require.NoError(t, err)

	recordAtNodeSync := func() *types.SessionRecordingConfigV2 {
		return &types.SessionRecordingConfigV2{
			Kind:    types.KindSessionRecordingConfig,
			Version: types.V2,
			Spec: types.SessionRecordingConfigSpecV2{
				Mode: types.RecordAtNodeSync,
			},
		}
	}

	type testCase struct {
		name         string
		openSession  func(context.Context, *SessionRegistry, ssh.Channel, *ServerContext) error
		sessionRoles services.RoleSet
		configure    func(*testing.T, *mockServer, *ServerContext, *trackerService)
		errAssertion require.ErrorAssertionFunc
	}

	cases := []testCase{
		{
			name: "interactive track session failure",
			openSession: func(ctx context.Context, reg *SessionRegistry, ch ssh.Channel, scx *ServerContext) error {
				return reg.OpenSession(ctx, ch, scx)
			},
			sessionRoles: services.NewRoleSet(moderatedRole),
			configure: func(t *testing.T, srv *mockServer, scx *ServerContext, trackingService *trackerService) {
				scx.SessionRecordingConfig = recordAtNodeSync()
				trackingService.createError = trace.ConnectionProblem(context.DeadlineExceeded, "")
			},
		},
		{
			name: "interactive recorder failure",
			openSession: func(ctx context.Context, reg *SessionRegistry, ch ssh.Channel, scx *ServerContext) error {
				return reg.OpenSession(ctx, ch, scx)
			},
			configure: func(t *testing.T, srv *mockServer, scx *ServerContext, _ *trackerService) {
				srv.datadir = ""
				scx.SessionRecordingConfig = recordAtNodeSync()
			},
		},
		{
			name: "exec unapproved moderated session",
			openSession: func(ctx context.Context, reg *SessionRegistry, ch ssh.Channel, scx *ServerContext) error {
				return reg.OpenExecSession(ctx, ch, scx)
			},
			sessionRoles: services.NewRoleSet(moderatedRole),
			configure: func(t *testing.T, _ *mockServer, scx *ServerContext, _ *trackerService) {
				scx.SessionRecordingConfig = recordAtNodeSync()
			},
			errAssertion: func(t require.TestingT, err error, _ ...any) {
				require.ErrorIs(t, err, errCannotStartUnattendedSession)
			},
		},
		{
			name: "exec unapproved moderated file transfer",
			openSession: func(ctx context.Context, reg *SessionRegistry, ch ssh.Channel, scx *ServerContext) error {
				return reg.OpenExecSession(ctx, ch, scx)
			},
			configure: func(t *testing.T, _ *mockServer, scx *ServerContext, _ *trackerService) {
				scx.SetEnv(sftp.EnvModeratedSessionID, string(rsession.NewID()))
			},
			errAssertion: func(t require.TestingT, err error, _ ...any) {
				require.True(t, trace.IsNotFound(err))
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			srv := newMockServer(t)

			mockTrackerService := &mockSessiontrackerService{
				trackers: make(map[string]types.SessionTracker),
			}
			trackingService := &trackerService{
				SessionTrackerService: mockTrackerService,
			}

			reg, err := NewSessionRegistry(SessionRegistryConfig{
				Srv:                   srv,
				SessionTrackerService: trackingService,
				clock:                 clockwork.NewFakeClock(),
			})
			require.NoError(t, err)
			t.Cleanup(func() { reg.Close() })

			// Use strict recording mode so that recorder errors aren't ignored.
			accessPermit := decisionpb.SSHAccessPermit_builder{
				SessionRecordingMode: string(constants.SessionRecordingModeStrict),
			}.Build()

			scx := newTestServerContext(t, reg.Srv, tt.sessionRoles, accessPermit)
			t.Cleanup(func() { require.NoError(t, scx.Close()) })

			tt.configure(t, srv, scx, trackingService)

			// Open a new session
			clientChan, serverChan := newMockSSHChannel(t)
			clientChan.Drain()

			countBefore := testutil.ToFloat64(serverSessions)

			err = tt.openSession(t.Context(), reg, serverChan, scx)
			require.Error(t, err)
			require.Nil(t, scx.party)

			if tt.errAssertion != nil {
				tt.errAssertion(t, err)
			}

			require.InDelta(t, countBefore, testutil.ToFloat64(serverSessions), 0)

			// If we created a tracker before the failure, the tracker should be updated to terminated.
			if trackingService.CreatedCount() != 0 {
				for _, tracker := range mockTrackerService.trackers {
					require.Equal(t, types.SessionState_SessionStateTerminated, tracker.GetState())
				}
			}
		})
	}
}

func TestSession_emitAuditEvent(t *testing.T) {
	t.Parallel()

	logger := logtest.NewLogger()

	t.Run("FallbackConcurrency", func(t *testing.T) {
		srv := newMockServer(t)
		reg, err := NewSessionRegistry(SessionRegistryConfig{
			Srv:                   srv,
			SessionTrackerService: srv.auth,
		})
		require.NoError(t, err)
		t.Cleanup(func() { reg.Close() })

		sess := &session{
			id:     "test",
			logger: logger,
			recorder: &mockRecorder{
				SessionPreparerRecorder: events.WithNoOpPreparer(events.NewDiscardRecorder()),
				done:                    true,
			},
			emitter:  srv,
			registry: reg,
			scx:      newTestServerContext(t, srv, nil, nil),
		}

		controlCh := make(chan struct{})
		emit := func() {
			<-controlCh
			sess.emitSessionStartEvent(sess.scx)
		}

		// Multiple goroutines to emit events.
		go emit()
		go emit()

		// Trigger events...
		close(controlCh)

		// Wait for the events on the new recorder
		require.Eventually(t, func() bool {
			return len(srv.Events()) == 2
		}, 1000*time.Second, 100*time.Millisecond)
	})
}

// TestInteractiveSession tests interactive session lifecycles
// and validates audit events and session recordings are emitted.
func TestInteractiveSession(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	srv := newMockServer(t)
	srv.component = teleport.ComponentNode
	t.Cleanup(func() { require.NoError(t, srv.auth.Close()) })

	reg, err := NewSessionRegistry(SessionRegistryConfig{
		Srv:                   srv,
		SessionTrackerService: srv.auth,
	})
	require.NoError(t, err)
	t.Cleanup(func() { reg.Close() })

	// Create a server context with an overridden recording mode
	// so that sessions are recorded with the test emitter.
	scx := newTestServerContext(t, reg.Srv, nil, &decisionpb.SSHAccessPermit{})
	rcfg := types.DefaultSessionRecordingConfig()
	rcfg.SetMode(types.RecordAtNodeSync)
	scx.SessionRecordingConfig = rcfg

	// Allocate a terminal for the session so that
	// events are properly recorded.
	terminal, err := newLocalTerminal(scx)
	require.NoError(t, err)
	scx.term = terminal

	// Open a new session
	clientChan, serverChan := newMockSSHChannel(t)
	clientChan.Drain()

	require.NoError(t, reg.OpenSession(ctx, serverChan, scx))
	require.NotNil(t, scx.party)

	// Simulate changing window size to capture an additional event.
	require.NoError(t, reg.NotifyWinChange(ctx, rsession.TerminalParams{W: 100, H: 100}, scx))

	// Stopping the session should trigger the session
	// to end and cleanup in the background
	scx.party.s.Stop()

	// Wait for the session to be removed from the registry.
	require.Eventually(t, func() bool {
		_, found := reg.findSession(scx.party.s.id)
		return !found
	}, time.Second*15, time.Millisecond*500)

	// Validate that the expected audit events were emitted.
	expectedEvents := []string{events.SessionStartEvent, events.ResizeEvent, events.SessionEndEvent, events.SessionLeaveEvent}
	require.Eventually(t, func() bool {
		actual := srv.MockRecorderEmitter.Events()

		for _, evt := range expectedEvents {
			contains := slices.ContainsFunc(actual, func(event apievents.AuditEvent) bool {
				return event.GetType() == evt
			})
			if !contains {
				return false
			}
		}
		return true
	}, 15*time.Second, 500*time.Millisecond)

	// Validate that the expected recording events were emitted.
	require.Eventually(t, func() bool {
		actual := srv.MockRecorderEmitter.RecordedEvents()

		for _, evt := range expectedEvents {
			contains := slices.ContainsFunc(actual, func(event apievents.PreparedSessionEvent) bool {
				return event.GetAuditEvent().GetType() == evt
			})
			if !contains {
				return false
			}
		}

		return true
	}, 15*time.Second, 500*time.Millisecond)
}

// TestNonInteractiveSession tests non-interactive session lifecycles
// and validates audit events and session recordings are emitted when
// appropriate.
func TestNonInteractiveSession(t *testing.T) {
	t.Parallel()

	t.Run("without BPF", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		srv := newMockServer(t)
		srv.component = teleport.ComponentNode
		t.Cleanup(func() { require.NoError(t, srv.auth.Close()) })

		reg, err := NewSessionRegistry(SessionRegistryConfig{
			Srv:                   srv,
			SessionTrackerService: srv.auth,
		})
		require.NoError(t, err)
		t.Cleanup(func() { reg.Close() })

		// Create a server context with an overridden recording mode
		// so that sessions are recorded with the test emitter.
		scx := newTestServerContext(t, reg.Srv, nil, &decisionpb.SSHAccessPermit{})
		rcfg := types.DefaultSessionRecordingConfig()
		rcfg.SetMode(types.RecordAtNodeSync)
		scx.SessionRecordingConfig = rcfg

		// Modify the execRequest to actually execute a command.
		scx.execRequest = &localExec{Ctx: scx, Command: "true"}

		// Open a new session
		clientChan, serverChan := newMockSSHChannel(t)
		clientChan.Drain()

		require.NoError(t, reg.OpenExecSession(ctx, serverChan, scx))
		require.NotNil(t, scx.party)

		// Wait for the command execution to complete and the session to be terminated.
		require.Eventually(t, func() bool {
			_, found := reg.findSession(scx.party.s.id)
			return !found
		}, time.Second*15, time.Millisecond*500)

		// Verify that all the expected audit events are eventually emitted.
		expected := []string{events.SessionStartEvent, events.ExecEvent, events.SessionEndEvent, events.SessionLeaveEvent}
		require.Eventually(t, func() bool {
			actual := srv.MockRecorderEmitter.Events()

			for _, evt := range expected {
				contains := slices.ContainsFunc(actual, func(event apievents.AuditEvent) bool {
					return event.GetType() == evt
				})
				if !contains {
					return false
				}
			}

			return true
		}, 15*time.Second, 500*time.Millisecond)

		// Verify that NO recordings were emitted
		require.Empty(t, srv.MockRecorderEmitter.RecordedEvents())
	})

	t.Run("with BPF", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()

		srv := newMockServer(t)
		srv.component = teleport.ComponentNode
		// Modify bpf to "enable" enhanced recording. This should
		// trigger recordings to be captured.
		srv.bpf = fakeBPF{}
		t.Cleanup(func() { require.NoError(t, srv.auth.Close()) })

		reg, err := NewSessionRegistry(SessionRegistryConfig{
			Srv:                   srv,
			SessionTrackerService: srv.auth,
		})
		require.NoError(t, err)
		t.Cleanup(func() { reg.Close() })

		// Create a server context with an overridden recording mode
		// so that sessions are recorded with the test emitter.
		scx := newTestServerContext(t, reg.Srv, nil, &decisionpb.SSHAccessPermit{})
		rcfg := types.DefaultSessionRecordingConfig()
		rcfg.SetMode(types.RecordAtNodeSync)
		scx.SessionRecordingConfig = rcfg

		// Modify the execRequest to actually execute a command.
		scx.execRequest = &localExec{Ctx: scx, Command: "true"}

		// Open a new session
		clientChan, serverChan := newMockSSHChannel(t)
		clientChan.Drain()

		require.NoError(t, reg.OpenExecSession(ctx, serverChan, scx))
		require.NotNil(t, scx.party)

		// Wait for the command execution to complete and the session to be terminated.
		require.Eventually(t, func() bool {
			_, found := reg.findSession(scx.party.s.id)
			return !found
		}, time.Second*15, time.Millisecond*500)

		// Verify that all the expected audit events are eventually emitted.
		expectedEvents := []string{events.SessionStartEvent, events.ExecEvent, events.SessionEndEvent, events.SessionLeaveEvent}
		require.Eventually(t, func() bool {
			actual := srv.MockRecorderEmitter.Events()

			for _, evt := range expectedEvents {
				contains := slices.ContainsFunc(actual, func(event apievents.AuditEvent) bool {
					return event.GetType() == evt
				})
				if !contains {
					return false
				}
			}

			return true
		}, 15*time.Second, 500*time.Millisecond)

		// Validate that the expected recording events were emitted.
		require.Eventually(t, func() bool {
			actual := srv.MockRecorderEmitter.RecordedEvents()

			for _, evt := range expectedEvents {
				if evt == events.ExecEvent {
					continue
				}
				contains := slices.ContainsFunc(actual, func(event apievents.PreparedSessionEvent) bool {
					return event.GetAuditEvent().GetType() == evt
				})
				if !contains {
					return false
				}
			}

			return true
		}, 15*time.Second, 500*time.Millisecond)
	})
}

// TestStopUnstarted tests that a session may be stopped before it launches.
func TestStopUnstarted(t *testing.T) {
	modulestest.SetTestModules(t, modulestest.Modules{TestBuildType: modules.BuildEnterprise})
	srv := newMockServer(t)
	srv.component = teleport.ComponentNode

	reg, err := NewSessionRegistry(SessionRegistryConfig{
		Srv:                   srv,
		SessionTrackerService: srv.auth,
	})
	require.NoError(t, err)
	t.Cleanup(func() { reg.Close() })

	role, err := types.NewRole("access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			RequireSessionJoin: []*types.SessionRequirePolicy{{
				Name:   "foo",
				Filter: "contains(user.roles, 'auditor')",
				Kinds:  []string{string(types.SSHSessionKind)},
				Modes:  []string{string(types.SessionPeerMode)},
				Count:  999,
			}},
		},
	})
	require.NoError(t, err)

	roles := services.NewRoleSet(role)
	sess, _ := testOpenSession(t, reg, roles, &decisionpb.SSHAccessPermit{})

	// Stopping the session should trigger the session
	// to end and cleanup in the background
	sess.Stop()

	sessionClosed := func() bool {
		_, found := reg.findSession(sess.id)
		return !found
	}
	require.Eventually(t, sessionClosed, time.Second*15, time.Millisecond*500)
}

// fakeEvaluatorAuthClient embeds authclient.ClientI so it satisfies the
// interface (all other methods are nil and unused) and overrides
// EvaluateCommand so an AI-moderated session can be exercised without a real
// auth server. The embedded nil interface would panic only if some other
// method were called, which these tests do not do.
type fakeEvaluatorAuthClient struct {
	authclient.ClientI
	resp   *proto.EvaluateCommandResponse
	err    error
	gotReq *proto.EvaluateCommandRequest
}

func (f *fakeEvaluatorAuthClient) EvaluateCommand(ctx context.Context, req *proto.EvaluateCommandRequest, opts ...grpc.CallOption) (*proto.EvaluateCommandResponse, error) {
	f.gotReq = req
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

// TestSessionCommandGate verifies that a command gate is installed on the
// session's TermManager when (and only when) a human per-command approval
// policy applies to the session.
func TestSessionCommandGate(t *testing.T) {
	modulestest.SetTestModules(t, modulestest.Modules{TestBuildType: modules.BuildEnterprise})

	newRegWithServer := func(t *testing.T) (*SessionRegistry, *mockServer) {
		srv := newMockServer(t)
		srv.component = teleport.ComponentNode
		reg, err := NewSessionRegistry(SessionRegistryConfig{
			Srv:                   srv,
			SessionTrackerService: srv.auth,
		})
		require.NoError(t, err)
		t.Cleanup(func() { reg.Close() })
		return reg, srv
	}
	newReg := func(t *testing.T) *SessionRegistry {
		reg, _ := newRegWithServer(t)
		return reg
	}

	t.Run("human command approval installs gate", func(t *testing.T) {
		reg := newReg(t)
		role, err := types.NewRole("access", types.RoleSpecV6{
			Allow: types.RoleConditions{
				RequireSessionJoin: []*types.SessionRequirePolicy{{
					Name:   "approve",
					Filter: "contains(user.roles, 'moderator')",
					Kinds:  []string{string(types.SSHSessionKind)},
					Modes:  []string{string(types.SessionModeratorMode)},
					Count:  1,
					CommandApproval: &types.CommandApproval{
						Enabled:  true,
						Approver: types.CommandApproverHuman,
					},
				}},
			},
		})
		require.NoError(t, err)

		sess, _ := testOpenSession(t, reg, services.NewRoleSet(role), &decisionpb.SSHAccessPermit{})
		t.Cleanup(func() { sess.Stop() })

		require.True(t, sess.io.commandGateEnabled(), "expected command gate to be installed")
		require.NotNil(t, sess.getCommandApprover(), "expected a command approver to be set")
	})

	aiRole := func(t *testing.T) types.Role {
		role, err := types.NewRole("access", types.RoleSpecV6{
			Allow: types.RoleConditions{
				RequireSessionJoin: []*types.SessionRequirePolicy{{
					Name:   "approve",
					Filter: "contains(user.roles, 'moderator')",
					Kinds:  []string{string(types.SSHSessionKind)},
					Modes:  []string{string(types.SessionModeratorMode)},
					Count:  1,
					CommandApproval: &types.CommandApproval{
						Enabled:  true,
						Approver: types.CommandApproverAI,
						AI:       &types.CommandApprovalAI{Policy: "deny dangerous commands", Model: "claude"},
					},
				}},
			},
		})
		require.NoError(t, err)
		return role
	}

	t.Run("ai command approval without auth client fails closed", func(t *testing.T) {
		// With no auth client available, an AI-configured session MUST install a
		// gate that denies every command rather than running commands ungated
		// (fail-open).
		reg := newReg(t)

		sess, _ := testOpenSession(t, reg, services.NewRoleSet(aiRole(t)), &decisionpb.SSHAccessPermit{})
		t.Cleanup(func() { sess.Stop() })

		require.True(t, sess.io.commandGateEnabled(), "expected an AI fail-closed command gate to be installed")

		gate := sess.io.commandGate
		require.NotNil(t, gate)
		require.False(t, gate.approve("rm -rf /"), "AI gate must deny commands (fail-closed) when no auth client is present")
	})

	t.Run("ai command approval with auth client installs AI-backed gate", func(t *testing.T) {
		// When an auth client is present, the AI approver delegates each command
		// to the EvaluateCommand RPC. Here a fake client approves the command.
		reg, srv := newRegWithServer(t)
		fake := &fakeEvaluatorAuthClient{
			resp: &proto.EvaluateCommandResponse{Approved: true, Reasoning: "ok"},
		}
		srv.authClient = fake

		sess, _ := testOpenSession(t, reg, services.NewRoleSet(aiRole(t)), &decisionpb.SSHAccessPermit{})
		t.Cleanup(func() { sess.Stop() })

		require.True(t, sess.io.commandGateEnabled(), "expected an AI-backed command gate to be installed")

		gate := sess.io.commandGate
		require.NotNil(t, gate)
		// The gate is AI-backed (not deny-all): the fake approves, so the
		// command is allowed and the RPC was invoked with the configured policy
		// and model.
		require.True(t, gate.approve("ls"), "AI-backed gate must honor the approver decision")
		require.NotNil(t, fake.gotReq)
		require.Equal(t, "ls", fake.gotReq.Command)
		require.Equal(t, "deny dangerous commands", fake.gotReq.Policy)
		require.Equal(t, "claude", fake.gotReq.Model)

		// An approved command must emit a CommandApproval audit event with the
		// approved code and AI approver details.
		var approval *apievents.CommandApproval
		for _, e := range srv.MockRecorderEmitter.Events() {
			if ca, ok := e.(*apievents.CommandApproval); ok {
				approval = ca
			}
		}
		require.NotNil(t, approval, "expected a CommandApproval audit event to be emitted")
		require.Equal(t, events.CommandApprovalApprovedEvent, approval.Metadata.Type)
		require.Equal(t, events.CommandApprovalApprovedCode, approval.Metadata.Code)
		require.Equal(t, "ls", approval.Command)
		require.Equal(t, "approved", approval.Decision)
		require.Equal(t, "ai", approval.ApproverMode)
		require.Equal(t, "claude", approval.Model)
	})

	t.Run("no command approval installs no gate", func(t *testing.T) {
		reg := newReg(t)
		role, err := types.NewRole("access", types.RoleSpecV6{
			Allow: types.RoleConditions{
				RequireSessionJoin: []*types.SessionRequirePolicy{{
					Name:   "join",
					Filter: "contains(user.roles, 'moderator')",
					Kinds:  []string{string(types.SSHSessionKind)},
					Modes:  []string{string(types.SessionModeratorMode)},
					Count:  1,
				}},
			},
		})
		require.NoError(t, err)

		sess, _ := testOpenSession(t, reg, services.NewRoleSet(role), &decisionpb.SSHAccessPermit{})
		t.Cleanup(func() { sess.Stop() })

		require.False(t, sess.io.commandGateEnabled(), "expected no command gate")
		require.Nil(t, sess.getCommandApprover(), "expected no command approver")
	})
}

// signalingBroadcaster is a test approval.Broadcaster that records every
// broadcast request and signals its arrival on a channel. Because HumanApprover
// registers the pending decision channel BEFORE calling
// BroadcastApprovalRequest, observing a broadcast guarantees the corresponding
// Submit will find a registered pending entry — this is the synchronization
// point that lets the round-trip tests avoid time.Sleep flakiness.
type signalingBroadcaster struct {
	requests chan approval.CommandApprovalRequest
	err      error
}

func newSignalingBroadcaster() *signalingBroadcaster {
	return &signalingBroadcaster{requests: make(chan approval.CommandApprovalRequest, 8)}
}

func (b *signalingBroadcaster) BroadcastApprovalRequest(req approval.CommandApprovalRequest) error {
	if b.err != nil {
		return b.err
	}
	b.requests <- req
	return nil
}

// findCommandApproval returns the last CommandApproval audit event recorded by
// the emitter, or nil if none was emitted.
func findCommandApproval(events []apievents.AuditEvent) *apievents.CommandApproval {
	var found *apievents.CommandApproval
	for _, e := range events {
		if ca, ok := e.(*apievents.CommandApproval); ok {
			found = ca
		}
	}
	return found
}

// TestCommandApprovalRoundTrip exercises the per-command approval pipeline end
// to end at the session/TermManager seam: a real session (built via the test
// harness) gets a sessionCommandGate installed on its TermManager, command
// bytes are driven through the real TermManager byte path (gateInput), and the
// approval handshake (HumanApprover.Approve -> Submit, or the WithTimeout
// fail-closed path) resolves the decision. The gate's emitted CommandApproval
// audit event is captured via the harness's MockRecorderEmitter.
//
// gateInput is the gate-bypass boundary: it returns a carriage return for an
// approved command (the line runs) and Ctrl-U for a denied/failed command (the
// readline buffer is cleared and the line never executes). Asserting on the
// returned bytes therefore proves the security-relevant behavior, not just the
// boolean decision.
func TestCommandApprovalRoundTrip(t *testing.T) {
	modulestest.SetTestModules(t, modulestest.Modules{TestBuildType: modules.BuildEnterprise})

	newRegWithServer := func(t *testing.T) (*SessionRegistry, *mockServer) {
		srv := newMockServer(t)
		srv.component = teleport.ComponentNode
		reg, err := NewSessionRegistry(SessionRegistryConfig{
			Srv:                   srv,
			SessionTrackerService: srv.auth,
		})
		require.NoError(t, err)
		t.Cleanup(func() { reg.Close() })
		return reg, srv
	}

	humanRole := func(t *testing.T) types.Role {
		role, err := types.NewRole("access", types.RoleSpecV6{
			Allow: types.RoleConditions{
				RequireSessionJoin: []*types.SessionRequirePolicy{{
					Name:   "approve",
					Filter: "contains(user.roles, 'moderator')",
					Kinds:  []string{string(types.SSHSessionKind)},
					Modes:  []string{string(types.SessionModeratorMode)},
					Count:  1,
					CommandApproval: &types.CommandApproval{
						Enabled:  true,
						Approver: types.CommandApproverHuman,
					},
				}},
			},
		})
		require.NoError(t, err)
		return role
	}

	// installHumanGate builds a real sessionCommandGate backed by a real
	// HumanApprover and the supplied broadcaster, wrapped with WithTimeout
	// exactly as production does, and installs it on the session's TermManager.
	// It returns the approver so the test can Submit a moderator decision.
	installHumanGate := func(t *testing.T, sess *session, b approval.Broadcaster, timeout time.Duration) *approval.HumanApprover {
		approver := approval.NewHumanApprover(b)
		gate := &sessionCommandGate{
			s:        sess,
			approver: approval.WithTimeout(approver, timeout),
			mode:     string(approval.ModeHuman),
		}
		sess.io.SetCommandGate(gate)
		return approver
	}

	// drive runs gateInput on a goroutine and returns a channel that yields the
	// bytes gateInput forwards to the shell. gateInput blocks inside approve()
	// until the decision resolves, so the caller drives the moderator side while
	// this is in flight.
	drive := func(io *TermManager, input string) <-chan []byte {
		out := make(chan []byte, 1)
		go func() { out <- io.gateInput([]byte(input)) }()
		return out
	}

	t.Run("human approve forwards the command", func(t *testing.T) {
		reg, srv := newRegWithServer(t)
		sess, _ := testOpenSession(t, reg, services.NewRoleSet(humanRole(t)), &decisionpb.SSHAccessPermit{})
		t.Cleanup(func() { sess.Stop() })
		require.True(t, sess.io.commandGateEnabled(), "harness must install a gate for a human policy")

		b := newSignalingBroadcaster()
		approver := installHumanGate(t, sess, b, approval.DefaultTimeout)

		out := drive(sess.io, "ls -la\r")

		// Wait for the broadcast (pending entry is registered) then approve.
		var req approval.CommandApprovalRequest
		select {
		case req = <-b.requests:
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for approval broadcast")
		}
		require.Equal(t, "ls -la", req.Command)
		approver.Submit(req.ID, true, "looks fine", "moderator-1", types.SessionModeratorMode)

		var forwarded []byte
		select {
		case forwarded = <-out:
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for gateInput to return")
		}
		// Approved: a carriage return is forwarded and NO Ctrl-U appears.
		require.Contains(t, string(forwarded), string(rune(byteCR)), "approved command must forward a carriage return")
		require.NotContains(t, forwarded, byte(byteCtrlU), "approved command must not inject Ctrl-U")

		approvalEv := findCommandApproval(srv.MockRecorderEmitter.Events())
		require.NotNil(t, approvalEv, "expected a CommandApproval audit event")
		require.Equal(t, events.CommandApprovalApprovedEvent, approvalEv.Metadata.Type)
		require.Equal(t, events.CommandApprovalApprovedCode, approvalEv.Metadata.Code)
		require.Equal(t, "ls -la", approvalEv.Command)
		require.Equal(t, "approved", approvalEv.Decision)
		require.Equal(t, "human", approvalEv.ApproverMode)
		require.Equal(t, "moderator-1", approvalEv.Approver)
	})

	t.Run("human deny injects Ctrl-U", func(t *testing.T) {
		reg, srv := newRegWithServer(t)
		sess, _ := testOpenSession(t, reg, services.NewRoleSet(humanRole(t)), &decisionpb.SSHAccessPermit{})
		t.Cleanup(func() { sess.Stop() })

		b := newSignalingBroadcaster()
		approver := installHumanGate(t, sess, b, approval.DefaultTimeout)

		out := drive(sess.io, "rm -rf /\r")

		var req approval.CommandApprovalRequest
		select {
		case req = <-b.requests:
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for approval broadcast")
		}
		approver.Submit(req.ID, false, "too dangerous", "moderator-1", types.SessionModeratorMode)

		var forwarded []byte
		select {
		case forwarded = <-out:
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for gateInput to return")
		}
		// Denied: Ctrl-U is injected to clear the line and NO bare CR runs it.
		require.Contains(t, forwarded, byte(byteCtrlU), "denied command must inject Ctrl-U")
		require.NotContains(t, forwarded, byte(byteCR), "denied command must not forward a carriage return")

		approvalEv := findCommandApproval(srv.MockRecorderEmitter.Events())
		require.NotNil(t, approvalEv, "expected a CommandApproval audit event")
		require.Equal(t, events.CommandApprovalDeniedEvent, approvalEv.Metadata.Type)
		require.Equal(t, events.CommandApprovalDeniedCode, approvalEv.Metadata.Code)
		require.Equal(t, "rm -rf /", approvalEv.Command)
		require.Equal(t, "denied", approvalEv.Decision)
		require.Equal(t, "human", approvalEv.ApproverMode)
		require.Equal(t, "moderator-1", approvalEv.Approver)
	})

	t.Run("no moderator response fails closed (timeout)", func(t *testing.T) {
		reg, srv := newRegWithServer(t)
		sess, _ := testOpenSession(t, reg, services.NewRoleSet(humanRole(t)), &decisionpb.SSHAccessPermit{})
		t.Cleanup(func() { sess.Stop() })

		b := newSignalingBroadcaster()
		// Short timeout: nobody Submits, so WithTimeout must deny (fail-closed).
		installHumanGate(t, sess, b, 50*time.Millisecond)

		out := drive(sess.io, "whoami\r")

		// The broadcast still happens; we simply never respond.
		select {
		case <-b.requests:
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for approval broadcast")
		}

		var forwarded []byte
		select {
		case forwarded = <-out:
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for gateInput to return after timeout")
		}
		require.Contains(t, forwarded, byte(byteCtrlU), "timed-out command must inject Ctrl-U")
		require.NotContains(t, forwarded, byte(byteCR), "timed-out command must not forward a carriage return")

		approvalEv := findCommandApproval(srv.MockRecorderEmitter.Events())
		require.NotNil(t, approvalEv, "expected a CommandApproval audit event")
		require.Equal(t, events.CommandApprovalFailedEvent, approvalEv.Metadata.Type)
		require.Equal(t, events.CommandApprovalFailedCode, approvalEv.Metadata.Code)
		require.Equal(t, "failed", approvalEv.Decision)
		require.Equal(t, approval.ApproverSystem, approvalEv.Approver)
	})

	t.Run("broadcast failure fails closed", func(t *testing.T) {
		reg, srv := newRegWithServer(t)
		sess, _ := testOpenSession(t, reg, services.NewRoleSet(humanRole(t)), &decisionpb.SSHAccessPermit{})
		t.Cleanup(func() { sess.Stop() })

		b := newSignalingBroadcaster()
		b.err = trace.ConnectionProblem(nil, "no moderators reachable")
		installHumanGate(t, sess, b, approval.DefaultTimeout)

		// Broadcast fails synchronously inside Approve, so the decision returns
		// immediately without any moderator Submit.
		forwarded := sess.io.gateInput([]byte("cat /etc/shadow\r"))
		require.Contains(t, forwarded, byte(byteCtrlU), "broadcast failure must deny (Ctrl-U)")
		require.NotContains(t, forwarded, byte(byteCR))

		approvalEv := findCommandApproval(srv.MockRecorderEmitter.Events())
		require.NotNil(t, approvalEv, "expected a CommandApproval audit event")
		require.Equal(t, events.CommandApprovalFailedEvent, approvalEv.Metadata.Type)
		require.Equal(t, events.CommandApprovalFailedCode, approvalEv.Metadata.Code)
		require.Equal(t, approval.ApproverSystem, approvalEv.Approver)
	})

	t.Run("ai evaluator error fails closed (failed)", func(t *testing.T) {
		// An AI policy with an auth client whose EvaluateCommand returns the
		// enterprise-only NotImplemented error must fail closed: the command is
		// denied (Ctrl-U injected, no carriage return) and an audit event is
		// emitted.
		//
		// NOTE on event classification: an RPC/evaluation error is an
		// infrastructure failure, not a deliberate AI denial, so AIApprover
		// attributes it to the system (Approver=ApproverSystem) and the gate
		// records the "failed" event (T4006E / command.approval.failed) used
		// for system fail-closed outcomes (timeout/panic/broadcast failure).
		// The AI mode is preserved, so approver_mode=ai for context. This is
		// distinct from a deliberate AI deny (model returns approved=false),
		// which is audited as "denied" by ai-moderator. The "no auth client at
		// all" path (denyAllCommandGate) also emits "failed" and is covered by
		// TestSessionCommandGate.
		reg, srv := newRegWithServer(t)
		fake := &fakeEvaluatorAuthClient{
			err: trace.NotImplemented("per-command AI approval requires Teleport Enterprise"),
		}
		srv.authClient = fake

		aiRole, err := types.NewRole("access", types.RoleSpecV6{
			Allow: types.RoleConditions{
				RequireSessionJoin: []*types.SessionRequirePolicy{{
					Name:   "approve",
					Filter: "contains(user.roles, 'moderator')",
					Kinds:  []string{string(types.SSHSessionKind)},
					Modes:  []string{string(types.SessionModeratorMode)},
					Count:  1,
					CommandApproval: &types.CommandApproval{
						Enabled:  true,
						Approver: types.CommandApproverAI,
						AI:       &types.CommandApprovalAI{Policy: "deny dangerous commands", Model: "claude"},
					},
				}},
			},
		})
		require.NoError(t, err)

		sess, _ := testOpenSession(t, reg, services.NewRoleSet(aiRole), &decisionpb.SSHAccessPermit{})
		t.Cleanup(func() { sess.Stop() })
		require.True(t, sess.io.commandGateEnabled(), "expected an AI-backed gate")

		// Drive the full byte path: the evaluator errors, so the AI approver
		// fails closed and Ctrl-U is injected.
		forwarded := sess.io.gateInput([]byte("ls\r"))
		require.Contains(t, forwarded, byte(byteCtrlU), "AI evaluator error must deny (Ctrl-U)")
		require.NotContains(t, forwarded, byte(byteCR))
		require.NotNil(t, fake.gotReq, "the evaluator RPC must have been attempted")

		approvalEv := findCommandApproval(srv.MockRecorderEmitter.Events())
		require.NotNil(t, approvalEv, "expected a CommandApproval audit event")
		require.Equal(t, events.CommandApprovalFailedEvent, approvalEv.Metadata.Type)
		require.Equal(t, events.CommandApprovalFailedCode, approvalEv.Metadata.Code)
		require.Equal(t, "failed", approvalEv.Decision)
		require.Equal(t, approval.ApproverSystem, approvalEv.Approver)
		require.Equal(t, "ai", approvalEv.ApproverMode)
		require.Contains(t, approvalEv.Reason, "AI evaluation failed")
	})

	t.Run("backward compatible: no policy gates nothing", func(t *testing.T) {
		reg, srv := newRegWithServer(t)
		role, err := types.NewRole("access", types.RoleSpecV6{
			Allow: types.RoleConditions{
				RequireSessionJoin: []*types.SessionRequirePolicy{{
					Name:   "join",
					Filter: "contains(user.roles, 'moderator')",
					Kinds:  []string{string(types.SSHSessionKind)},
					Modes:  []string{string(types.SessionModeratorMode)},
					Count:  1,
				}},
			},
		})
		require.NoError(t, err)

		sess, _ := testOpenSession(t, reg, services.NewRoleSet(role), &decisionpb.SSHAccessPermit{})
		t.Cleanup(func() { sess.Stop() })

		require.False(t, sess.io.commandGateEnabled(), "no command_approval policy must install no gate")
		require.Nil(t, sess.getCommandApprover())

		// With no gate, gateInput is a pass-through: the input bytes are returned
		// unchanged (carriage return preserved, no Ctrl-U) and no audit event is
		// emitted for command approval.
		forwarded := sess.io.gateInput([]byte("ls\r"))
		require.Equal(t, "ls\r", string(forwarded), "ungated input must pass through unchanged")
		require.NotContains(t, forwarded, byte(byteCtrlU))
		require.Nil(t, findCommandApproval(srv.MockRecorderEmitter.Events()), "ungated session must emit no command approval event")
	})
}

func TestModeratedSessionPresence(t *testing.T) {
	modulestest.SetTestModules(t, modulestest.Modules{TestBuildType: modules.BuildEnterprise})

	srv := newMockServer(t)
	srv.component = teleport.ComponentNode

	reg, err := NewSessionRegistry(SessionRegistryConfig{
		Srv:                   srv,
		SessionTrackerService: srv.auth,
		clock:                 srv.clock,
	})
	require.NoError(t, err)
	t.Cleanup(func() { reg.Close() })

	hostRole, err := types.NewRole("moderated", types.RoleSpecV6{
		Allow: types.RoleConditions{
			RequireSessionJoin: []*types.SessionRequirePolicy{{
				Name:    "moderated",
				Filter:  "contains(user.roles, \"moderator\")",
				Kinds:   []string{string(types.SSHSessionKind)},
				Count:   1,
				Modes:   []string{string(types.SessionModeratorMode)},
				OnLeave: string(types.OnSessionLeaveTerminate),
			}},
		},
	})
	require.NoError(t, err)

	moderatorRole, err := types.NewRole("moderator", types.RoleSpecV6{
		Allow: types.RoleConditions{
			JoinSessions: []*types.SessionJoinPolicy{{
				Name:  "moderated",
				Roles: []string{hostRole.GetName()},
				Kinds: []string{string(types.SSHSessionKind)},
				Modes: []string{string(types.SessionModeratorMode)},
			}},
		},
	})
	require.NoError(t, err)

	hostCtx := newTestServerContext(t, reg.Srv, services.NewRoleSet(hostRole), &decisionpb.SSHAccessPermit{})
	hostCtx.Identity.UnmappedIdentity.MFAVerified = "mfa-device"
	moderatorCtx := newTestServerContext(t, reg.Srv, services.NewRoleSet(moderatorRole), &decisionpb.SSHAccessPermit{})
	moderatorCtx.Identity.TeleportUser = "moderator"

	findModeratorParticipant := func(t require.TestingT, tracker types.SessionTracker) types.Participant {
		for _, participant := range tracker.GetParticipants() {
			if participant.User == moderatorCtx.Identity.TeleportUser {
				return participant
			}
		}
		require.Fail(t, "moderator participant not found")
		return types.Participant{}
	}
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	// Create a new pending moderated session.
	hostClientChan, hostServerChan := newMockSSHChannel(t)
	hostClientChan.Drain()

	require.NoError(t, reg.OpenSession(ctx, hostServerChan, hostCtx))
	require.NotNil(t, hostCtx.party)
	sess := hostCtx.party.s
	t.Cleanup(sess.Stop)

	tracker, err := srv.auth.GetSessionTracker(t.Context(), sess.id.String())
	require.NoError(t, err)
	require.Equal(t, types.SessionState_SessionStatePending, tracker.GetState())

	// Have a moderator join the session to start it.
	modClientChan, modServerChan := newMockSSHChannel(t)
	modClientChan.Drain()
	require.NoError(t, reg.JoinSession(ctx, modServerChan, moderatorCtx, sess.id.String(), types.SessionModeratorMode))
	tracker, err = srv.auth.GetSessionTracker(t.Context(), sess.id.String())
	require.NoError(t, err)
	require.Equal(t, types.SessionState_SessionStateRunning, tracker.GetState())
	require.True(t, sess.started.Load())
	require.Len(t, sess.getParties(), 2)

	moderatorParticipant := findModeratorParticipant(t, tracker)
	require.NotEmpty(t, moderatorParticipant.ID)

	// Wait for the session tracker expiration timer and presence check ticker.
	srv.clock.BlockUntil(2)

	// Advance the clock and the moderators presence to the original stale threshold without exceeding it.
	presenceCheckInterval := srv.GetPresenceMaxDuration() / 4
	srv.clock.Advance(presenceCheckInterval)
	srv.clock.BlockUntil(2)
	presenceUpdateTime := srv.clock.Now().UTC()
	require.NoError(t, srv.auth.UpdatePresence(t.Context(), sess.id.String(), moderatorParticipant.User, moderatorParticipant.Cluster))

	// Advance the clock past the original stale threshold. The session should continue running.
	srv.clock.Advance(srv.GetPresenceMaxDuration() - presenceCheckInterval + time.Second)
	srv.clock.BlockUntil(2)

	tracker, err = srv.auth.GetSessionTracker(t.Context(), sess.id.String())
	require.NoError(t, err)
	refreshedParticipant := findModeratorParticipant(t, tracker)
	require.Equal(t, moderatorParticipant.ID, refreshedParticipant.ID)
	require.Equal(t, presenceUpdateTime, refreshedParticipant.LastActive)

	require.Never(t, func() bool {
		updatedTracker, err := srv.auth.GetSessionTracker(ctx, sess.id.String())
		require.NoError(t, err)
		return updatedTracker.GetState() != types.SessionState_SessionStateRunning
	}, 500*time.Millisecond, 100*time.Millisecond)

	// Advance the server clock so that the moderator is stale. The session should terminate.
	srv.clock.Advance(srv.GetPresenceMaxDuration())

	require.Eventually(t, sess.isStopped, time.Second, 10*time.Millisecond)
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		updatedTracker, err := srv.auth.GetSessionTracker(ctx, sess.id.String())
		require.NoError(t, err)
		require.Equal(t, types.SessionState_SessionStateTerminated, updatedTracker.GetState())
		for _, participant := range updatedTracker.GetParticipants() {
			require.NotEqual(t, moderatorParticipant.ID, participant.ID)
		}
	}, 5*time.Second, 100*time.Millisecond)
}

// TestParties tests the party mechanisms within an interactive session,
// including party leave, party disconnect, and empty session lingerAndDie.
func TestParties(t *testing.T) {
	t.Parallel()

	srv := newMockServer(t)
	srv.component = teleport.ComponentNode

	// Use a separate clock from srv so we can use BlockUntil.
	regClock := clockwork.NewFakeClock()
	reg, err := NewSessionRegistry(SessionRegistryConfig{
		Srv:                   srv,
		SessionTrackerService: srv.auth,
		clock:                 regClock,
	})
	require.NoError(t, err)
	t.Cleanup(func() { reg.Close() })

	// Create a session with 3 parties
	sess, _ := testOpenSession(t, reg, nil, &decisionpb.SSHAccessPermit{})
	require.Len(t, sess.getParties(), 1)
	testJoinSession(t, reg, sess.ID())
	require.Len(t, sess.getParties(), 2)
	testJoinSession(t, reg, sess.ID())
	require.Len(t, sess.getParties(), 3)

	// If a party leaves, the session should remove the party and continue.
	p := sess.getParties()[0]
	require.NoError(t, p.Close())

	partyIsRemoved := func() bool {
		return len(sess.getParties()) == 2 && !sess.isStopped()
	}
	require.Eventually(t, partyIsRemoved, time.Second*5, time.Millisecond*500)

	// If a party's session context is closed, the party should leave the session.
	p = sess.getParties()[0]

	// TODO(Joerger): Closing the host party's server context will result in the terminal
	// shell being killed, and the session ending for all parties. Once this bug is
	// fixed, we can re-enable this section of the test. For now just close the party.
	// https://github.com/gravitational/teleport/issues/46308
	//
	// require.NoError(t, p.ctx.Close())

	require.NoError(t, p.Close())

	partyIsRemoved = func() bool {
		return len(sess.getParties()) == 1 && !sess.isStopped()
	}
	require.Eventually(t, partyIsRemoved, time.Second*5, time.Millisecond*500)

	p.closeOnce.Do(func() {
		t.Fatalf("party should be closed already")
	})

	// If all parties are gone, the session should linger for a short duration.
	p = sess.getParties()[0]
	require.NoError(t, p.Close())
	require.False(t, sess.isStopped())

	// Wait for session to linger (time.Sleep)
	regClock.BlockUntil(2)

	// If a party connects to the lingering session, it will continue.
	testJoinSession(t, reg, sess.ID())
	require.Len(t, sess.getParties(), 1)

	// advance clock and give lingerAndDie goroutine a second to complete.
	regClock.Advance(defaults.SessionIdlePeriod)
	require.False(t, sess.isStopped())

	// If no parties remain it should be closed after the duration.
	p = sess.getParties()[0]
	require.NoError(t, p.Close())
	require.False(t, sess.isStopped())

	// Wait for session to linger (time.Sleep)
	regClock.BlockUntil(2)

	// Session should close.
	regClock.Advance(defaults.SessionIdlePeriod)
	require.Eventually(t, sess.isStopped, time.Second*5, time.Millisecond*500)
}

func TestSessionRecordingModes(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		desc                 string
		sessionRecordingMode constants.SessionRecordingMode
		expectClosedSession  bool
	}{
		{
			desc:                 "StrictMode",
			sessionRecordingMode: constants.SessionRecordingModeStrict,
			expectClosedSession:  true,
		},
		{
			desc:                 "BestEffortMode",
			sessionRecordingMode: constants.SessionRecordingModeBestEffort,
			expectClosedSession:  false,
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			srv := newMockServer(t)
			srv.component = teleport.ComponentNode

			reg, err := NewSessionRegistry(SessionRegistryConfig{
				Srv:                   srv,
				SessionTrackerService: srv.auth,
			})
			require.NoError(t, err)
			t.Cleanup(func() { reg.Close() })

			sess, sessCh := testOpenSession(t, reg, nil, decisionpb.SSHAccessPermit_builder{
				SessionRecordingMode: string(tt.sessionRecordingMode),
			}.Build())

			// Write to the session as a synchronization barrier.
			_, err = sessCh.Write([]byte("hello"))
			require.NoError(t, err)

			// Close the recorder, indicating there is some error.
			err = sess.Recorder().Complete(context.Background())
			require.NoError(t, err)

			// Write after completion. The recording failure may race the message if it
			// gets triggered by the initial "hello", so tolerate a closed pipe.
			if _, err = sessCh.Write([]byte("world")); err != nil {
				require.ErrorIs(t, err, io.ErrClosedPipe)
			}

			// Ensure the session is stopped.
			if !tt.expectClosedSession {
				sess.Stop()
			}

			// Wait until the session is stopped.
			require.Eventually(t, sess.isStopped, time.Second*5, time.Millisecond*500)

			// Wait until server receives all non-print events.
			require.EventuallyWithT(t, func(t *assert.CollectT) {
				// Events can appear in different orders. Use a set to track.
				eventsNotReceived := map[string]struct{}{
					events.SessionStartEvent: {},
					events.SessionLeaveEvent: {},
					events.SessionEndEvent:   {},
				}
				for _, e := range srv.Events() {
					delete(eventsNotReceived, e.GetType())
				}
				require.Empty(t, slices.Collect(maps.Keys(eventsNotReceived)))
			}, time.Second*5, time.Millisecond*500, "Some events not received")
		})
	}
}

// TestBPFEnabledAndNoPermitEvents tests that when ESR is enabled and no
// permit events are configured for the user, opening a non-interactive
// session does not attempt to open a ESR session.
func TestBPFEnabledAndNoPermitEvents(t *testing.T) {
	t.Parallel()

	srv := newMockServer(t)
	bpfSrv := &countingBPF{enabled: true}
	srv.bpf = bpfSrv

	reg, err := NewSessionRegistry(SessionRegistryConfig{
		Srv:                   srv,
		SessionTrackerService: srv.auth,
	})
	require.NoError(t, err)
	t.Cleanup(reg.Close)

	scx := newTestServerContext(t, reg.Srv, nil, &decisionpb.SSHAccessPermit{})
	execRequest := &mockExec{command: "true"}
	scx.execRequest = execRequest

	clientChan, serverChan := newMockSSHChannel(t)
	clientChan.Drain()
	require.NoError(t, reg.OpenExecSession(t.Context(), serverChan, scx))

	sess := scx.getSession()
	require.NotNil(t, sess)
	select {
	case <-sess.doneCh:
	case <-time.After(3 * time.Second):
		require.Fail(t, "exec session did not complete")
	}

	require.Zero(t, bpfSrv.openSessionCalls.Load())
	require.Zero(t, bpfSrv.closeSessionCalls.Load())
	require.False(t, sess.hasEnhancedRecording)
}

func TestStopSessionWithoutClientDisconnect(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name      string
		component string
		setTerm   func(*testing.T, *ServerContext)
	}{
		{
			name:      "local terminal",
			component: teleport.ComponentNode,
			setTerm: func(t *testing.T, scx *ServerContext) {
				term, err := newLocalTerminal(scx)
				require.NoError(t, err)
				scx.term = term
			},
		},
		{
			name:      "remote terminal",
			component: teleport.ComponentForwardingNode,
			setTerm: func(t *testing.T, scx *ServerContext) {
				term, err := newRemoteTerminal(scx)
				require.NoError(t, err)
				scx.term = term
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := newMockServer(t)
			srv.component = tt.component

			reg, err := NewSessionRegistry(SessionRegistryConfig{
				Srv:                   srv,
				SessionTrackerService: srv.auth,
			})
			require.NoError(t, err)
			t.Cleanup(func() { reg.Close() })

			scx := newTestServerContext(t, reg.Srv, nil, &decisionpb.SSHAccessPermit{})
			if tt.setTerm != nil {
				tt.setTerm(t, scx)
			}

			// Unlike real SSH clients, the mock SSH channel does not automatically close
			// when the session ends, which is the misbehavior this test is meant to ensure
			// is handled.
			clientChan, serverChan := newMockSSHChannel(t)
			clientChan.Drain()

			require.NoError(t, reg.OpenSession(t.Context(), serverChan, scx))
			require.NotNil(t, scx.party)

			stopDone := make(chan struct{})
			go func() {
				scx.party.s.Stop()
				close(stopDone)
			}()

			select {
			case <-stopDone:
			case <-time.After(5 * time.Second):
				require.Fail(t, "session Stop blocked while client channel remained open")
			}
		})
	}
}

func testOpenSession(t *testing.T, reg *SessionRegistry, sessionJoiningRoleSet services.RoleSet, accessPermit *decisionpb.SSHAccessPermit) (*session, ssh.Channel) {
	scx := newTestServerContext(t, reg.Srv, sessionJoiningRoleSet, accessPermit)

	clientChan, serverChan := newMockSSHChannel(t)
	clientChan.Drain()

	err := reg.OpenSession(t.Context(), serverChan, scx)
	require.NoError(t, err)

	require.NotNil(t, scx.party)
	return scx.party.s, clientChan
}

func testJoinSession(t *testing.T, reg *SessionRegistry, sid string) {
	scx := newTestServerContext(t, reg.Srv, nil, &decisionpb.SSHAccessPermit{})

	clientChan, serverChan := newMockSSHChannel(t)
	clientChan.Drain()

	err := reg.JoinSession(t.Context(), serverChan, scx, sid, types.SessionPeerMode)
	require.NoError(t, err)
}

type mockRecorder struct {
	events.SessionPreparerRecorder
	emitter eventstest.MockRecorderEmitter
	done    bool
}

func (m *mockRecorder) Done() <-chan struct{} {
	ch := make(chan struct{})
	if m.done {
		close(ch)
	}

	return ch
}

func (m *mockRecorder) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	return m.emitter.EmitAuditEvent(ctx, event)
}

type trackerService struct {
	created     atomic.Int32
	createError error
	services.SessionTrackerService
}

func (t *trackerService) CreatedCount() int {
	return int(t.created.Load())
}

func (t *trackerService) CreateSessionTracker(ctx context.Context, tracker types.SessionTracker) (types.SessionTracker, error) {
	t.created.Add(1)

	if t.createError != nil {
		return nil, t.createError
	}

	return t.SessionTrackerService.CreateSessionTracker(ctx, tracker)
}

type sessionEvaluator struct {
	moderated bool
	SessionAccessEvaluator
}

func (s sessionEvaluator) IsModerated() bool {
	return s.moderated
}

type mockExec struct {
	command string
}

func (s *mockExec) GetCommand() string {
	return s.command
}

func (s *mockExec) SetCommand(command string) {
	s.command = command
}

func (s *mockExec) Start(ctx context.Context, channel ssh.Channel) error {
	return nil
}

func (s *mockExec) Wait() ExecResult {
	return ExecResult{Command: s.command}
}

func (s *mockExec) ReadAuditSessionID() (uint32, error) {
	return 0, nil
}

func (s *mockExec) Continue() {}

func (s *mockExec) PID() int {
	return 0
}

type countingBPF struct {
	enabled           bool
	openSessionCalls  atomic.Int32
	closeSessionCalls atomic.Int32
}

func (c *countingBPF) OpenSession(ctx *bpf.SessionContext) error {
	c.openSessionCalls.Add(1)
	return nil
}

func (c *countingBPF) CloseSession(ctx *bpf.SessionContext) error {
	c.closeSessionCalls.Add(1)
	return nil
}

func (c *countingBPF) Close(restarting bool) error {
	return nil
}

func (c *countingBPF) Enabled() bool {
	return c.enabled
}

func (c *countingBPF) LostEvents() bpf.EventCount {
	return bpf.EventCount{}
}

func TestTrackingSession(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	me, err := user.Current()
	require.NoError(t, err)

	cases := []struct {
		name            string
		component       string
		recordingMode   string
		createError     error
		moderated       bool
		interactive     bool
		botUser         bool
		assertion       require.ErrorAssertionFunc
		createAssertion func(t *testing.T, count int)
	}{
		{
			name:          "node with proxy recording mode",
			component:     teleport.ComponentNode,
			recordingMode: types.RecordAtProxy,
			interactive:   true,
			assertion:     require.NoError,
			createAssertion: func(t *testing.T, count int) {
				require.Equal(t, 0, count)
			},
		},
		{
			name:          "node with node recording mode",
			component:     teleport.ComponentNode,
			recordingMode: types.RecordAtNode,
			interactive:   true,
			assertion:     require.NoError,
			createAssertion: func(t *testing.T, count int) {
				require.Equal(t, 1, count)
			},
		},
		{
			name:          "forwarding node with proxy recording mode",
			component:     teleport.ComponentForwardingNode,
			recordingMode: types.RecordAtProxy,
			interactive:   true,
			assertion:     require.NoError,
			createAssertion: func(t *testing.T, count int) {
				require.Equal(t, 1, count)
			},
		},
		{
			name:          "forwarding node with node recording mode (agentless)",
			component:     teleport.ComponentForwardingNode,
			recordingMode: types.RecordAtNode,
			interactive:   true,
			assertion:     require.NoError,
			createAssertion: func(t *testing.T, count int) {
				require.Equal(t, 1, count)
			},
		},
		{
			name:          "auth outage for non moderated session",
			component:     teleport.ComponentNode,
			recordingMode: types.RecordAtNodeSync,
			assertion:     require.NoError,
			interactive:   true,
			createError:   trace.ConnectionProblem(context.DeadlineExceeded, ""),
			createAssertion: func(t *testing.T, count int) {
				require.Equal(t, 1, count)
			},
		},
		{
			name:          "auth outage for moderated session",
			component:     teleport.ComponentNode,
			recordingMode: types.RecordAtNodeSync,
			moderated:     true,
			interactive:   true,
			assertion:     require.Error,
			createError:   trace.ConnectionProblem(context.DeadlineExceeded, ""),
			createAssertion: func(t *testing.T, count int) {
				require.Equal(t, 1, count)
			},
		},
		{
			name:          "bot session",
			component:     teleport.ComponentNode,
			recordingMode: types.RecordAtNode,
			interactive:   true,
			botUser:       true,
			assertion:     require.NoError,
			createAssertion: func(t *testing.T, count int) {
				require.Equal(t, 0, count)
			},
		},
		{
			name:          "non-interactive session",
			component:     teleport.ComponentNode,
			recordingMode: types.RecordAtNode,
			assertion:     require.NoError,
			createAssertion: func(t *testing.T, count int) {
				require.Equal(t, 0, count)
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			srv := newMockServer(t)
			srv.component = tt.component

			trackingService := &trackerService{
				SessionTrackerService: &mockSessiontrackerService{
					trackers: make(map[string]types.SessionTracker),
				},
				createError: tt.createError,
			}

			scx := newTestServerContext(t, srv, nil, &decisionpb.SSHAccessPermit{})
			scx.SessionRecordingConfig = &types.SessionRecordingConfigV2{
				Kind:    types.KindSessionRecordingConfig,
				Version: types.V2,
				Spec: types.SessionRecordingConfigSpecV2{
					Mode: tt.recordingMode,
				},
			}

			if tt.botUser {
				scx.Identity.BotName = "test-bot"
			}

			sess := &session{
				id:     rsession.NewID(),
				logger: logtest.With(teleport.ComponentKey, "test-session"),
				registry: &SessionRegistry{
					logger: logtest.NewLogger(),
					SessionRegistryConfig: SessionRegistryConfig{
						Srv:                   srv,
						SessionTrackerService: trackingService,
						clock:                 clockwork.NewFakeClock(), // use a fake clock to prevent the update loop from running
					},
				},
				serverMeta: apievents.ServerMetadata{
					ServerVersion:  teleport.Version,
					ServerHostname: "test",
					ServerID:       "123",
				},
				scx:       scx,
				serverCtx: ctx,
				login:     me.Name,
				access:    sessionEvaluator{moderated: tt.moderated},
			}

			p := &party{
				user: me.Name,
				id:   rsession.NewID(),
				mode: types.SessionPeerMode,
			}

			sessType := sessionTypeNonInteractive
			if tt.interactive {
				sessType = sessionTypeInteractive
			}

			err = sess.trackSession(ctx, me.Name, nil, p, sessType)
			tt.assertion(t, err)
			tt.createAssertion(t, trackingService.CreatedCount())
		})
	}
}

func TestSessionRecordingMode(t *testing.T) {
	tests := []struct {
		name          string
		serverSubKind string
		mode          string
		expectedMode  string
	}{
		{
			name:          "teleport node record at node",
			serverSubKind: types.SubKindTeleportNode,
			mode:          types.RecordAtNode,
			expectedMode:  types.RecordAtNode,
		},
		{
			name:          "teleport node record at proxy",
			serverSubKind: types.SubKindTeleportNode,
			mode:          types.RecordAtProxy,
			expectedMode:  types.RecordAtProxy,
		},
		{
			name:          "agentless node record at node",
			serverSubKind: types.SubKindOpenSSHNode,
			mode:          types.RecordAtNode,
			expectedMode:  types.RecordAtProxy,
		},
		{
			name:          "agentless node record at proxy",
			serverSubKind: types.SubKindOpenSSHNode,
			mode:          types.RecordAtProxy,
			expectedMode:  types.RecordAtProxy,
		},
		{
			name:          "agentless node record at node sync",
			serverSubKind: types.SubKindOpenSSHNode,
			mode:          types.RecordAtNodeSync,
			expectedMode:  types.RecordAtProxySync,
		},
		{
			name:          "agentless node record at proxy sync",
			serverSubKind: types.SubKindOpenSSHNode,
			mode:          types.RecordAtProxySync,
			expectedMode:  types.RecordAtProxySync,
		},
		{
			name:          "ec2 node record at node",
			serverSubKind: types.SubKindOpenSSHEICENode,
			mode:          types.RecordAtNode,
			expectedMode:  types.RecordAtProxy,
		},
		{
			name:          "ec2 node record at proxy",
			serverSubKind: types.SubKindOpenSSHEICENode,
			mode:          types.RecordAtProxy,
			expectedMode:  types.RecordAtProxy,
		},
		{
			name:          "ec2 node record at node sync",
			serverSubKind: types.SubKindOpenSSHEICENode,
			mode:          types.RecordAtNodeSync,
			expectedMode:  types.RecordAtProxySync,
		},
		{
			name:          "ec2 node record at proxy sync",
			serverSubKind: types.SubKindOpenSSHEICENode,
			mode:          types.RecordAtProxySync,
			expectedMode:  types.RecordAtProxySync,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sess := session{
				scx: &ServerContext{
					SessionRecordingConfig: &types.SessionRecordingConfigV2{
						Spec: types.SessionRecordingConfigSpecV2{
							Mode: tt.mode,
						},
					},
				},
				serverMeta: apievents.ServerMetadata{
					ServerSubKind: tt.serverSubKind,
				},
			}

			gotMode := sess.sessionRecordingLocation()
			require.Equal(t, tt.expectedMode, gotMode)
		})
	}
}

func TestCloseProxySession(t *testing.T) {
	ctx := t.Context()

	srv := newMockServer(t)
	srv.component = teleport.ComponentForwardingNode

	reg, err := NewSessionRegistry(SessionRegistryConfig{
		Srv:                   srv,
		SessionTrackerService: srv.auth,
	})
	require.NoError(t, err)
	t.Cleanup(func() { reg.Close() })

	scx := newTestServerContext(t, reg.Srv, nil, &decisionpb.SSHAccessPermit{})

	// Open a new session
	clientChan, serverChan := newMockSSHChannel(t)
	clientChan.Drain()
	err = reg.OpenSession(ctx, serverChan, scx)
	require.NoError(t, err)
	require.NotNil(t, scx.party)

	// After the session is open, we force a close coming from the server. Do
	// this inside a goroutine to avoid being blocked.
	closeChan := make(chan error)
	go func() {
		closeChan <- scx.party.s.Close()
	}()

	select {
	case err := <-closeChan:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		require.Fail(t, "expected session to be closed")
	}
}

// TestClodeRemoteSession given a remote session recording at proxy ensure that
// closing the session releases all the resources, and return properly to the
// user.
func TestCloseRemoteSession(t *testing.T) {
	ctx := t.Context()

	srv := newMockServer(t)
	srv.component = teleport.ComponentForwardingNode

	// init a session registry
	reg, _ := NewSessionRegistry(SessionRegistryConfig{
		Srv:                   srv,
		SessionTrackerService: srv.auth,
	})
	t.Cleanup(func() { reg.Close() })

	scx := newTestServerContext(t, reg.Srv, nil, &decisionpb.SSHAccessPermit{})
	scx.SessionRecordingConfig.SetMode(types.RecordAtProxy)
	scx.RemoteSession = mockSSHSession(t)

	// Open a new session
	clientChan, serverChan := newMockSSHChannel(t)
	clientChan.Drain()

	err := reg.OpenSession(ctx, serverChan, scx)
	require.NoError(t, err)
	require.NotNil(t, scx.party)

	// After the session is open, we force a close coming from the server. Do
	// this inside a goroutine to avoid being blocked.
	closeChan := make(chan error)
	go func() {
		closeChan <- scx.party.s.Close()
	}()

	select {
	case err := <-closeChan:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		require.Fail(t, "expected session to be closed")
	}
}

func mockSSHSession(t *testing.T) *tracessh.Session {
	t.Helper()

	ctx := t.Context()

	_, key, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(key)
	require.NoError(t, err)

	cfg := &ssh.ServerConfig{NoClientAuth: true}
	cfg.AddHostKey(signer)

	listener, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			t.Logf("error while accepting ssh connections: %s", err)
			return
		}

		srvConn, chCh, reqCh, err := ssh.NewServerConn(conn, cfg)
		if err != nil {
			t.Logf("error while accepting creating a new ssh server conn: %s", err)
			return
		}
		t.Cleanup(func() { srvConn.Close() })

		go ssh.DiscardRequests(reqCh)
		for newChannel := range chCh {
			channel, requests, err := newChannel.Accept()
			if err != nil {
				t.Logf("failed to accept channel: %s", err)
				continue
			}

			go func() {
				for req := range requests {
					req.Reply(true, nil)
				}
			}()

			sessTerm := term.NewTerminal(channel, "> ")
			go func() {
				defer channel.Close()
				for {
					_, err := sessTerm.ReadLine()
					if err != nil {
						break
					}
				}
			}()
		}
	}()

	// Establish a connection to the newly created server.
	sessCh := make(chan *tracessh.Session)
	go func() {
		client, err := apissh.Dial(ctx, listener.Addr().Network(), listener.Addr().String(), apissh.ClientConfig{
			Timeout: 10 * time.Second,
			User:    "user",
			PublicKeyAuth: apissh.PublicKeyAuthConfig{
				Signers: func() ([]ssh.Signer, error) {
					return []ssh.Signer{signer}, nil
				},
			},
			HostKeyCallback: ssh.FixedHostKey(signer.PublicKey()),
		})
		if err != nil {
			t.Logf("failed to dial test ssh server: %s", err)
			close(sessCh)
			return
		}
		t.Cleanup(func() { client.Close() })

		sess, err := client.NewSession(ctx)
		if err != nil {
			t.Logf("failed to dial test ssh server: %s", err)
			close(sessCh)
			return
		}
		t.Cleanup(func() { sess.Close() })

		sessCh <- sess
	}()

	select {
	case sess, ok := <-sessCh:
		require.True(t, ok, "expected SSH session but got nothing")
		return sess
	case <-time.After(10 * time.Second):
		require.Fail(t, "timeout while waiting for the SSH session")
		return nil
	}
}

func TestUpsertHostUser(t *testing.T) {
	username := "alice"

	cases := []struct {
		name string

		identityContext   IdentityContext
		hostUsers         *fakeHostUsersBackend
		createHostUser    bool
		obtainFallbackUID ObtainFallbackUIDFunc

		expectCreated     bool
		expectErrIs       error
		expectErrContains string
		expectUsers       map[string]fakeUser
	}{
		{
			name:           "should upsert existing user with permission",
			createHostUser: true,
			identityContext: IdentityContext{
				Login: username,
				AccessPermit: decisionpb.SSHAccessPermit_builder{
					HostUsersInfo: decisionpb.HostUsersInfo_builder{
						Groups: []string{"foo", "bar"},
					}.Build(),
				}.Build(),
			},
			hostUsers: &fakeHostUsersBackend{users: map[string]fakeUser{
				username: {},
			}},

			expectCreated: true,

			expectUsers: map[string]fakeUser{
				username: {groups: []string{"foo", "bar"}},
			},
		},
		{
			name:           "should upsert new user with permission",
			createHostUser: true,
			identityContext: IdentityContext{
				Login: username,
				AccessPermit: decisionpb.SSHAccessPermit_builder{
					HostUsersInfo: decisionpb.HostUsersInfo_builder{
						Groups: []string{"foo", "bar"},
					}.Build(),
				}.Build(),
			},
			hostUsers: &fakeHostUsersBackend{},

			expectCreated: true,
			expectUsers: map[string]fakeUser{
				username: {groups: []string{"foo", "bar"}},
			},
		},
		{
			name:           "should not upsert existing user without permission",
			createHostUser: true,
			identityContext: IdentityContext{
				Login: username,
				AccessPermit: decisionpb.SSHAccessPermit_builder{
					HostUsersInfo: nil,
				}.Build(),
			},
			hostUsers: &fakeHostUsersBackend{
				users: map[string]fakeUser{
					username: {},
				},
			},

			expectCreated: false,
			expectErrIs:   errHostUserCreationNotAuthorized,
			expectUsers: map[string]fakeUser{
				username: {},
			},
		},
		{
			name:           "should not upsert new user without permission",
			createHostUser: true,
			identityContext: IdentityContext{
				Login: username,
				AccessPermit: decisionpb.SSHAccessPermit_builder{
					HostUsersInfo: nil,
				}.Build(),
			},
			hostUsers: &fakeHostUsersBackend{},

			expectCreated: false,
			expectUsers:   nil,
			expectErrIs:   errHostUserCreationNotAuthorized,
		},
		{
			name:            "should do nothing if login is session join principal",
			createHostUser:  true,
			identityContext: IdentityContext{Login: teleport.SSHSessionJoinPrincipal},
			hostUsers:       &fakeHostUsersBackend{},

			expectCreated: false,
			expectUsers:   nil,
		},
		{
			name:           "should use fallback UIDs in keep mode",
			createHostUser: true,
			identityContext: IdentityContext{
				Login: username,
				AccessPermit: decisionpb.SSHAccessPermit_builder{
					HostUsersInfo: decisionpb.HostUsersInfo_builder{
						Mode: decisionpb.HostUserMode_HOST_USER_MODE_KEEP,
					}.Build(),
				}.Build(),
			},
			hostUsers: &fakeHostUsersBackend{},
			obtainFallbackUID: func(ctx context.Context, username string) (uid int32, ok bool, _ error) {
				return 1, true, nil
			},

			expectCreated: true,
			expectUsers: map[string]fakeUser{
				username: {uid: "1", gid: "1"},
			},
		},
		{
			name:           "should only use fallback UIDs in keep mode",
			createHostUser: true,
			identityContext: IdentityContext{
				Login: username,
				AccessPermit: decisionpb.SSHAccessPermit_builder{
					HostUsersInfo: decisionpb.HostUsersInfo_builder{
						Mode: decisionpb.HostUserMode_HOST_USER_MODE_DROP,
					}.Build(),
				}.Build(),
			},
			hostUsers: &fakeHostUsersBackend{},
			obtainFallbackUID: func(ctx context.Context, username string) (uid int32, ok bool, _ error) {
				return 0, false, trace.BadParameter("not reached")
			},

			expectCreated: true,
			expectUsers: map[string]fakeUser{
				username: {},
			},
		},
		{
			name:           "should only use fallback UIDs for users that don't exist",
			createHostUser: true,
			identityContext: IdentityContext{
				Login: username,
				AccessPermit: decisionpb.SSHAccessPermit_builder{
					HostUsersInfo: decisionpb.HostUsersInfo_builder{
						Mode: decisionpb.HostUserMode_HOST_USER_MODE_KEEP,
					}.Build(),
				}.Build(),
			},
			hostUsers: &fakeHostUsersBackend{
				users: map[string]fakeUser{
					username: {},
				},
			},
			obtainFallbackUID: func(ctx context.Context, username string) (uid int32, ok bool, _ error) {
				return 0, false, trace.BadParameter("not reached")
			},

			expectCreated: true,
			expectUsers: map[string]fakeUser{
				username: {},
			},
		},
		{
			name:           "should not override a configured GID",
			createHostUser: true,
			identityContext: IdentityContext{
				Login: username,
				AccessPermit: decisionpb.SSHAccessPermit_builder{
					HostUsersInfo: decisionpb.HostUsersInfo_builder{
						Mode: decisionpb.HostUserMode_HOST_USER_MODE_KEEP,
						Gid:  "set",
					}.Build(),
				}.Build(),
			},
			hostUsers: &fakeHostUsersBackend{},
			obtainFallbackUID: func(ctx context.Context, username string) (uid int32, ok bool, _ error) {
				return 1, true, nil
			},

			expectCreated: true,
			expectUsers: map[string]fakeUser{
				username: {uid: "1", gid: "set"},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			registry := SessionRegistry{
				logger: logtest.NewLogger(),
				SessionRegistryConfig: SessionRegistryConfig{
					Srv: &fakeServer{createHostUser: c.createHostUser},
				},
				users: c.hostUsers,
			}

			userCreated, _, err := registry.UpsertHostUser(c.identityContext, c.obtainFallbackUID)

			if c.expectErrIs != nil {
				assert.ErrorIs(t, err, c.expectErrIs)
			}

			if c.expectErrContains != "" {
				assert.Contains(t, err.Error(), c.expectErrContains)
			}

			if c.expectErrIs == nil && c.expectErrContains == "" {
				assert.NoError(t, err)
			}

			assert.Equal(t, c.expectCreated, userCreated)

			for name, user := range c.hostUsers.users {
				expectedUser, ok := c.expectUsers[name]
				assert.True(t, ok, "user must be present in expected users")
				assert.ElementsMatch(t, expectedUser.groups, user.groups)
				assert.Equal(t, expectedUser.uid, user.uid)
				assert.Equal(t, expectedUser.gid, user.gid)
			}
		})
	}
}

func TestWriteSudoersFile(t *testing.T) {
	username := "alice"

	cases := []struct {
		name string

		identityContext IdentityContext
		hostSudoers     *fakeSudoersBackend

		expectSudoers     map[string][]string
		expectErrIs       error
		expectErrContains string
	}{
		{
			name: "should write sudoers with permission",
			identityContext: IdentityContext{
				Login: username,
				AccessPermit: decisionpb.SSHAccessPermit_builder{
					HostSudoers: []string{"foo", "bar"},
				}.Build(),
			},
			hostSudoers: &fakeSudoersBackend{},

			expectSudoers: map[string][]string{
				username: {"foo", "bar"},
			},
		},
		{
			name: "should do nothing if no sudoers defined",
			identityContext: IdentityContext{
				Login: username,
				AccessPermit: decisionpb.SSHAccessPermit_builder{
					HostSudoers: nil,
				}.Build(),
			},
			hostSudoers: &fakeSudoersBackend{},

			expectSudoers: map[string][]string{},
		},
		{
			name: "should do nothing for session join principal",
			identityContext: IdentityContext{
				Login: teleport.SSHSessionJoinPrincipal,
				AccessPermit: decisionpb.SSHAccessPermit_builder{
					HostSudoers: []string{"foo", "bar"}, // should not be written
				}.Build(),
			},
			hostSudoers: &fakeSudoersBackend{},

			expectSudoers: map[string][]string{},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			registry := SessionRegistry{
				logger: logtest.NewLogger(),
				SessionRegistryConfig: SessionRegistryConfig{
					Srv: &fakeServer{hostSudoers: c.hostSudoers},
				},
				sessionsByUser: &userSessions{
					sessionsByUser: make(map[string]int),
				},
			}

			_, err := registry.WriteSudoersFile(c.identityContext)

			if c.expectErrIs != nil {
				assert.ErrorIs(t, err, c.expectErrIs)
			}

			if c.expectErrContains != "" {
				assert.Contains(t, err.Error(), c.expectErrContains)
			}

			if c.expectErrIs == nil && c.expectErrContains == "" {
				assert.NoError(t, err)
			}

			for name, sudoers := range c.hostSudoers.sudoers {
				expectedSudoers, ok := c.expectSudoers[name]
				assert.True(t, ok, "there should be an expected name for each login name")
				assert.ElementsMatch(t, expectedSudoers, sudoers)
			}
		})
	}
}

type fakeServer struct {
	Server

	createHostUser bool
	hostSudoers    HostSudoers
}

func (f *fakeServer) GetCreateHostUser() bool {
	return f.createHostUser
}

func (f *fakeServer) GetHostSudoers() HostSudoers {
	return f.hostSudoers
}

func (f *fakeServer) GetInfo() types.Server {
	return nil
}

func (f *fakeServer) Context() context.Context {
	return context.Background()
}

type fakeUser struct {
	groups []string
	uid    string
	gid    string
}

type fakeHostUsersBackend struct {
	HostUsers

	users map[string]fakeUser
}

func (f *fakeHostUsersBackend) UpsertUser(name string, hostRoleInfo *decisionpb.HostUsersInfo, opts ...UpsertHostUserOption) (io.Closer, error) {
	if f.users == nil {
		f.users = make(map[string]fakeUser)
	}

	f.users[name] = fakeUser{
		groups: hostRoleInfo.GetGroups(),
		uid:    hostRoleInfo.GetUid(),
		gid:    hostRoleInfo.GetGid(),
	}
	return nil, nil
}

func (f *fakeHostUsersBackend) UserExists(name string) error {
	if _, exists := f.users[name]; !exists {
		return trace.NotFound("%v", name)
	}

	return nil
}

type fakeSudoersBackend struct {
	sudoers map[string][]string
	err     error
}

func (f *fakeSudoersBackend) WriteSudoers(name string, sudoers []string) error {
	if f.sudoers == nil {
		f.sudoers = make(map[string][]string)
	}

	f.sudoers[name] = append(f.sudoers[name], sudoers...)
	return f.err
}

func (f *fakeSudoersBackend) RemoveSudoers(name string) error {
	if f.sudoers == nil {
		return nil
	}

	delete(f.sudoers, name)
	return f.err
}

func TestServerContextEmitters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		component     string
		recordingMode string
		wantDiscard   bool
	}{
		{
			name:          "node component, node recording",
			component:     teleport.ComponentNode,
			recordingMode: types.RecordAtNode,
			wantDiscard:   false,
		},
		{
			name:          "node component, proxy recording",
			component:     teleport.ComponentNode,
			recordingMode: types.RecordAtProxy,
			wantDiscard:   true,
		},
		{
			name:          "node component, proxy-sync recording",
			component:     teleport.ComponentNode,
			recordingMode: types.RecordAtProxySync,
			wantDiscard:   true,
		},
		{
			name:          "forwarding node component, proxy recording",
			component:     teleport.ComponentForwardingNode,
			recordingMode: types.RecordAtProxy,
			wantDiscard:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
				Mode: tt.recordingMode,
			})
			require.NoError(t, err)

			srv := &mockServer{component: tt.component}
			scx := &ServerContext{
				SessionRecordingConfig: recConfig,
				srv:                    srv,
			}

			// BPFEmitter must always be the underlying server so ESR events
			// reach the audit log regardless of cluster recording mode.
			require.Same(t, srv, scx.BPFEmitter())

			if tt.wantDiscard {
				require.IsType(t, (*events.DiscardAuditLog)(nil), scx.AuditEmitter())
			} else {
				require.Same(t, srv, scx.AuditEmitter())
			}
		})
	}
}

func TestHandleForceTerminate(t *testing.T) {
	t.Parallel()

	srv := newMockServer(t)
	reg, err := NewSessionRegistry(SessionRegistryConfig{
		Srv:                   srv,
		SessionTrackerService: srv.auth,
	})
	require.NoError(t, err)
	t.Cleanup(func() { reg.Close() })

	termHandlers := &TermHandlers{SessionRegistry: reg}

	tests := []struct {
		name         string
		joinMode     types.SessionParticipantMode
		accessDenied bool
	}{
		{
			name:         "empty join mode denied",
			accessDenied: true,
		},
		{
			name:         "peer denied",
			joinMode:     types.SessionPeerMode,
			accessDenied: true,
		},
		{
			name:         "observer denied",
			joinMode:     types.SessionObserverMode,
			accessDenied: true,
		},
		{
			name:     "moderator allowed",
			joinMode: types.SessionModeratorMode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hostSess, _ := testOpenSession(t, reg, nil, &decisionpb.SSHAccessPermit{})
			t.Cleanup(hostSess.Stop)

			hostSess.scx.party.mode = tt.joinMode

			err := termHandlers.HandleForceTerminate(nil, nil, hostSess.scx)
			if tt.accessDenied {
				require.True(t, trace.IsAccessDenied(err), "expected AccessDenied, got %v", err)
				return
			}
			require.NoError(t, err)
		})
	}
}
