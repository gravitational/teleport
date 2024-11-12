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
	"reflect"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/events"
	testingkubemock "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
	"github.com/gravitational/teleport/lib/modules"
)

func TestModeratedSessions(t *testing.T) {
	// enable enterprise features to have access to ModeratedSessions.
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise, TestFeatures: modules.Features{
		Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
			entitlements.K8s: {Enabled: true},
		},
	}})
	const (
		moderatorUsername       = "moderator_user"
		moderatorRoleName       = "mod_role"
		userRequiringModeration = "user_wmod"
		roleRequiringModeration = "role_wmod"
		stdinPayload            = "sessionPayload"
		discardPayload          = "discardPayload"
		exitKeyword             = "exit"
	)
	var (
		// numSessionEndEvents represents the number of session.end events received
		// from audit events.
		numSessionEndEvents = new(int64)
		// numberOfExpectedSessionEnds is the number of session ends expected from
		// all tests. This value is incremented each time that a test has
		// tt.want.sessionEndEvent = true.
		numberOfExpectedSessionEnds int64
	)
	// kubeMock is a Kubernetes API mock for the session tests.
	// Once a new session is created, this mock will write to
	// stdout and stdin (if available) the pod name, followed
	// by copying the contents of stdin into both streams.
	kubeMock, err := testingkubemock.NewKubeAPIMock()
	require.NoError(t, err)
	t.Cleanup(func() { kubeMock.Close() })

	// creates a Kubernetes service with a configured cluster pointing to mock api server
	testCtx := SetupTestContext(
		context.Background(),
		t,
		TestConfig{
			Clusters: []KubeClusterConfig{{Name: kubeCluster, APIEndpoint: kubeMock.URL}},
			// onEvent is called each time a new event is produced. We only care about
			// sessionEnd events.
			OnEvent: func(ae apievents.AuditEvent) {
				if ae.GetType() == events.SessionEndEvent {
					atomic.AddInt64(numSessionEndEvents, 1)
				}
			},
		},
	)
	// close tests
	t.Cleanup(func() { require.NoError(t, testCtx.Close()) })

	t.Cleanup(func() {
		// Set a cleanup function to make sure it runs after every test completes.
		// It will validate if the # of session ends is the same as expected
		// number of session ends.
		require.Eventually(t, func() bool {
			// checks if the # of session ends is the same as expected
			// number of session ends.
			return atomic.LoadInt64(numSessionEndEvents) == numberOfExpectedSessionEnds
		}, 20*time.Second, 500*time.Millisecond)
	})

	// create a user with access to kubernetes that does not require any moderator.
	// (kubernetes_user and kubernetes_groups specified)
	user, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		username,
		RoleSpec{
			Name:       roleName,
			KubeUsers:  roleKubeUsers,
			KubeGroups: roleKubeGroups,
		})

	// create a moderator user with access to kubernetes
	// (kubernetes_user and kubernetes_groups specified)
	moderator, modRole := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		moderatorUsername,
		RoleSpec{
			Name: moderatorRoleName,
			// sessionJoin:
			SessionJoin: []*types.SessionJoinPolicy{
				{
					Name:  "Auditor oversight",
					Roles: []string{"*"},
					Kinds: []string{"k8s"},
					Modes: []string{string(types.SessionModeratorMode)},
				},
			},
			SetupRoleFunc: func(r types.Role) {
				// set kubernetes labels to empty to test relaxed join rules
				r.SetKubernetesLabels(types.Allow, types.Labels{})
			},
		})

	// create a userRequiringModerator with access to kubernetes thar requires
	// one moderator to join the session.
	userRequiringModerator, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		userRequiringModeration,
		RoleSpec{
			Name:       roleRequiringModeration,
			KubeUsers:  roleKubeUsers,
			KubeGroups: roleKubeGroups,
			SessionRequire: []*types.SessionRequirePolicy{
				{
					Name:   "Auditor oversight",
					Filter: fmt.Sprintf("contains(user.spec.roles, %q)", modRole.GetName()),
					Kinds:  []string{"k8s"},
					Modes:  []string{string(types.SessionModeratorMode)},
					Count:  1,
				},
			},
		})

	type args struct {
		user                 types.User
		moderator            types.User
		closeSession         bool
		moderatorForcedClose bool
		reason               string
		invite               []string
	}
	type want struct {
		sessionEndEvent bool
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "create session for user without moderation",
			args: args{
				user:   user,
				reason: "reason 1",
				invite: []string{"user1", "user2"},
			},
			want: want{
				sessionEndEvent: true,
			},
		},
		{
			name: "create session with moderation",
			args: args{
				user:      userRequiringModerator,
				moderator: moderator,
				reason:    "reason 2",
				invite:    []string{"user1", "user2"},
			},
			want: want{
				sessionEndEvent: true,
			},
		},
		{
			name: "create session without needing moderation but leave it without proper closing",
			args: args{
				user:         user,
				closeSession: true,
				reason:       "reason 3",
				invite:       []string{"user1", "user2"},
			},
			want: want{
				sessionEndEvent: true,
			},
		},
		{
			name: "create session with moderation but close connection before moderator joined",
			args: args{
				user:         userRequiringModerator,
				closeSession: true,
				reason:       "reason 4",
				invite:       []string{"user1", "user2"},
			},
			want: want{
				// until moderator joins the session is not started. If the connection
				// is closed, Teleport does not create any start or end events.
				sessionEndEvent: false,
			},
		},
		{
			name: "create session with moderation and once the session is active, the moderator closes it",
			args: args{
				user:                 userRequiringModerator,
				moderator:            moderator,
				moderatorForcedClose: true,
				reason:               "reason 5",
				invite:               []string{"user1", "user2"},
			},
			want: want{
				sessionEndEvent: true,
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		if tt.want.sessionEndEvent {
			numberOfExpectedSessionEnds++
		}
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			group := &errgroup.Group{}

			// generate a kube client with user certs for auth
			_, config := testCtx.GenTestKubeClientTLSCert(
				t,
				tt.args.user.GetName(),
				kubeCluster,
			)

			// create a user session.
			var (
				stdinReader, stdinWritter   = io.Pipe()
				stdoutReader, stdoutWritter = io.Pipe()
			)
			streamOpts := remotecommand.StreamOptions{
				Stdin:  stdinReader,
				Stdout: stdoutWritter,
				// when Tty is enabled, Stderr must be nil.
				Stderr: nil,
				Tty:    true,
			}
			req, err := generateExecRequest(
				generateExecRequestConfig{
					addr:          testCtx.KubeProxyAddress(),
					podName:       podName,
					podNamespace:  podNamespace,
					containerName: podContainerName,
					cmd:           containerCommmandExecute, // placeholder for commands to execute in the dummy pod
					options:       streamOpts,
					reason:        tt.args.reason,
					invite:        tt.args.invite,
				},
			)
			require.NoError(t, err)

			exec, err := remotecommand.NewSPDYExecutor(config, http.MethodPost, req.URL())
			require.NoError(t, err)

			// sessionIDC is used to share the sessionID between the user exec request and
			// the moderator. Once the moderator receives it, he can join the session.
			sessionIDC := make(chan string, 1)
			// moderatorJoined is used to syncronize when the moderator joins the session.
			moderatorJoined := make(chan struct{})
			once := sync.Once{}
			if tt.args.moderator != nil {
				// generate moderator certs
				_, config := testCtx.GenTestKubeClientTLSCert(
					t,
					tt.args.moderator.GetName(),
					kubeCluster,
				)

				// Simulate a moderator joining the session.
				group.Go(func() error {
					// waits for user to send the sessionID of his exec request.
					sessionID := <-sessionIDC
					// validate that the sessionID is valid and the reason is the one we expect.
					if err := validateSessionTracker(testCtx, sessionID, tt.args.reason, tt.args.invite); err != nil {
						return trace.Wrap(err)
					}
					t.Logf("moderator is joining sessionID %q", sessionID)
					// join the session.
					stream, err := testCtx.NewJoiningSession(config, sessionID, types.SessionModeratorMode)
					if err != nil {
						return trace.Wrap(err)
					}
					t.Cleanup(func() {
						require.NoError(t, stream.Close())
					})
					// always send the force terminate even when the session is normally closed.
					defer func() {
						stream.ForceTerminate()
					}()

					// moderator waits for the user informed that he joined the session.
					<-moderatorJoined
					dataFound := false
					for {
						p := make([]byte, 1024)
						n, err := stream.Read(p)
						if errors.Is(err, io.EOF) {
							break
						} else if err != nil {
							return trace.Wrap(err)
						}
						stringData := string(p[:n])
						// discardPayload is sent before the moderator joined the session.
						// If the moderator sees it, it means that the payload was not correctly
						// discarded.
						if strings.Contains(stringData, discardPayload) {
							return trace.Wrap(errors.New("discardPayload was not properly discarded"))
						}

						// stdinPayload is sent by the user after the session started.
						if strings.Contains(stringData, stdinPayload) {
							dataFound = true
						}

						// podContainerName is returned by the kubemock server and it's used
						// to control that the session has effectively started.
						// return to force the defer to run.
						if strings.Contains(stringData, podContainerName) && tt.args.moderatorForcedClose {
							if err := stream.ForceTerminate(); err != nil {
								return trace.Wrap(err)
							}
							continue
						}
					}
					if !dataFound && !tt.args.moderatorForcedClose {
						return trace.Wrap(errors.New("stdinPayload was not received"))
					}
					return nil
				})
			}

			// Simulate a user creating a session.
			group.Go(func() (err error) {
				// sessionIDRegex is used to parse the sessionID from the payload sent
				// to the user when moderation is needed
				sessionIDRegex := regexp.MustCompile(`(?m)ID: (.*)\.\.\.`)
				// identifiedData is used to control if the stdin data was correctly
				// received from the session TermManager.
				identifiedData := false
				defer func() {
					// close pipes.
					stdinWritter.Close()
					stdoutReader.Close()
					// validate if the data payload was received.
					// When force closing (user or moderator) we do not expect identifiedData
					// to be true.
					if err == nil && !identifiedData && !tt.args.closeSession && !tt.args.moderatorForcedClose {
						err = errors.New("data payload was not identified")
					}
				}()
				for {
					data := make([]byte, 1024)
					n, err := stdoutReader.Read(data)
					// ignore EOF and ErrClosedPipe errors
					if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
						return nil
					} else if err != nil {
						return trace.Wrap(err)
					}

					if tt.args.closeSession {
						return nil
					}

					stringData := string(data[:n])
					if sessionIDRegex.MatchString(stringData) {
						matches := sessionIDRegex.FindAllStringSubmatch(stringData, -1)
						sessionID := matches[0][1]
						t.Logf("sessionID identified %q", sessionID)

						// if sessionID is identified it means that the user waits for
						// a moderator to join. In the meanwhile we write to stdin to make sure it's
						// discarded.
						_, err := stdinWritter.Write([]byte(discardPayload))
						if err != nil {
							return trace.Wrap(err)
						}

						// send sessionID to moderator.
						sessionIDC <- sessionID
					}

					// checks if moderator has joined the session.
					// Each time a user joins a session the following message is broadcasted
					// User <user> joined the session with participant mode: <mode>.
					if strings.Contains(stringData, fmt.Sprintf("User %s joined the session with participant mode: moderator.", moderatorUsername)) {
						t.Logf("identified that moderator joined the session")
						// inform moderator goroutine that the user detected that he joined the
						// session.
						once.Do(func() {
							close(moderatorJoined)
						})
					}

					// if podContainerName is received, it means the session has already reached
					// the mock server. If we expect the moderated to force close the session
					// we don't send the stdinPayload data and the session will remain active.
					if strings.Contains(stringData, podContainerName) && !tt.args.moderatorForcedClose {
						// successfully connected to
						_, err := stdinWritter.Write([]byte(stdinPayload))
						if err != nil {
							return trace.Wrap(err)
						}
					}

					// check if we received the data we wrote into the stdin pipe.
					if strings.Contains(stringData, stdinPayload) {
						t.Logf("received the payload written to stdin")
						identifiedData = true
						break
					}
				}
				return nil
			},
			)

			group.Go(func() error {
				defer func() {
					// once stream is finished close the pipes.
					stdinReader.Close()
					stdoutWritter.Close()
				}()
				// start user session.
				err := exec.StreamWithContext(testCtx.Context, streamOpts)
				// ignore closed pipes error.
				if errors.Is(err, io.ErrClosedPipe) {
					return nil
				}
				if tt.args.moderatorForcedClose && isSessionTerminatedError(err) {
					return nil
				}
				return trace.Wrap(err)
			})
			// wait for every go-routine to finish without errors returned.
			require.NoError(t, group.Wait())
		})
	}
}

// validateSessionTracker validates that the session tracker has the expected
// reason and invited users.
func validateSessionTracker(testCtx *TestContext, sessionID string, reason string, invited []string) error {
	sessionTracker, err := testCtx.AuthClient.GetSessionTracker(testCtx.Context, sessionID)
	if err != nil {
		return trace.Wrap(err)
	}
	if sessionTracker.GetReason() != reason {
		return trace.BadParameter("expected reason %q, got %q", reason, sessionTracker.GetReason())
	}
	if !reflect.DeepEqual(sessionTracker.GetInvited(), invited) {
		return trace.BadParameter("expected invited %q, got %q", invited, sessionTracker.GetInvited())
	}
	return nil
}

// TestInteractiveSessionsNoAuth tests the interactive sessions when auth server
// is down or the connection is not available.
// This test complements github.com/gravitational/teleport/integration.TestKube/ExecWithNoAuth
// which tests sessions that require and do not require moderation. This test exists
// to test the scenario where the user has a role that defines the LockingMode to
// "strict" and the user has no access to the auth server. This scenario is not
// covered by the integration test mentioned above because we need to fake the
// Lock watcher connection to be stale and it takes ~5 minutes to happen.
func TestInteractiveSessionsNoAuth(t *testing.T) {
	// enable enterprise features to have access to ModeratedSessions.
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise, TestFeatures: modules.Features{
		Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
			entitlements.K8s: {Enabled: true},
		},
	}})
	const (
		moderatorUsername       = "moderator_user"
		moderatorRoleName       = "mod_role"
		userRequiringModeration = "user_wmod"
		roleRequiringModeration = "role_wmod"
		userStrictLocking       = "user_strict"
		roleStrictLocking       = "role_strict"
		stdinPayload            = "sessionPayload"
		discardPayload          = "discardPayload"
		exitKeyword             = "exit"
	)

	// kubeMock is a Kubernetes API mock for the session tests.
	// Once a new session is created, this mock will write to
	// stdout and stdin (if available) the pod name, followed
	// by copying the contents of stdin into both streams.
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
	// close tests
	t.Cleanup(func() { require.NoError(t, testCtx.Close()) })

	// create a user with access to kubernetes that does not require any moderator.
	// but his role defines strict locking.
	userWithStrictLock, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		userStrictLocking,
		RoleSpec{
			Name:       roleStrictLocking,
			KubeUsers:  roleKubeUsers,
			KubeGroups: roleKubeGroups,
			SetupRoleFunc: func(role types.Role) {
				// set session lock to strict
				opts := role.GetOptions()
				opts.Lock = constants.LockingModeStrict
				role.SetOptions(opts)
			},
		})

	// create a adminUser user with access to kubernetes
	// (kubernetes_user and kubernetes_groups specified)
	adminUser, modRole := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		moderatorUsername,
		RoleSpec{
			Name:       moderatorRoleName,
			KubeUsers:  roleKubeUsers,
			KubeGroups: roleKubeGroups,
			// sessionJoin:
			SessionJoin: []*types.SessionJoinPolicy{
				{
					Name:  "Auditor oversight",
					Roles: []string{"*"},
					Kinds: []string{"k8s"},
					Modes: []string{string(types.SessionModeratorMode)},
				},
			},
		})

	// create a userRequiringModerator with access to kubernetes thar requires
	// one moderator to join the session.
	userRequiringModerator, _ := testCtx.CreateUserAndRole(
		testCtx.Context,
		t,
		userRequiringModeration,
		RoleSpec{
			Name:       roleRequiringModeration,
			KubeUsers:  roleKubeUsers,
			KubeGroups: roleKubeGroups,
			SessionRequire: []*types.SessionRequirePolicy{
				{
					Name:   "Auditor oversight",
					Filter: fmt.Sprintf("contains(user.spec.roles, %q)", modRole.GetName()),
					Kinds:  []string{"k8s"},
					Modes:  []string{string(types.SessionModeratorMode)},
					Count:  1,
				},
			},
		})

	// generate a kube client with user certs for auth
	_, userStrictLockingConfig := testCtx.GenTestKubeClientTLSCert(
		t,
		userWithStrictLock.GetName(),
		kubeCluster,
	)

	_, moderatorConfig := testCtx.GenTestKubeClientTLSCert(
		t,
		adminUser.GetName(),
		kubeCluster,
	)

	_, userRequiringModeratorConfig := testCtx.GenTestKubeClientTLSCert(
		t,
		userRequiringModerator.GetName(),
		kubeCluster,
	)

	// Mark the lock as stale so that the session with strict locking is denied.
	close(testCtx.lockWatcher.StaleC)
	// force the auth client to return an error when trying to create a session.
	close(testCtx.closeSessionTrackers)

	require.Eventually(t, func() bool {
		return testCtx.lockWatcher.IsStale()
	}, 3*time.Second, time.Millisecond*100, "lock watcher should be stale")
	tests := []struct {
		name      string
		config    *rest.Config
		assertErr require.ErrorAssertionFunc
	}{
		{
			// this session does not require moderation and should be created without any issues.
			name:      "create session for user without moderation",
			assertErr: require.NoError,
			config:    moderatorConfig,
		},
		{
			// this session is denied because moderator sessions are not allowed when no auth connection is available.
			name:      "create session requiring moderation",
			assertErr: require.Error,
			config:    userRequiringModeratorConfig,
		},
		{
			// this session is denied because strict locking sessions are not allowed when no auth connection is available and the connector is expired.
			name:      "create session with strict locking",
			assertErr: require.Error,
			config:    userStrictLockingConfig,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// create a user session.
			var (
				stdinReader   = bytes.NewReader([]byte("ls"))
				stdoutWritter = &bytes.Buffer{}
			)
			streamOpts := remotecommand.StreamOptions{
				Stdin:  stdinReader,
				Stdout: stdoutWritter,
				// when Tty is enabled, Stderr must be nil.
				Stderr: nil,
				Tty:    true,
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

			exec, err := remotecommand.NewSPDYExecutor(tt.config, http.MethodPost, req.URL())
			require.NoError(t, err)
			err = exec.StreamWithContext(context.TODO(), streamOpts)
			tt.assertErr(t, err)
		})
	}
}
