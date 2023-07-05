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

package srv

import (
	"context"
	"io"
	"os/user"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	rsession "github.com/gravitational/teleport/lib/session"
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

	logger := logrus.WithFields(logrus.Fields{
		trace.Component: teleport.ComponentAuth,
	})

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
				id:  "test",
				log: logger,
				registry: &SessionRegistry{
					SessionRegistryConfig: SessionRegistryConfig{
						Srv: &mockServer{
							component: teleport.ComponentNode,
						},
					},
				},
			},
			sctx: &ServerContext{
				SessionRecordingConfig: proxyRecording,
			},
			errAssertion: require.NoError,
			recAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.NotNil(t, i)
				_, ok := i.(*events.DiscardStream)
				require.True(t, ok)
			},
		},
		{
			desc: "discard-stream--when-proxy-sync-recording",
			sess: &session{
				id:  "test",
				log: logger,
				registry: &SessionRegistry{
					SessionRegistryConfig: SessionRegistryConfig{
						Srv: &mockServer{
							component: teleport.ComponentNode,
						},
					},
				},
			},
			sctx: &ServerContext{
				SessionRecordingConfig: proxyRecordingSync,
			},
			errAssertion: require.NoError,
			recAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.NotNil(t, i)
				_, ok := i.(*events.DiscardStream)
				require.True(t, ok)
			},
		},
		{
			desc: "strict-err-new-audit-writer-fails",
			sess: &session{
				id:  "test",
				log: logger,
				registry: &SessionRegistry{
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
				Identity: IdentityContext{
					AccessChecker: services.NewAccessCheckerWithRoleSet(&services.AccessInfo{
						Roles: []string{"dev"},
					}, "test", services.RoleSet{
						&types.RoleV5{
							Metadata: types.Metadata{Name: "dev", Namespace: apidefaults.Namespace},
							Spec: types.RoleSpecV5{
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
				id:  "test",
				log: logger,
				registry: &SessionRegistry{
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
				},
				Identity: IdentityContext{
					AccessChecker: services.NewAccessCheckerWithRoleSet(&services.AccessInfo{
						Roles: []string{"dev"},
					}, "test", services.RoleSet{
						&types.RoleV5{
							Metadata: types.Metadata{Name: "dev", Namespace: apidefaults.Namespace},
							Spec: types.RoleSpecV5{
								Options: types.RoleOptions{
									RecordSession: &types.RecordSession{
										SSH: constants.SessionRecordingModeBestEffort,
									},
								},
							},
						},
					}),
				},
			},
			errAssertion: require.NoError,
			recAssertion: func(t require.TestingT, i interface{}, _ ...interface{}) {
				require.NotNil(t, i)
				aw, ok := i.(*events.AuditWriter)
				require.True(t, ok)
				require.NoError(t, aw.Close(context.Background()))
			},
		},
		{
			desc: "audit-writer",
			sess: &session{
				id:  "test",
				log: logger,
				registry: &SessionRegistry{
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
					MockEmitter: &eventstest.MockEmitter{},
				},
			},
			errAssertion: require.NoError,
			recAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.NotNil(t, i)
				aw, ok := i.(*events.AuditWriter)
				require.True(t, ok)
				require.NoError(t, aw.Close(context.Background()))
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

	logger := logrus.WithFields(logrus.Fields{
		trace.Component: teleport.ComponentAuth,
	})

	t.Run("FallbackConcurrency", func(t *testing.T) {
		srv := newMockServer(t)
		reg, err := NewSessionRegistry(SessionRegistryConfig{
			Srv:                   srv,
			SessionTrackerService: srv.auth,
		})
		require.NoError(t, err)
		t.Cleanup(func() { reg.Close() })

		sess := &session{
			id:       "test",
			log:      logger,
			recorder: &mockRecorder{done: true},
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
		}, 1000*time.Millisecond, 100*time.Millisecond)
	})
}

// TestInteractiveSession tests interaction session lifecycles.
// Multiple sessions are opened in parallel tests to test for
// deadlocks between session registry, sessions, and parties.
func TestInteractiveSession(t *testing.T) {
	srv := newMockServer(t)
	srv.component = teleport.ComponentNode

	reg, err := NewSessionRegistry(SessionRegistryConfig{
		Srv:                   srv,
		SessionTrackerService: srv.auth,
	})
	require.NoError(t, err)
	t.Cleanup(func() { reg.Close() })

	t.Run("Stop", func(t *testing.T) {
		t.Parallel()
		sess, _ := testOpenSession(t, reg, nil)

		// Stopping the session should trigger the session
		// to end and cleanup in the background
		sess.Stop()

		sessionClosed := func() bool {
			_, found := reg.findSession(sess.id)
			return !found
		}
		require.Eventually(t, sessionClosed, time.Second*15, time.Millisecond*500)
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

	role, err := types.NewRole("access", types.RoleSpecV5{
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
	require.Equal(t, 1, len(sess.getParties()))
	testJoinSession(t, reg, sess)
	require.Equal(t, 2, len(sess.getParties()))
	testJoinSession(t, reg, sess)
	require.Equal(t, 3, len(sess.getParties()))

	// If a party leaves, the session should remove the party and continue.
	p := sess.getParties()[0]
	p.Close()

	partyIsRemoved := func() bool {
		return len(sess.getParties()) == 2 && !sess.isStopped()
	}
	require.Eventually(t, partyIsRemoved, time.Second*5, time.Millisecond*500)

	// If a party's session context is closed, the party should leave the session.
	p = sess.getParties()[0]
	err = p.ctx.Close()
	require.NoError(t, err)

	partyIsRemoved = func() bool {
		return len(sess.getParties()) == 1 && !sess.isStopped()
	}
	require.Eventually(t, partyIsRemoved, time.Second*5, time.Millisecond*500)

	p.closeOnce.Do(func() {
		t.Fatalf("party should be closed already")
	})

	// If all parties are gone, the session should linger for a short duration.
	sess.getParties()[0].Close()
	require.False(t, sess.isStopped())

	// Wait for session to linger (time.Sleep)
	regClock.BlockUntil(2)

	// If a party connects to the lingering session, it will continue.
	testJoinSession(t, reg, sess)
	require.Equal(t, 1, len(sess.getParties()))

	// andvance clock and give lingerAndDie goroutine a second to complete.
	regClock.Advance(defaults.SessionIdlePeriod)
	require.False(t, sess.isStopped())

	// If no parties remain it should be closed after the duration.
	sess.getParties()[0].Close()
	require.False(t, sess.isStopped())

	// Wait for session to linger (time.Sleep)
	regClock.BlockUntil(2)

	// Session should close.
	regClock.Advance(defaults.SessionIdlePeriod)
	require.Eventually(t, sess.isStopped, time.Second*5, time.Millisecond*500)
}

func testJoinSession(t *testing.T, reg *SessionRegistry, sess *session) {
	scx := newTestServerContext(t, reg.Srv, nil)
	scx.setSession(sess)

	// Open a new session
	sshChanOpen := newMockSSHChannel()
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
				&types.RoleV5{
					Metadata: types.Metadata{Name: "dev", Namespace: apidefaults.Namespace},
					Spec: types.RoleSpecV5{
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
	events.StreamWriter
	emitter eventstest.MockEmitter
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
		assertion       require.ErrorAssertionFunc
		createAssertion func(t *testing.T, count int)
	}{
		{
			name:          "node with proxy recording mode",
			component:     teleport.ComponentNode,
			recordingMode: types.RecordAtProxy,
			assertion:     require.NoError,
			createAssertion: func(t *testing.T, count int) {
				require.Equal(t, 0, count)
			},
		},
		{
			name:          "node with node recording mode",
			component:     teleport.ComponentNode,
			recordingMode: types.RecordAtNode,
			assertion:     require.NoError,
			createAssertion: func(t *testing.T, count int) {
				require.Equal(t, 1, count)
			},
		},
		{
			name:          "proxy with proxy recording mode",
			component:     teleport.ComponentProxy,
			recordingMode: types.RecordAtProxy,
			assertion:     require.NoError,
			createAssertion: func(t *testing.T, count int) {
				require.Equal(t, 1, count)
			},
		},
		{
			name:          "proxy with node recording mode",
			component:     teleport.ComponentProxy,
			recordingMode: types.RecordAtNode,
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
			assertion:     require.Error,
			createError:   trace.ConnectionProblem(context.DeadlineExceeded, ""),
			createAssertion: func(t *testing.T, count int) {
				require.Equal(t, 1, count)
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

			sess := &session{
				id:  rsession.NewID(),
				log: utils.NewLoggerForTests().WithField(trace.Component, "test-session"),
				registry: &SessionRegistry{
					SessionRegistryConfig: SessionRegistryConfig{
						Srv:                   srv,
						SessionTrackerService: trackingService,
						clock:                 clockwork.NewFakeClock(), // use a fake clock to prevent the update loop from running
					},
				},
				serverMeta: apievents.ServerMetadata{
					ServerHostname: "test",
					ServerID:       "123",
				},
				scx:       scx,
				serverCtx: ctx,
				login:     me.Name,
				access:    sessionEvaluator{moderated: tt.moderated},
			}

			err = sess.trackSession(ctx, me.Name, nil)
			tt.assertion(t, err)
			tt.createAssertion(t, trackingService.CreatedCount())
		})
	}
}
