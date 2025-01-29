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
	"net"
	"os/user"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils/sftp"
	"github.com/gravitational/teleport/lib/utils"
)

func TestParseAccessRequestIDs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		input     string
		comment   string
		result    []string
		assertErr require.ErrorAssertionFunc
	}{
		{
			input:     `{"access_requests":["1a7483e0-575a-4bd1-9faa-022500a49325","30b344f5-d1ba-49fc-b2aa-b04234d0a4ec"]}`,
			comment:   "complete valid input",
			assertErr: require.NoError,
			result:    []string{"1a7483e0-575a-4bd1-9faa-022500a49325", "30b344f5-d1ba-49fc-b2aa-b04234d0a4ec"},
		},
		{
			input:     `{"access_requests":["1a7483e0-575a-4bd1-9faa-022500a49325","30b344f5-d1ba-49fc-b2aa"]}`,
			comment:   "invalid uuid",
			assertErr: require.Error,
			result:    nil,
		},
		{
			input:     `{"access_requests":[nil,"30b344f5-d1ba-49fc-b2aa-b04234d0a4ec"]}`,
			comment:   "invalid value, value in slice is nil",
			assertErr: require.Error,
			result:    nil,
		},
		{
			input:     `{"access_requests":nil}`,
			comment:   "invalid value, whole value is nil",
			assertErr: require.Error,
			result:    nil,
		},
	}
	for _, tt := range testCases {
		t.Run(tt.comment, func(t *testing.T) {
			out, err := ParseAccessRequestIDs(tt.input)
			tt.assertErr(t, err)
			require.Equal(t, out, tt.result)
		})
	}
}

func TestIsApprovedFileTransfer(t *testing.T) {
	// set enterprise for tests
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})
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
	auditScx := newTestServerContext(t, reg.Srv, auditorRoleSet)
	// change the teleport user so we don't match the user in the test cases
	auditScx.Identity.TeleportUser = "mod"
	auditSess, _ := testOpenSession(t, reg, auditorRoleSet)
	approvers := make(map[string]*party)
	auditChan := newMockSSHChannel()
	approvers["mod"] = newParty(auditSess, types.SessionModeratorMode, auditChan, auditScx)

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
		req            *FileTransferRequest
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
			req: &FileTransferRequest{
				ID:        "123",
				Requester: "michael",
				approvers: make(map[string]*party),
			},
		},
		{
			name:           "current request location does not match original location",
			expectedResult: false,
			expectedError:  "requested destination path does not match the current request",
			reqID:          "123",
			location:       "~/Downloads",
			req: &FileTransferRequest{
				ID:        "123",
				Requester: "teleportUser",
				approvers: make(map[string]*party),
				Location:  "~/badlocation",
			},
		},
		{
			name:           "approved request",
			expectedResult: true,
			expectedError:  "",
			reqID:          "123",
			location:       "~/Downloads",
			req: &FileTransferRequest{
				ID:        "123",
				Requester: "teleportUser",
				approvers: approvers,
				Location:  "~/Downloads",
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			// create and add a session to the registry
			sess, _ := testOpenSession(t, reg, accessRoleSet)

			// create a FileTransferRequest. can be nil
			sess.fileTransferReq = tt.req

			// new exec request context
			scx := newTestServerContext(t, reg.Srv, accessRoleSet)
			scx.SetEnv(string(sftp.ModeratedSessionID), sess.ID())
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

	logger := utils.NewSlogLoggerForTests()

	isNotSessionWriter := func(t require.TestingT, i interface{}, i2 ...interface{}) {
		require.NotNil(t, i)
		_, ok := i.(*events.SessionWriter)
		require.False(t, ok)
	}

	cases := []struct {
		desc         string
		sess         *session
		sctx         *ServerContext
		errAssertion require.ErrorAssertionFunc
		recAssertion require.ValueAssertionFunc
	}{
		{
			desc: "discard-stream-when-proxy-recording",
			sess: &session{
				id:     "test",
				logger: logger,
				registry: &SessionRegistry{
					logger: utils.NewSlogLoggerForTests(),
					SessionRegistryConfig: SessionRegistryConfig{
						Srv: &mockServer{
							component: teleport.ComponentNode,
						},
					},
				},
			},
			sctx: &ServerContext{
				SessionRecordingConfig: proxyRecording,
				term:                   &terminal{},
			},
			errAssertion: require.NoError,
			recAssertion: isNotSessionWriter,
		},
		{
			desc: "discard-stream--when-proxy-sync-recording",
			sess: &session{
				id:     "test",
				logger: logger,
				registry: &SessionRegistry{
					logger: utils.NewSlogLoggerForTests(),
					SessionRegistryConfig: SessionRegistryConfig{
						Srv: &mockServer{
							component: teleport.ComponentNode,
						},
					},
				},
			},
			sctx: &ServerContext{
				SessionRecordingConfig: proxyRecordingSync,
				term:                   &terminal{},
			},
			errAssertion: require.NoError,
			recAssertion: isNotSessionWriter,
		},
		{
			desc: "strict-err-new-audit-writer-fails",
			sess: &session{
				id:     "test",
				logger: logger,
				registry: &SessionRegistry{
					logger: utils.NewSlogLoggerForTests(),
					SessionRegistryConfig: SessionRegistryConfig{
						Srv: &mockServer{
							component: teleport.ComponentNode,
						},
					},
				},
			},
			sctx: &ServerContext{
				SessionRecordingConfig: nodeRecordingSync,
				srv: &mockServer{
					component: teleport.ComponentNode,
				},
				term: &terminal{},
				Identity: IdentityContext{
					AccessChecker: services.NewAccessCheckerWithRoleSet(&services.AccessInfo{
						Roles: []string{"dev"},
					}, "test", services.RoleSet{
						&types.RoleV6{
							Metadata: types.Metadata{Name: "dev", Namespace: apidefaults.Namespace},
							Spec: types.RoleSpecV6{
								Options: types.RoleOptions{
									RecordSession: &types.RecordSession{
										SSH: constants.SessionRecordingModeStrict,
									},
								},
							},
						},
					}),
				},
			},
			errAssertion: require.Error,
			recAssertion: require.Nil,
		},
		{
			desc: "best-effort-err-new-audit-writer-succeeds",
			sess: &session{
				id:     "test",
				logger: logger,
				registry: &SessionRegistry{
					logger: utils.NewSlogLoggerForTests(),
					SessionRegistryConfig: SessionRegistryConfig{
						Srv: &mockServer{
							component: teleport.ComponentNode,
						},
					},
				},
			},
			sctx: &ServerContext{
				ClusterName:            "test",
				SessionRecordingConfig: nodeRecordingSync,
				srv: &mockServer{
					component: teleport.ComponentNode,
					datadir:   t.TempDir(),
				},
				Identity: IdentityContext{
					AccessChecker: services.NewAccessCheckerWithRoleSet(&services.AccessInfo{
						Roles: []string{"dev"},
					}, "test", services.RoleSet{
						&types.RoleV6{
							Metadata: types.Metadata{Name: "dev", Namespace: apidefaults.Namespace},
							Spec: types.RoleSpecV6{
								Options: types.RoleOptions{
									RecordSession: &types.RecordSession{
										SSH: constants.SessionRecordingModeBestEffort,
									},
								},
							},
						},
					}),
				},
				term: &terminal{},
			},
			errAssertion: require.NoError,
			recAssertion: func(t require.TestingT, i interface{}, _ ...interface{}) {
				require.NotNil(t, i)
				sw, ok := i.(apievents.Stream)
				require.True(t, ok)
				require.NoError(t, sw.Close(context.Background()))
			},
		},
		{
			desc: "audit-writer",
			sess: &session{
				id:     "test",
				logger: logger,
				registry: &SessionRegistry{
					logger: utils.NewSlogLoggerForTests(),
					SessionRegistryConfig: SessionRegistryConfig{
						Srv: &mockServer{
							component: teleport.ComponentNode,
						},
					},
				},
			},
			sctx: &ServerContext{
				ClusterName:            "test",
				SessionRecordingConfig: nodeRecordingSync,
				srv: &mockServer{
					MockRecorderEmitter: &eventstest.MockRecorderEmitter{},
					datadir:             t.TempDir(),
				},
				term: &terminal{},
			},
			errAssertion: require.NoError,
			recAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.NotNil(t, i)
				sw, ok := i.(apievents.Stream)
				require.True(t, ok)
				require.NoError(t, sw.Close(context.Background()))
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.desc, func(t *testing.T) {
			rec, err := newRecorder(tt.sess, tt.sctx)
			tt.errAssertion(t, err)
			tt.recAssertion(t, rec)
		})
	}
}

func TestSession_emitAuditEvent(t *testing.T) {
	t.Parallel()

	logger := utils.NewSlogLoggerForTests()

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
			scx:      newTestServerContext(t, srv, nil),
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
	scx := newTestServerContext(t, reg.Srv, nil)
	rcfg := types.DefaultSessionRecordingConfig()
	rcfg.SetMode(types.RecordAtNodeSync)
	scx.SessionRecordingConfig = rcfg

	// Allocate a terminal for the session so that
	// events are properly recorded.
	terminal, err := newLocalTerminal(scx)
	require.NoError(t, err)
	scx.term = terminal

	// Open a new session
	sshChanOpen := newMockSSHChannel()
	go func() {
		// Consume stdout sent to the channel
		io.ReadAll(sshChanOpen)
	}()
	require.NoError(t, reg.OpenSession(context.Background(), sshChanOpen, scx))
	require.NotNil(t, scx.session)

	// Simulate changing window size to capture an additional event.
	require.NoError(t, reg.NotifyWinChange(context.Background(), rsession.TerminalParams{W: 100, H: 100}, scx))

	// Stopping the session should trigger the session
	// to end and cleanup in the background
	scx.session.Stop()

	// Wait for the session to be removed from the registry.
	require.Eventually(t, func() bool {
		_, found := reg.findSession(scx.session.id)
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
		scx := newTestServerContext(t, reg.Srv, nil)
		rcfg := types.DefaultSessionRecordingConfig()
		rcfg.SetMode(types.RecordAtNodeSync)
		scx.SessionRecordingConfig = rcfg

		// Modify the execRequest to actually execute a command.
		scx.execRequest = &localExec{Ctx: scx, Command: "true"}

		// Open a new session
		sshChanOpen := newMockSSHChannel()
		go func() {
			// Consume stdout sent to the channel
			io.ReadAll(sshChanOpen)
		}()
		require.NoError(t, reg.OpenExecSession(context.Background(), sshChanOpen, scx))
		require.NotNil(t, scx.session)

		// Wait for the command execution to complete and the session to be terminated.
		require.Eventually(t, func() bool {
			_, found := reg.findSession(scx.session.id)
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
		scx := newTestServerContext(t, reg.Srv, nil)
		rcfg := types.DefaultSessionRecordingConfig()
		rcfg.SetMode(types.RecordAtNodeSync)
		scx.SessionRecordingConfig = rcfg

		// Modify the execRequest to actually execute a command.
		scx.execRequest = &localExec{Ctx: scx, Command: "true"}

		// Open a new session
		sshChanOpen := newMockSSHChannel()
		go func() {
			// Consume stdout sent to the channel
			io.ReadAll(sshChanOpen)
		}()
		require.NoError(t, reg.OpenExecSession(context.Background(), sshChanOpen, scx))
		require.NotNil(t, scx.session)

		// Wait for the command execution to complete and the session to be terminated.
		require.Eventually(t, func() bool {
			_, found := reg.findSession(scx.session.id)
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
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})
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
	sess, _ := testOpenSession(t, reg, roles)

	// Stopping the session should trigger the session
	// to end and cleanup in the background
	sess.Stop()

	sessionClosed := func() bool {
		_, found := reg.findSession(sess.id)
		return !found
	}
	require.Eventually(t, sessionClosed, time.Second*15, time.Millisecond*500)
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
	sess, _ := testOpenSession(t, reg, nil)
	require.Len(t, sess.getParties(), 1)
	testJoinSession(t, reg, sess)
	require.Len(t, sess.getParties(), 2)
	testJoinSession(t, reg, sess)
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
	testJoinSession(t, reg, sess)
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

func testJoinSession(t *testing.T, reg *SessionRegistry, sess *session) {
	scx := newTestServerContext(t, reg.Srv, nil)
	sshChanOpen := newMockSSHChannel()
	scx.setSession(context.Background(), sess, sshChanOpen)

	// Open a new session
	go func() {
		// Consume stdout sent to the channel
		io.ReadAll(sshChanOpen)
	}()

	err := reg.OpenSession(context.Background(), sshChanOpen, scx)
	require.NoError(t, err)
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

			sess, sessCh := testOpenSession(t, reg, services.RoleSet{
				&types.RoleV6{
					Metadata: types.Metadata{Name: "dev", Namespace: apidefaults.Namespace},
					Spec: types.RoleSpecV6{
						Options: types.RoleOptions{
							RecordSession: &types.RecordSession{
								SSH: tt.sessionRecordingMode,
							},
						},
					},
				},
			})

			// Write stuff in the session
			_, err = sessCh.Write([]byte("hello"))
			require.NoError(t, err)

			// Close the recorder, indicating there is some error.
			err = sess.Recorder().Complete(context.Background())
			require.NoError(t, err)

			// Send more writes.
			_, err = sessCh.Write([]byte("world"))
			require.NoError(t, err)

			// Ensure the session is stopped.
			if !tt.expectClosedSession {
				sess.Stop()
			}

			// Wait until the session is stopped.
			require.Eventually(t, sess.isStopped, time.Second*5, time.Millisecond*500)

			// Wait until server receives all non-print events.
			checkEventsReceived := func() bool {
				expectedEventTypes := []string{
					events.SessionStartEvent,
					events.SessionLeaveEvent,
					events.SessionEndEvent,
				}

				emittedEvents := srv.Events()
				if len(emittedEvents) != len(expectedEventTypes) {
					return false
				}

				// Events can appear in different orders. Use a set to track.
				eventsNotReceived := utils.StringsSet(expectedEventTypes)
				for _, e := range emittedEvents {
					delete(eventsNotReceived, e.GetType())
				}
				return len(eventsNotReceived) == 0
			}
			require.Eventually(t, checkEventsReceived, time.Second*5, time.Millisecond*500, "Some events are not received.")
		})
	}
}

func testOpenSession(t *testing.T, reg *SessionRegistry, roleSet services.RoleSet) (*session, ssh.Channel) {
	scx := newTestServerContext(t, reg.Srv, roleSet)

	// Open a new session
	sshChanOpen := newMockSSHChannel()
	go func() {
		// Consume stdout sent to the channel
		io.ReadAll(sshChanOpen)
	}()

	err := reg.OpenSession(context.Background(), sshChanOpen, scx)
	require.NoError(t, err)

	require.NotNil(t, scx.session)
	return scx.session, sshChanOpen
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

func TestTrackingSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

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
			name:          "proxy with proxy recording mode",
			component:     teleport.ComponentProxy,
			recordingMode: types.RecordAtProxy,
			interactive:   true,
			assertion:     require.NoError,
			createAssertion: func(t *testing.T, count int) {
				require.Equal(t, 1, count)
			},
		},
		{
			name:          "proxy with node recording mode",
			component:     teleport.ComponentProxy,
			recordingMode: types.RecordAtNode,
			interactive:   true,
			assertion:     require.NoError,
			createAssertion: func(t *testing.T, count int) {
				require.Equal(t, 0, count)
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

			scx := newTestServerContext(t, srv, nil)
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
				logger: utils.NewSlogLoggerForTests().With(teleport.ComponentKey, "test-session"),
				registry: &SessionRegistry{
					logger: utils.NewSlogLoggerForTests(),
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

			gotMode := sess.sessionRecordingMode()
			require.Equal(t, tt.expectedMode, gotMode)
		})
	}
}

func TestCloseProxySession(t *testing.T) {
	srv := newMockServer(t)
	srv.component = teleport.ComponentProxy

	reg, err := NewSessionRegistry(SessionRegistryConfig{
		Srv:                   srv,
		SessionTrackerService: srv.auth,
	})
	require.NoError(t, err)
	t.Cleanup(func() { reg.Close() })

	scx := newTestServerContext(t, reg.Srv, nil)

	// Open a new session
	sshChanOpen := newMockSSHChannel()
	// Always close the session from the client side to avoid it being stuck
	// on closing (server side).
	t.Cleanup(func() { sshChanOpen.Close() })
	go func() {
		// Consume stdout sent to the channel
		io.ReadAll(sshChanOpen)
	}()

	err = reg.OpenSession(context.Background(), sshChanOpen, scx)
	require.NoError(t, err)
	require.NotNil(t, scx.session)

	// After the session is open, we force a close coming from the server. Do
	// this inside a goroutine to avoid being blocked.
	closeChan := make(chan error)
	go func() {
		closeChan <- scx.session.Close()
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
	srv := newMockServer(t)
	srv.component = teleport.ComponentProxy

	// init a session registry
	reg, _ := NewSessionRegistry(SessionRegistryConfig{
		Srv:                   srv,
		SessionTrackerService: srv.auth,
	})
	t.Cleanup(func() { reg.Close() })

	scx := newTestServerContext(t, reg.Srv, nil)
	scx.SessionRecordingConfig.SetMode(types.RecordAtProxy)
	scx.RemoteSession = mockSSHSession(t)

	// Open a new session
	sshChanOpen := newMockSSHChannel()
	// Always close the session from the client side to avoid it being stuck
	// on closing (server side).
	t.Cleanup(func() { sshChanOpen.Close() })
	go func() {
		// Consume stdout sent to the channel
		io.ReadAll(sshChanOpen)
	}()

	err := reg.OpenSession(context.Background(), sshChanOpen, scx)
	require.NoError(t, err)
	require.NotNil(t, scx.session)

	// After the session is open, we force a close coming from the server. Do
	// this inside a goroutine to avoid being blocked.
	closeChan := make(chan error)
	go func() {
		closeChan <- scx.session.Close()
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

	ctx := context.Background()

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
		client, err := tracessh.Dial(ctx, listener.Addr().Network(), listener.Addr().String(), &ssh.ClientConfig{
			Timeout:         10 * time.Second,
			User:            "user",
			Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
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
				AccessChecker: &fakeAccessChecker{
					hostInfo: services.HostUsersInfo{
						Groups: []string{"foo", "bar"},
					},
				},
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
				AccessChecker: &fakeAccessChecker{
					hostInfo: services.HostUsersInfo{
						Groups: []string{"foo", "bar"},
					},
				},
			},
			hostUsers: &fakeHostUsersBackend{},

			expectCreated: true,
			expectUsers: map[string]fakeUser{
				username: {groups: []string{"foo", "bar"}},
			},
		},
		{
			name:            "should not upsert existing user without permission",
			createHostUser:  true,
			identityContext: IdentityContext{Login: username, AccessChecker: &fakeAccessChecker{err: trace.AccessDenied("test")}},
			hostUsers: &fakeHostUsersBackend{
				users: map[string]fakeUser{
					username: {},
				},
			},

			expectCreated: false,
			expectErrIs:   trace.AccessDenied("test"),
			expectUsers: map[string]fakeUser{
				username: {},
			},
		},
		{
			name:            "should not upsert new user without permission",
			createHostUser:  true,
			identityContext: IdentityContext{Login: username, AccessChecker: &fakeAccessChecker{err: trace.AccessDenied("test")}},
			hostUsers:       &fakeHostUsersBackend{},

			expectCreated:     false,
			expectUsers:       nil,
			expectErrIs:       trace.AccessDenied("test"),
			expectErrContains: "insufficient permissions for host user creation",
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
				AccessChecker: &fakeAccessChecker{
					hostInfo: services.HostUsersInfo{
						Mode: services.HostUserModeKeep,
					},
				},
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
				AccessChecker: &fakeAccessChecker{
					hostInfo: services.HostUsersInfo{
						Mode: services.HostUserModeDrop,
					},
				},
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
				AccessChecker: &fakeAccessChecker{
					hostInfo: services.HostUsersInfo{
						Mode: services.HostUserModeKeep,
					},
				},
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
				AccessChecker: &fakeAccessChecker{
					hostInfo: services.HostUsersInfo{
						Mode: services.HostUserModeKeep,
						GID:  "set",
					},
				},
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
				logger: utils.NewSlogLoggerForTests(),
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
			name:            "should write sudoers with permission",
			identityContext: IdentityContext{Login: username, AccessChecker: &fakeAccessChecker{}},
			hostSudoers:     &fakeSudoersBackend{},

			expectSudoers: map[string][]string{
				username: {"foo", "bar"},
			},
		},
		{
			name:            "should not write sudoers without permission",
			identityContext: IdentityContext{Login: username, AccessChecker: &fakeAccessChecker{err: trace.AccessDenied("test")}},
			hostSudoers:     &fakeSudoersBackend{},

			expectSudoers: map[string][]string{},
			expectErrIs:   trace.AccessDenied("test"),
		},
		{
			name:            "should do nothing for session join principal",
			identityContext: IdentityContext{Login: teleport.SSHSessionJoinPrincipal, AccessChecker: &fakeAccessChecker{}},
			hostSudoers:     &fakeSudoersBackend{},

			expectSudoers: map[string][]string{},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			registry := SessionRegistry{
				logger: utils.NewSlogLoggerForTests(),
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

type fakeAccessChecker struct {
	services.AccessChecker
	err      error
	hostInfo services.HostUsersInfo
}

func (f *fakeAccessChecker) HostSudoers(srv types.Server) ([]string, error) {
	return []string{"foo", "bar"}, f.err
}

func (f *fakeAccessChecker) HostUsers(srv types.Server) (*services.HostUsersInfo, error) {
	return &f.hostInfo, f.err
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

func (f *fakeHostUsersBackend) UpsertUser(name string, hostRoleInfo services.HostUsersInfo) (io.Closer, error) {
	if f.users == nil {
		f.users = make(map[string]fakeUser)
	}

	f.users[name] = fakeUser{
		groups: hostRoleInfo.Groups,
		uid:    hostRoleInfo.UID,
		gid:    hostRoleInfo.GID,
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
