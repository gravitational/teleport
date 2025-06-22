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

package regular

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/moby/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	stableunixusersv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/stableunixusers/v1"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/observability/tracing"
	libproxy "github.com/gravitational/teleport/lib/proxy"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
	sess "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/sshutils/x11"
	"github.com/gravitational/teleport/lib/utils"
)

// teleportTestUser is additional user used for tests
const teleportTestUser = "teleport-test"

// wildcardAllow is used in tests to allow access to all labels.
var wildcardAllow = types.Labels{
	types.Wildcard: []string{types.Wildcard},
}

// TestMain will re-execute Teleport to run a command if "exec" is passed to
// it as an argument. Otherwise it will run tests as normal.
func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	modules.SetInsecureTestMode(true)
	if srv.IsReexec() {
		srv.RunAndExit(os.Args[1])
		return
	}

	code := m.Run()
	os.Exit(code)
}

type sshInfo struct {
	srv            *Server
	srvAddress     string
	srvPort        string
	srvHostPort    string
	clt            *tracessh.Client
	cltConfig      *ssh.ClientConfig
	assertCltClose require.ErrorAssertionFunc
}

type sshTestFixture struct {
	ssh     sshInfo
	up      *upack
	signer  ssh.Signer
	user    string
	clock   *clockwork.FakeClock
	testSrv *auth.TestServer
}

func newFixture(t *testing.T) *sshTestFixture {
	return newCustomFixture(t, func(*auth.TestServerConfig) {})
}

func newFixtureWithoutDiskBasedLogging(t testing.TB, sshOpts ...ServerOption) *sshTestFixture {
	t.Helper()

	f := newCustomFixture(t, func(cfg *auth.TestServerConfig) {
		cfg.Auth.AuditLog = events.NewDiscardAuditLog()
	}, sshOpts...)

	// use a sync recording mode because the disk-based uploader
	// that runs in the background introduces races with test cleanup
	recConfig := types.DefaultSessionRecordingConfig()
	recConfig.SetMode(types.RecordAtNodeSync)
	_, err := f.testSrv.Auth().UpsertSessionRecordingConfig(context.Background(), recConfig)
	require.NoError(t, err)

	return f
}

func (f *sshTestFixture) newSSHClient(ctx context.Context, t testing.TB, user *user.User) *tracessh.Client {
	// set up SSH client using the user private key for signing
	up, err := newUpack(f.testSrv, user.Username, []string{user.Username}, wildcardAllow)
	require.NoError(t, err)

	// set up an agent server and a client that uses agent for forwarding
	keyring := agent.NewKeyring()
	addedKey := agent.AddedKey{
		PrivateKey:  up.pkey,
		Certificate: up.pcert,
	}
	require.NoError(t, keyring.Add(addedKey))

	cltConfig := &ssh.ClientConfig{
		User:            user.Username,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(f.signer.PublicKey()),
	}

	client, err := tracessh.Dial(ctx, "tcp", f.ssh.srvAddress, cltConfig)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})

	return client
}

func newCustomFixture(t testing.TB, mutateCfg func(*auth.TestServerConfig), sshOpts ...ServerOption) *sshTestFixture {
	ctx := context.Background()

	u, err := user.Current()
	require.NoError(t, err)

	clock := clockwork.NewFakeClock()

	serverCfg := auth.TestServerConfig{
		Auth: auth.TestAuthServerConfig{
			ClusterName: "localhost",
			Dir:         t.TempDir(),
			Clock:       clock,
		},
	}
	mutateCfg(&serverCfg)

	testServer, err := auth.NewTestServer(serverCfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testServer.Shutdown(ctx)) })

	signer := newSigner(t, ctx, testServer)
	nodeID := uuid.New().String()
	nodeClient, err := testServer.NewClient(auth.TestIdentity{
		I: authz.BuiltinRole{
			Role:     types.RoleNode,
			Username: nodeID,
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, nodeClient.Close()) })

	lockWatcher := newLockWatcher(ctx, t, nodeClient)

	sessionController, err := srv.NewSessionController(srv.SessionControllerConfig{
		Semaphores:   nodeClient,
		AccessPoint:  nodeClient,
		LockEnforcer: lockWatcher,
		Emitter:      nodeClient,
		Component:    teleport.ComponentNode,
		ServerID:     nodeID,
	})
	require.NoError(t, err)

	nodeDir := t.TempDir()
	serverOptions := []ServerOption{
		SetUUID(nodeID),
		SetNamespace(apidefaults.Namespace),
		SetEmitter(nodeClient),
		SetPAMConfig(&servicecfg.PAMConfig{Enabled: false}),
		SetLabels(
			map[string]string{"foo": "bar"},
			services.CommandLabels{
				"baz": &types.CommandLabelV2{
					Period:  types.NewDuration(time.Second),
					Command: []string{"expr", "1", "+", "3"},
				},
			}, nil,
		),
		SetBPF(&bpf.NOP{}),
		SetClock(clock),
		SetLockWatcher(lockWatcher),
		SetX11ForwardingConfig(&x11.ServerConfig{}),
		SetSessionController(sessionController),
		SetStoragePresenceService(testServer.AuthServer.AuthServer.PresenceInternal),
		SetConnectedProxyGetter(reversetunnel.NewConnectedProxyGetter()),
	}

	serverOptions = append(serverOptions, sshOpts...)

	sshSrv, err := New(
		ctx,
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		testServer.ClusterName(),
		sshutils.StaticHostSigners(signer),
		nodeClient,
		nodeDir,
		"",
		utils.NetAddr{},
		nodeClient,
		serverOptions...)
	require.NoError(t, err)
	require.NoError(t, sshSrv.Start())
	t.Cleanup(func() {
		require.NoError(t, sshSrv.Close())
		sshSrv.Wait()
	})

	server, err := sshSrv.getServerInfo(ctx)
	require.NoError(t, err)
	_, err = testServer.Auth().UpsertNode(ctx, server)
	require.NoError(t, err)

	sshSrvAddress := sshSrv.Addr()
	_, sshSrvPort, err := net.SplitHostPort(sshSrvAddress)
	require.NoError(t, err)
	sshSrvHostPort := fmt.Sprintf("%v:%v", testServer.ClusterName(), sshSrvPort)

	// set up SSH client using the user private key for signing
	up, err := newUpack(testServer, u.Username, []string{u.Username}, wildcardAllow)
	require.NoError(t, err)

	// set up an agent server and a client that uses agent for forwarding
	keyring := agent.NewKeyring()
	addedKey := agent.AddedKey{
		PrivateKey:  up.pkey,
		Certificate: up.pcert,
	}
	require.NoError(t, keyring.Add(addedKey))

	cltConfig := &ssh.ClientConfig{
		User:            u.Username,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(signer.PublicKey()),
	}

	client, err := tracessh.Dial(ctx, "tcp", sshSrv.Addr(), cltConfig)
	require.NoError(t, err)

	f := &sshTestFixture{
		ssh: sshInfo{
			srv:            sshSrv,
			srvAddress:     sshSrvAddress,
			srvPort:        sshSrvPort,
			srvHostPort:    sshSrvHostPort,
			clt:            client,
			cltConfig:      cltConfig,
			assertCltClose: require.NoError,
		},
		up:      up,
		signer:  signer,
		user:    u.Username,
		clock:   clock,
		testSrv: testServer,
	}

	t.Cleanup(func() { f.ssh.assertCltClose(t, client.Close()) })
	require.NoError(t, agent.ForwardToAgent(client.Client, keyring))
	return f
}

// TestTerminalSizeRequest validates that terminal size requests are processed and
// responded to appropriately. Namely, it ensures that a response is sent if the client
// requests a reply whether processing the request was successful or not.
func TestTerminalSizeRequest(t *testing.T) {
	f := newFixtureWithoutDiskBasedLogging(t)
	ctx := t.Context()

	t.Run("Invalid session", func(t *testing.T) {
		ok, resp, err := f.ssh.clt.SendRequest(ctx, teleport.TerminalSizeRequest, true, []byte("1234"))
		require.NoError(t, err)
		require.False(t, ok)
		require.Empty(t, resp)
	})

	t.Run("Active session", func(t *testing.T) {
		se, err := f.ssh.clt.NewSession(ctx)
		require.NoError(t, err)
		defer se.Close()

		require.NoError(t, se.Shell(ctx))

		sessions, err := f.ssh.srv.termHandlers.SessionRegistry.SessionTrackerService.GetActiveSessionTrackers(ctx)
		require.NoError(t, err)
		require.Len(t, sessions, 1)

		sessionID := sessions[0].GetSessionID()

		expectedSize := term.Winsize{Height: 100, Width: 200}

		// Explicitly set the window size to the expected value.
		require.NoError(t, se.WindowChange(ctx, int(expectedSize.Height), int(expectedSize.Width)))

		// Wait for the window change request to be reflected in the session before
		// initiating the client request for the window size to prevent flakiness.
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			size, err := f.ssh.srv.termHandlers.SessionRegistry.GetTerminalSize(sessionID)
			if assert.NoError(t, err) {
				return
			}
			assert.Empty(t, cmp.Diff(expectedSize, size, cmp.AllowUnexported(term.Winsize{})))
		}, 10*time.Second, 100*time.Millisecond)

		// Send a request for the window size now that we know the window change
		// request was honored.
		ok, resp, err := f.ssh.clt.SendRequest(ctx, teleport.TerminalSizeRequest, true, []byte(sessionID))
		require.NoError(t, err)
		require.True(t, ok)
		require.NotNil(t, resp)

		// Assert that the window size matches the expected dimensions.
		var ws term.Winsize
		require.NoError(t, json.Unmarshal(resp, &ws))
		require.Empty(t, cmp.Diff(expectedSize, ws, cmp.AllowUnexported(term.Winsize{})))
	})
}

// TestMultipleExecCommands asserts that multiple SSH exec commands can not be
// sent over a single SSH channel, which is disallowed by the SSH standard
// https://www.ietf.org/rfc/rfc4254.txt
//
// Conformant clients (tsh, openssh) will never try to do this, but we must
// correctly handle the case where an attacker would try to send multiple
// commands over the same channel to try to cover their tracks in the audit log
// or do other nefarious things.
//
// We make sure that:
//   - the first command is correctly added to the audit log
//   - the second command does not appear in the audit log (as a proxy for testing
//     that it was blocked/not executed)
//   - there are no panics or unexpected errors
//   - and we give the race detector a chance to detect any possible race
//     conditions on this code path.
func TestMultipleExecCommands(t *testing.T) {
	f := newFixtureWithoutDiskBasedLogging(t)
	ctx := t.Context()

	// Set up a mock emitter so we can capture audit events.
	emitter := eventstest.NewChannelEmitter(32)
	f.ssh.srv.StreamEmitter = events.StreamerAndEmitter{
		Streamer: events.NewDiscardStreamer(),
		Emitter:  emitter,
	}

	// Manually open an ssh channel
	channel, _, err := f.ssh.clt.OpenChannel(ctx, "session", nil)
	require.NoError(t, err)

	sendExec := func(cmd string) error {
		type execRequest struct {
			Command string
		}
		// Intentionally don't request a reply so that SendRequest won't wait,
		// we want to maximize the chance of triggering any potential race
		// condition.
		_, err := channel.SendRequest(ctx, "exec", false /*wantReply*/, ssh.Marshal(&execRequest{Command: cmd}))
		return err
	}

	t.Log("sending first exec request over channel")
	err = sendExec("echo 1")
	require.NoError(t, err)

	t.Log("sending second exec request over channel")
	// Since we don't wait for a reply, this may or may not return an error, and
	// we don't really care either way
	_ = sendExec("echo 2")

	// Wait for session.end event
	timeout := time.After(10 * time.Second)
	var execEvent *apievents.Exec
loop:
	for {
		select {
		case event := <-emitter.C():
			switch e := event.(type) {
			case *apievents.Exec:
				require.Nil(t, execEvent, "found more than 1 exec event in audit log")
				execEvent = e
			case *apievents.SessionEnd:
				break loop
			}
		case <-timeout:
			require.Fail(t, "hit timeout while waiting for session.end event")
		}
	}

	// The first command, which was actually executed, should be recorded
	// correctly in the audit log
	require.NotNil(t, execEvent)
	require.Equal(t, "echo 1", execEvent.CommandMetadata.Command)
}

// TestSessionAuditLog tests that the expected audit events are emitted for a
// session with various subsystems involved.
//
// Note: This is a regression test for a bug which resulted in extra, empty session.data
// events for networking requests.
// See https://github.com/gravitational/teleport/issues/48728.
func TestSessionAuditLog(t *testing.T) {
	ctx := context.Background()
	t.Parallel()
	f := newFixtureWithoutDiskBasedLogging(t)

	// Set up a mock emitter so we can capture audit events.
	emitter := eventstest.NewChannelEmitter(32)
	f.ssh.srv.StreamEmitter = events.StreamerAndEmitter{
		Streamer: events.NewDiscardStreamer(),
		Emitter:  emitter,
	}

	// Enable x11 forwarding
	f.ssh.srv.x11 = &x11.ServerConfig{
		Enabled:       true,
		DisplayOffset: x11.DefaultDisplayOffset,
		MaxDisplay:    x11.DefaultMaxDisplays,
	}

	// Allow x11, agent, and port forwarding for the user.
	roleName := services.RoleNameForUser(f.user)
	role, err := f.testSrv.Auth().GetRole(ctx, roleName)
	require.NoError(t, err)
	roleOptions := role.GetOptions()
	roleOptions.PermitX11Forwarding = types.NewBool(true)
	roleOptions.ForwardAgent = types.NewBool(true)
	role.SetOptions(roleOptions)
	_, err = f.testSrv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	nextEvent := func() apievents.AuditEvent {
		select {
		case event := <-emitter.C():
			return event
		case <-time.After(time.Second):
			require.Fail(t, "timed out waiting for event")
		}
		return nil
	}

	// Start a new session
	se, err := f.ssh.clt.NewSession(ctx)
	require.NoError(t, err)

	// start interactive SSH session (new shell):
	err = se.Shell(ctx)
	require.NoError(t, err)

	e := nextEvent()
	startEvent, ok := e.(*apievents.SessionStart)
	require.True(t, ok, "expected SessionStart event but got event of type %T", e)
	require.NotEmpty(t, startEvent.SessionID, "expected non empty sessionID")
	sessionID := startEvent.SessionID

	// Request agent forwarding, no individual event emitted.
	err = agent.RequestAgentForwarding(se.Session)
	require.NoError(t, err)

	// Request x11 forwarding, event should be emitted immediately.
	clientXAuthEntry, err := x11.NewFakeXAuthEntry(x11.Display{})
	require.NoError(t, err)
	err = x11.RequestForwarding(se.Session, clientXAuthEntry)
	require.NoError(t, err)

	x11Event := nextEvent()
	require.IsType(t, &apievents.X11Forward{}, x11Event, "expected X11Forward event but got event of tgsype %T", x11Event)

	// LOCAL PORT FORWARDING
	// Start up a test server that doesn't do any remote port forwarding
	nonForwardServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello, world")
	}))
	t.Cleanup(nonForwardServer.Close)
	nonForwardServer.Start()

	// Each locally forwarded dial should result in a new "start" event and each closed connection should result in a "stop"
	// event. Note that we don't know what port the server will forward the connection on, so we don't have an easy way to validate the
	// event's addr field.
	localConn, err := f.ssh.clt.DialContext(context.Background(), "tcp", nonForwardServer.Listener.Addr().String())
	require.NoError(t, err)

	e = nextEvent()
	localForwardStart, ok := e.(*apievents.PortForward)
	require.True(t, ok, "expected PortForward event but got event of type %T", e)
	require.Equal(t, events.PortForwardLocalEvent, localForwardStart.GetType())
	require.Equal(t, events.PortForwardCode, localForwardStart.GetCode())
	require.Equal(t, nonForwardServer.Listener.Addr().String(), localForwardStart.Addr)

	// closed connections should result in PortForwardLocal stop events
	localConn.Close()
	e = nextEvent()
	localForwardStop, ok := e.(*apievents.PortForward)
	require.True(t, ok, "expected PortForward event but got event of type %T", e)
	require.Equal(t, events.PortForwardLocalEvent, localForwardStop.GetType())
	require.Equal(t, events.PortForwardStopCode, localForwardStop.GetCode())
	require.Equal(t, nonForwardServer.Listener.Addr().String(), localForwardStop.Addr)

	// REMOTE PORT FORWARDING
	// Creation of a port forwarded listener should generate PortForwardRemote start events
	listener, err := f.ssh.clt.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	e = nextEvent()
	remoteForwardStart, ok := e.(*apievents.PortForward)
	require.True(t, ok, "expected PortForward event but got event of type %T", e)
	require.Equal(t, listener.Addr().String(), remoteForwardStart.Addr)
	require.Equal(t, events.PortForwardRemoteEvent, remoteForwardStart.GetType())
	require.Equal(t, events.PortForwardCode, remoteForwardStart.GetCode())

	// Start up a test server that uses the remote port forwarded listener.
	remoteForwardServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello, world")
	}))
	t.Cleanup(remoteForwardServer.Close)
	remoteForwardServer.Listener = listener
	remoteForwardServer.Start()

	// Each dial to the remote listener should result in a new "start" event and each closed connection should result in a "stop" event.
	// Note that we don't know what port the server will forward the connection on, so we don't have an easy way to validate the event's
	// addr field.
	remoteConn, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	e = nextEvent()
	remoteConnStart, ok := e.(*apievents.PortForward)
	require.True(t, ok, "expected PortForward event but got event of type %T", e)
	require.Equal(t, events.PortForwardRemoteConnEvent, remoteConnStart.GetType())
	require.Equal(t, events.PortForwardCode, remoteConnStart.GetCode())

	remoteConn.Close()
	e = nextEvent()
	remoteConnStop, ok := e.(*apievents.PortForward)
	require.True(t, ok, "expected PortForward event but got event of type %T", e)
	require.Equal(t, events.PortForwardRemoteConnEvent, remoteConnStop.GetType())
	require.Equal(t, events.PortForwardStopCode, remoteConnStop.GetCode())

	// Closing the server (and therefore the listener) should generate an PortForwardRemote stop event
	remoteForwardServer.Close()
	e = nextEvent()
	remoteForwardStop, ok := e.(*apievents.PortForward)
	require.True(t, ok, "expected PortForward event but got event of type %T", e)
	require.Equal(t, events.PortForwardRemoteEvent, remoteForwardStop.GetType())
	require.Equal(t, events.PortForwardStopCode, remoteForwardStop.Code)
	require.Equal(t, listener.Addr().String(), remoteForwardStop.Addr)

	// End the session. Session leave, data, and end events should be emitted.
	se.Close()

	e = nextEvent()
	leaveEvent, ok := e.(*apievents.SessionLeave)
	require.True(t, ok, "expected SessionLeave event but got event of type %T", e)
	require.Equal(t, sessionID, leaveEvent.SessionID)

	e = nextEvent()
	dataEvent, ok := e.(*apievents.SessionData)
	require.True(t, ok, "expected SessionData event but got event of type %T", e)
	require.Equal(t, sessionID, dataEvent.SessionID)

	e = nextEvent()
	endEvent, ok := e.(*apievents.SessionEnd)
	require.True(t, ok, "expected SessionEnd event but got event of type %T", e)
	require.Equal(t, sessionID, endEvent.SessionID)
}

func newProxyClient(t *testing.T, testSvr *auth.TestServer) (*authclient.Client, string) {
	// create proxy client used in some tests
	proxyID := uuid.New().String()
	proxyClient, err := testSvr.NewClient(auth.TestIdentity{
		I: authz.BuiltinRole{
			Role:     types.RoleProxy,
			Username: proxyID,
		},
	})
	require.NoError(t, err)
	return proxyClient, proxyID
}

func newNodeClient(t *testing.T, testSvr *auth.TestServer) (*authclient.Client, string) {
	nodeID := uuid.New().String()
	nodeClient, err := testSvr.NewClient(auth.TestIdentity{
		I: authz.BuiltinRole{
			Role:     types.RoleNode,
			Username: nodeID,
		},
	})
	require.NoError(t, err)
	return nodeClient, nodeID
}

const hostID = "00000000-0000-0000-0000-000000000000"

func startReadAll(r io.Reader) <-chan []byte {
	ch := make(chan []byte)
	go func() {
		data, _ := io.ReadAll(r)
		ch <- data
	}()
	return ch
}

func waitForBytes(ch <-chan []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	select {
	case data := <-ch:
		return data, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestInactivityTimeout(t *testing.T) {
	const timeoutMessage = "You snooze, you lose."

	// Given
	//  * a running auth server configured with a 5s inactivity timeout,
	//  * a running SSH server configured with a given disconnection message
	//  * a client connected to the SSH server,
	//  * an SSH session running over the client connection
	mutateCfg := func(cfg *auth.TestServerConfig) {
		networkCfg := types.DefaultClusterNetworkingConfig()
		networkCfg.SetClientIdleTimeout(5 * time.Second)
		networkCfg.SetClientIdleTimeoutMessage(timeoutMessage)

		cfg.Auth.ClusterNetworkingConfig = networkCfg
	}

	waitForTimeout := func(t *testing.T, f *sshTestFixture, se *tracessh.Session) {
		stderr, err := se.StderrPipe()
		require.NoError(t, err)
		stdErrCh := startReadAll(stderr)

		endCh := make(chan error)
		go func() { endCh <- f.ssh.clt.Wait() }()
		t.Cleanup(func() { f.ssh.clt.Close() })

		// When I let the session idle (with the clock running at approx 10x speed)...
		sessionHasFinished := func() bool {
			f.clock.Advance(1 * time.Second)
			select {
			case <-endCh:
				return true

			default:
				return false
			}
		}
		require.Eventually(t, sessionHasFinished, 6*time.Second, 100*time.Millisecond,
			"Timed out waiting for session to finish")

		// Expect that the idle timeout has been delivered via stderr
		text, err := waitForBytes(stdErrCh)
		require.NoError(t, err)
		require.Equal(t, timeoutMessage, string(text))
	}

	t.Run("Normal timeout", func(t *testing.T) {
		f := newCustomFixture(t, mutateCfg)

		// If all goes well, the client will be closed by the time cleanup happens,
		// so change the assertion on closing the client to expect it to fail
		f.ssh.assertCltClose = require.Error
		se, err := f.ssh.clt.NewSession(context.Background())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, err) })
		waitForTimeout(t, f, se)
	})

	t.Run("Reset timeout on input", func(t *testing.T) {
		f := newCustomFixture(t, mutateCfg)

		// If all goes well, the client will be closed by the time cleanup happens,
		// so change the assertion on closing the client to expect it to fail
		f.ssh.assertCltClose = require.Error
		se, err := f.ssh.clt.NewSession(context.Background())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, err) })

		stdin, err := se.StdinPipe()
		require.NoError(t, err)
		t.Cleanup(func() { require.ErrorIs(t, stdin.Close(), io.EOF) })

		endCh := make(chan error)
		go func() { endCh <- f.ssh.clt.Wait() }()
		t.Cleanup(func() { f.ssh.clt.Close() })

		f.clock.Advance(3 * time.Second)
		// Input should reset idle timeout.
		_, err = stdin.Write([]byte("echo hello\n"))
		require.NoError(t, err)
		f.clock.Advance(3 * time.Second)

		select {
		case <-endCh:
			require.Fail(t, "Session timed out too early")
		default:
		}

		waitForTimeout(t, f, se)
	})
}

func TestLockInForce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newFixtureWithoutDiskBasedLogging(t)

	// If all goes well, the client will be closed by the time cleanup happens,
	// so change the assertion on closing the client to expect it to fail.
	f.ssh.assertCltClose = require.Error

	se, err := f.ssh.clt.NewSession(ctx)
	require.NoError(t, err)

	stderr, err := se.StderrPipe()
	require.NoError(t, err)
	stdErrCh := startReadAll(stderr)

	endCh := make(chan error)
	go func() { endCh <- f.ssh.clt.Wait() }()
	t.Cleanup(func() { f.ssh.clt.Close() })

	lock, err := types.NewLock("test-lock", types.LockSpecV2{
		Target: types.LockTarget{Login: f.user},
	})
	require.NoError(t, err)

	watcher := f.ssh.srv.GetLockWatcher()
	sub, err := watcher.Subscribe(ctx, lock.Target())
	require.NoError(t, err)

	require.NoError(t, f.testSrv.Auth().UpsertLock(ctx, lock))

	// Wait for the lock to appear before proceeding.
	timeout := time.After(20 * time.Second)
	for wait := true; wait; {
		select {
		case evt := <-sub.Events():
			if evt.Type != types.OpPut {
				continue
			}

			eventLock, ok := evt.Resource.(types.Lock)
			require.True(t, ok)
			require.Empty(t, cmp.Diff(lock.Target(), eventLock.Target()))
			wait = false
		case <-sub.Done():
			t.Fatalf("lock subscription terminated unexpectedly %v", sub.Error())
		case <-timeout:
			t.Fatal("timed out waiting for lock target event")
		}
	}
	require.NoError(t, sub.Close())

	// Expect the session to eventually be terminated because of the lock.
	select {
	case <-endCh:
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for session to finish")
	}

	// Expect the lock-in-force message to have been delivered via stderr.
	lockInForceMsg := services.LockInForceAccessDenied(lock).Error()
	text, err := waitForBytes(stdErrCh)
	require.NoError(t, err)
	require.Equal(t, lockInForceMsg, string(text))

	// As long as the lock is in force, new sessions cannot be opened.
	newClient, err := tracessh.Dial(ctx, "tcp", f.ssh.srvAddress, f.ssh.cltConfig)
	require.NoError(t, err)
	t.Cleanup(func() {
		// The client is expected to be closed by the lock monitor therefore expect
		// an error on this second attempt.
		require.Error(t, newClient.Close())
	})
	_, err = newClient.NewSession(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), lockInForceMsg)

	// Once the lock is lifted, new sessions should go through without error.
	require.NoError(t, f.testSrv.Auth().DeleteLock(ctx, "test-lock"))
	newClient2, err := tracessh.Dial(ctx, "tcp", f.ssh.srvAddress, f.ssh.cltConfig)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, newClient2.Close()) })
	_, err = newClient2.NewSession(ctx)
	require.NoError(t, err)
}

func setPortForwarding(t *testing.T, ctx context.Context, f *sshTestFixture, legacy, remote, local *types.BoolOption) {
	roleName := services.RoleNameForUser(f.user)
	role, err := f.testSrv.Auth().GetRole(ctx, roleName)
	require.NoError(t, err)
	roleOptions := role.GetOptions()
	roleOptions.PermitX11Forwarding = types.NewBool(true)
	roleOptions.ForwardAgent = types.NewBool(true)
	//nolint:staticcheck // this field is preserved for existing deployments, but shouldn't be used going forward
	roleOptions.PortForwarding = legacy

	if remote != nil || local != nil {
		roleOptions.SSHPortForwarding = &types.SSHPortForwarding{
			Remote: &types.SSHRemotePortForwarding{
				Enabled: remote,
			},
			Local: &types.SSHLocalPortForwarding{
				Enabled: local,
			},
		}
	}

	role.SetOptions(roleOptions)
	_, err = f.testSrv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
}

// TestDirectTCPIP ensures that the server can create a "direct-tcpip"
// channel to the target address. The "direct-tcpip" channel is what port
// forwarding is built upon.
func TestDirectTCPIP(t *testing.T) {
	ctx := context.Background()

	setup := func(t *testing.T) (*sshTestFixture, *httptest.Server, *url.URL) {
		f := newFixtureWithoutDiskBasedLogging(t)

		// Startup a test server that will reply with "hello, world\n"
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, "hello, world")
		}))

		// Extract the host:port the test HTTP server is running on.
		u, err := url.Parse(ts.URL)
		require.NoError(t, err)

		return f, ts, u
	}

	t.Run("Local forwarding is successful", func(t *testing.T) {
		f, ts, u := setup(t)
		defer ts.Close()

		// Build a http.Client that will dial through the server to establish the
		// connection. That's why a custom dialer is used and the dialer uses
		// s.clt.Dial (which performs the "direct-tcpip" request).
		httpClient := http.Client{
			Transport: &http.Transport{
				Dial: func(network string, addr string) (net.Conn, error) {
					return f.ssh.clt.DialContext(context.Background(), "tcp", u.Host)
				},
			},
		}

		// Perform a HTTP GET to the test HTTP server through a "direct-tcpip" request.
		resp, err := httpClient.Get(ts.URL)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Make sure the response is what was expected.
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, []byte("hello, world\n"), body)
	})

	t.Run("Local forwarding fails when access is denied", func(t *testing.T) {
		f, ts, u := setup(t)
		defer ts.Close()

		// update rules before creating conn to ensure that permissions are calculated
		// using the updated rules.
		setPortForwarding(t, ctx, f, nil, nil, types.NewBoolOption(false))

		// create a new client connection to the node
		clientConn, err := tracessh.Dial(ctx, "tcp", f.ssh.srvAddress, f.ssh.cltConfig)
		require.NoError(t, err)
		defer clientConn.Close()

		// create an http client that does forwarding through the node
		httpClient := http.Client{
			Transport: &http.Transport{
				Dial: func(network string, addr string) (net.Conn, error) {
					return clientConn.DialContext(context.Background(), "tcp", u.Host)
				},
			},
		}

		// Perform a HTTP GET to the test HTTP server through a "direct-tcpip" request.
		//nolint:bodyclose // We expect an error here, no need to close.
		_, err = httpClient.Get(ts.URL)
		require.Error(t, err)
	})

	t.Run("Local forwarding fails when access is denied by legacy config", func(t *testing.T) {
		f, ts, u := setup(t)
		defer ts.Close()

		// update rules before creating conn to ensure that permissions are calculated
		// using the updated rules.
		setPortForwarding(t, ctx, f, types.NewBoolOption(false), nil, nil)

		// create a new client connection to the node
		clientConn, err := tracessh.Dial(ctx, "tcp", f.ssh.srvAddress, f.ssh.cltConfig)
		require.NoError(t, err)
		defer clientConn.Close()

		// create an http client that does forwarding through the node
		httpClient := http.Client{
			Transport: &http.Transport{
				Dial: func(network string, addr string) (net.Conn, error) {
					return clientConn.DialContext(context.Background(), "tcp", u.Host)
				},
			},
		}

		// Perform a HTTP GET to the test HTTP server through a "direct-tcpip" request.
		//nolint:bodyclose // We expect an error here, no need to close.
		_, err = httpClient.Get(ts.URL)
		require.Error(t, err)
	})

	t.Run("SessionJoinPrincipal cannot use direct-tcpip", func(t *testing.T) {
		f, ts, u := setup(t)
		defer ts.Close()

		// Ensure that ssh client using SessionJoinPrincipal as Login, cannot
		// connect using "direct-tcpip".
		ctx := context.Background()
		cliUsingSessionJoin := f.newSSHClient(ctx, t, &user.User{Username: teleport.SSHSessionJoinPrincipal})
		httpClientUsingSessionJoin := http.Client{
			Transport: &http.Transport{
				Dial: func(network string, addr string) (net.Conn, error) {
					return cliUsingSessionJoin.DialContext(ctx, "tcp", u.Host)
				},
			},
		}
		//nolint:bodyclose // We expect an error here, no need to close.
		_, err := httpClientUsingSessionJoin.Get(ts.URL)
		require.ErrorContains(t, err, "ssh: rejected: administratively prohibited (attempted direct-tcpip channel open in join-only mode")
	})
}

// TestTCPIPForward ensures that the server can create a listener from a
// "tcpip-forward" request and do remote port forwarding.
func TestTCPIPForward(t *testing.T) {
	t.Parallel()
	hostname, err := os.Hostname()
	require.NoError(t, err)
	tests := []struct {
		name        string
		listenAddr  string
		legacyAllow *types.BoolOption
		remoteAllow *types.BoolOption
		localAllow  *types.BoolOption
		expectErr   bool
	}{
		{
			name:       "localhost",
			listenAddr: "localhost:0",
		},
		{
			name:       "ip address",
			listenAddr: "127.0.0.1:0",
		},
		{
			name:       "hostname",
			listenAddr: hostname + ":0",
		},
		{
			name:        "remote deny",
			listenAddr:  "localhost:0",
			remoteAllow: types.NewBoolOption(false),
			expectErr:   true,
		},
		{
			name:        "legacy deny",
			listenAddr:  "localhost:0",
			legacyAllow: types.NewBoolOption(false),
			expectErr:   true,
		},
		{
			name:       "local deny",
			listenAddr: "localhost:0",
			localAllow: types.NewBoolOption(false),
			expectErr:  false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixtureWithoutDiskBasedLogging(t)
			setPortForwarding(t, context.Background(), f, tc.legacyAllow, tc.remoteAllow, tc.localAllow)

			// create a new client connection to the node which will have its permissions
			// calculated with the updated rules.
			clientConn, err := tracessh.Dial(context.Background(), "tcp", f.ssh.srvAddress, f.ssh.cltConfig)
			require.NoError(t, err)
			defer clientConn.Close()

			// Request a listener from the server.
			listener, err := clientConn.Listen("tcp", tc.listenAddr)
			if tc.expectErr {
				require.Error(t, err)
				return
			} else {
				require.NoError(t, err)
			}

			// Start up a test server that uses the port forwarded listener.
			ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, "hello, world")
			}))
			t.Cleanup(ts.Close)
			ts.Listener = listener
			ts.Start()

			// Dial the test server over the SSH connection.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			t.Cleanup(cancel)
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL, nil)
			require.NoError(t, err)
			resp, err := ts.Client().Do(req)
			require.NoError(t, err)

			t.Cleanup(func() {
				require.NoError(t, resp.Body.Close())
			})

			// Make sure the response is what was expected.
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, []byte("hello, world\n"), body)
		})
	}

	t.Run("SessionJoinPrincipal cannot use tcpip-forward", func(t *testing.T) {
		// Ensure that ssh client using SessionJoinPrincipal as Login, cannot
		// connect using "tcpip-forward".
		f := newFixtureWithoutDiskBasedLogging(t)
		ctx := context.Background()
		cliUsingSessionJoin := f.newSSHClient(ctx, t, &user.User{Username: teleport.SSHSessionJoinPrincipal})
		_, err := cliUsingSessionJoin.Listen("tcp", "127.0.0.1:0")
		require.ErrorContains(t, err, "ssh: tcpip-forward request denied by peer")
	})
}

func TestAdvertiseAddr(t *testing.T) {
	t.Parallel()
	f := newFixtureWithoutDiskBasedLogging(t)

	// No advertiseAddr was set in fixture, should default to srvAddress.
	require.Equal(t, f.ssh.srv.Addr(), f.ssh.srv.AdvertiseAddr())

	var (
		advIP      = utils.MustParseAddr("10.10.10.1")
		advIPPort  = utils.MustParseAddr("10.10.10.1:1234")
		advBadAddr = &utils.NetAddr{Addr: "localhost:badport", AddrNetwork: "tcp"}
	)
	// IP-only advertiseAddr should use the port from srvAddress.
	f.ssh.srv.setAdvertiseAddr(advIP)
	require.Equal(t, fmt.Sprintf("%s:%s", advIP, f.ssh.srvPort), f.ssh.srv.AdvertiseAddr())

	// IP and port advertiseAddr should fully override srvAddress.
	f.ssh.srv.setAdvertiseAddr(advIPPort)
	require.Equal(t, advIPPort.String(), f.ssh.srv.AdvertiseAddr())

	// nil advertiseAddr should default to srvAddress.
	f.ssh.srv.setAdvertiseAddr(nil)
	require.Equal(t, f.ssh.srvAddress, f.ssh.srv.AdvertiseAddr())

	// Invalid advertiseAddr should fall back to srvAddress.
	f.ssh.srv.setAdvertiseAddr(advBadAddr)
	require.Equal(t, f.ssh.srvAddress, f.ssh.srv.AdvertiseAddr())
}

// TestAgentForwardPermission makes sure if RBAC rules don't allow agent
// forwarding, we don't start an agent even if requested.
func TestAgentForwardPermission(t *testing.T) {
	t.Parallel()

	f := newFixtureWithoutDiskBasedLogging(t)
	ctx := context.Background()

	// make sure the role does not allow agent forwarding
	roleName := services.RoleNameForUser(f.user)
	role, err := f.testSrv.Auth().GetRole(ctx, roleName)
	require.NoError(t, err)

	roleOptions := role.GetOptions()
	roleOptions.ForwardAgent = types.NewBool(false)
	role.SetOptions(roleOptions)
	_, err = f.testSrv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	// create a new client connection to the node which will have its permissions
	// calculated with the updated rules.
	clientConn, err := tracessh.Dial(ctx, "tcp", f.ssh.srvAddress, f.ssh.cltConfig)
	require.NoError(t, err)
	defer clientConn.Close()

	se, err := clientConn.NewSession(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { se.Close() })

	// to interoperate with OpenSSH, requests for agent forwarding always succeed.
	// however that does not mean the users agent will actually be forwarded.
	require.NoError(t, agent.RequestAgentForwarding(se.Session))

	// the output of env, we should not see SSH_AUTH_SOCK in the output
	output, err := se.Output(ctx, "env")
	require.NoError(t, err)
	require.NotContains(t, string(output), "SSH_AUTH_SOCK")
}

// TestMaxSessions makes sure that MaxSessions RBAC rules prevent
// too many concurrent sessions.
func TestMaxSessions(t *testing.T) {
	t.Parallel()

	const maxSessions int64 = 2
	f := newFixtureWithoutDiskBasedLogging(t)
	ctx := context.Background()
	// make sure the role does not allow agent forwarding
	roleName := services.RoleNameForUser(f.user)
	role, err := f.testSrv.Auth().GetRole(ctx, roleName)
	require.NoError(t, err)
	roleOptions := role.GetOptions()
	roleOptions.MaxSessions = maxSessions
	role.SetOptions(roleOptions)
	_, err = f.testSrv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	// create a new client connection to the node which will have its permissions
	// calculated with the updated rules.
	clientConn, err := tracessh.Dial(ctx, "tcp", f.ssh.srvAddress, f.ssh.cltConfig)
	require.NoError(t, err)
	defer clientConn.Close()

	for range maxSessions {
		se, err := clientConn.NewSession(ctx)
		require.NoError(t, err)
		defer se.Close()
	}

	_, err = clientConn.NewSession(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "too many session channels")

	// verify that max sessions does not affect max connections.
	for i := int64(0); i <= maxSessions; i++ {
		clt, err := tracessh.Dial(ctx, "tcp", f.ssh.srv.Addr(), f.ssh.cltConfig)
		require.NoError(t, err)
		require.NoError(t, clt.Close())
	}
}

// TestExecLongCommand makes sure that commands that are longer than the
// maximum pipe size on the OS can still be started. This tests the reexec
// functionality of Teleport as Teleport will reexec itself when launching a
// command and send the command to then launch through a pipe.
func TestExecLongCommand(t *testing.T) {
	t.Parallel()
	f := newFixtureWithoutDiskBasedLogging(t)
	ctx := context.Background()

	// Get the path to where the "echo" command is on disk.
	echoPath, err := exec.LookPath("echo")
	require.NoError(t, err)

	se, err := f.ssh.clt.NewSession(ctx)
	require.NoError(t, err)
	defer se.Close()

	// Write a message that larger than the maximum pipe size.
	_, err = se.Output(ctx, fmt.Sprintf("%v %v", echoPath, strings.Repeat("a", maxPipeSize)))
	require.NoError(t, err)
}

// TestOpenExecSessionSetsSession tests that OpenExecSession()
// sets ServerContext session.
func TestOpenExecSessionSetsSession(t *testing.T) {
	t.Parallel()
	f := newFixtureWithoutDiskBasedLogging(t)
	ctx := context.Background()

	se, err := f.ssh.clt.NewSession(ctx)
	require.NoError(t, err)
	defer se.Close()

	// This will trigger an exec request, which will start a non-interactive session,
	// which then triggers setting env for SSH_SESSION_ID.
	output, err := se.Output(ctx, "env")
	require.NoError(t, err)
	require.Contains(t, string(output), teleport.SSHSessionID)
}

// TestAgentForward tests agent forwarding via unix sockets
func TestAgentForward(t *testing.T) {
	t.Parallel()
	f := newFixtureWithoutDiskBasedLogging(t)

	ctx := context.Background()
	roleName := services.RoleNameForUser(f.user)
	role, err := f.testSrv.Auth().GetRole(ctx, roleName)
	require.NoError(t, err)
	roleOptions := role.GetOptions()
	roleOptions.ForwardAgent = types.NewBool(true)
	role.SetOptions(roleOptions)
	_, err = f.testSrv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	se, err := f.ssh.clt.NewSession(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { se.Close() })

	err = agent.RequestAgentForwarding(se.Session)
	require.NoError(t, err)

	// prepare to send virtual "keyboard input" into the shell:
	keyboard, err := se.StdinPipe()
	require.NoError(t, err)
	t.Cleanup(func() { keyboard.Close() })

	// start interactive SSH session (new shell):
	err = se.Shell(ctx)
	require.NoError(t, err)

	// create a temp file to collect the shell output into:
	tmpFile, err := os.CreateTemp(t.TempDir(), "teleport-agent-forward-test")
	require.NoError(t, err)
	tmpFile.Close()

	// type 'printenv SSH_AUTH_SOCK > /path/to/tmp/file' into the session (dumping the value of SSH_AUTH_STOCK into the temp file)
	_, err = fmt.Fprintf(keyboard, "printenv %v >> %s\n\r", teleport.SSHAuthSock, tmpFile.Name())
	require.NoError(t, err)

	// wait for the output
	var socketPath string
	require.Eventually(t, func() bool {
		output, err := os.ReadFile(tmpFile.Name())
		if err == nil && len(output) != 0 {
			socketPath = strings.TrimSpace(string(output))
			return true
		}
		return false
	}, 10*time.Second, 100*time.Millisecond, "failed to read socket path")

	// try dialing the ssh agent socket:
	file, err := net.Dial("unix", socketPath)
	require.NoError(t, err)

	clientAgent := agent.NewClient(file)
	signers, err := clientAgent.Signers()
	require.NoError(t, err)

	sshConfig := &ssh.ClientConfig{
		User:            f.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signers...)},
		HostKeyCallback: ssh.FixedHostKey(f.signer.PublicKey()),
	}

	client, err := tracessh.Dial(ctx, "tcp", f.ssh.srv.Addr(), sshConfig)
	require.NoError(t, err)
	err = client.Close()
	require.NoError(t, err)

	// make sure the socket persists after the session is closed.
	// (agents are started from specific sessions, but apply to all
	// sessions on the connection).
	err = se.Close()
	require.NoError(t, err)

	// Pause to allow closure to propagate.
	time.Sleep(150 * time.Millisecond)
	_, err = net.Dial("unix", socketPath)
	require.NoError(t, err)

	// make sure the socket is gone after we closed the connection. Note that
	// we now expect the client close to fail during the test cleanup, so we
	// change the assertion accordingly
	require.NoError(t, f.ssh.clt.Close())
	f.ssh.assertCltClose = require.Error

	// clt must be nullified to prevent double-close during test cleanup
	f.ssh.clt = nil
	require.Eventually(t, func() bool {
		_, err := clientAgent.List()
		return err != nil
	},
		10*time.Second, 100*time.Millisecond,
		"expected socket to be closed, still could dial")
}

// TestX11Forward tests x11 forwarding via unix sockets
func TestX11Forward(t *testing.T) {
	ctx := context.Background()
	if os.Getenv("TELEPORT_XAUTH_TEST") == "" {
		t.Skip("Skipping test as xauth is not enabled")
	}

	t.Parallel()
	f := newFixtureWithoutDiskBasedLogging(t)
	f.ssh.srv.x11 = &x11.ServerConfig{
		Enabled:       true,
		DisplayOffset: x11.DefaultDisplayOffset,
		MaxDisplay:    x11.DefaultMaxDisplays,
	}

	roleName := services.RoleNameForUser(f.user)
	role, err := f.testSrv.Auth().GetRole(ctx, roleName)
	require.NoError(t, err)
	roleOptions := role.GetOptions()
	roleOptions.PermitX11Forwarding = types.NewBool(true)
	role.SetOptions(roleOptions)
	_, err = f.testSrv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	// Open two x11 sessions from two separate clients to
	// ensure concurrent X11 forwarding sessions are supported.
	serverDisplay := x11EchoSession(ctx, t, f.ssh.clt)
	user, err := user.Current()
	require.NoError(t, err)
	client2 := f.newSSHClient(ctx, t, user)
	serverDisplay2 := x11EchoSession(ctx, t, client2)

	// Create multiple XServer requests, the server should
	// handle multiple concurrent XServer requests.
	errCh := make(chan error)
	go func() {
		errCh <- x11EchoRequest(serverDisplay)
	}()
	go func() {
		errCh <- x11EchoRequest(serverDisplay)
	}()
	go func() {
		errCh <- x11EchoRequest(serverDisplay2)
	}()
	go func() {
		errCh <- x11EchoRequest(serverDisplay2)
	}()

	for range 4 {
		select {
		case err := <-errCh:
			assert.NoError(t, err)
		case <-ctx.Done():
			assert.NoError(t, context.Cause(ctx))
		}
	}
}

// x11EchoSession creates a new ssh session and handles x11 forwarding for the session,
// echoing XServer requests received back to the client. Returns the Display opened on the
// session, which is set in $DISPLAY.
func x11EchoSession(ctx context.Context, t *testing.T, clt *tracessh.Client) x11.Display {
	se, err := clt.NewSession(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { se.Close() })

	// Create a fake client XServer listener which echos
	// back whatever it receives.
	fakeClientDisplay, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	go func() {
		for {
			conn, err := fakeClientDisplay.Accept()
			if err != nil {
				return
			}
			_, err = io.Copy(conn, conn)
			assert.NoError(t, err)
			conn.Close()
		}
	}()
	t.Cleanup(func() { fakeClientDisplay.Close() })

	// Handle any x11 channel requests received from the server
	// and start x11 forwarding to the client display.
	err = x11.ServeChannelRequests(ctx, clt.Client, func(ctx context.Context, nch ssh.NewChannel) {
		sch, sin, err := nch.Accept()
		assert.NoError(t, err)
		defer sch.Close()

		clientConn, err := net.Dial("tcp", fakeClientDisplay.Addr().String())
		assert.NoError(t, err)
		clientXConn, ok := clientConn.(*net.TCPConn)
		assert.True(t, ok)
		defer clientConn.Close()

		go func() {
			_ = sshutils.ForwardRequests(ctx, sin, se)
		}()

		err = utils.ProxyConn(ctx, clientXConn, sch)

		// Error should be nil if the ssh client is closed first, or canceled if the context is closed first.
		if !errors.Is(err, context.Canceled) {
			assert.NoError(t, err)
		}
	})
	require.NoError(t, err)

	// Client requests x11 forwarding for the server session.
	clientXAuthEntry, err := x11.NewFakeXAuthEntry(x11.Display{})
	require.NoError(t, err)
	err = x11.RequestForwarding(se.Session, clientXAuthEntry)
	require.NoError(t, err)

	// prepare to send virtual "keyboard input" into the shell:
	keyboard, err := se.StdinPipe()
	require.NoError(t, err)

	// start interactive SSH session with x11 forwarding enabled (new shell):
	err = se.Shell(ctx)
	require.NoError(t, err)

	// create a temp file to collect the shell output into:
	tmpFile, err := os.CreateTemp(os.TempDir(), "teleport-x11-forward-test")
	require.NoError(t, err)

	// Allow non-root user to write to the temp file
	err = tmpFile.Chmod(fs.FileMode(0o777))
	require.NoError(t, err)
	t.Cleanup(func() {
		os.Remove(tmpFile.Name())
	})

	// Reading the display may fail if the session is not fully initialized
	// and the write to stdin is swallowed.
	display := make(chan string, 1)
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		// enter 'printenv DISPLAY > /path/to/tmp/file' into the session (dumping the value of DISPLAY into the temp file)
		_, err = fmt.Fprintf(keyboard, "printenv %v > %s\n\r", x11.DisplayEnv, tmpFile.Name())
		assert.NoError(t, err)

		assert.Eventually(t, func() bool {
			output, err := os.ReadFile(tmpFile.Name())
			if err == nil && len(output) != 0 {
				select {
				case display <- strings.TrimSpace(string(output)):
				default:
				}
				return true
			}
			return false
		}, time.Second, 100*time.Millisecond, "failed to read display")
	}, 10*time.Second, 1*time.Second)

	// Make a new connection to the XServer proxy, the client
	// XServer should echo back anything written on it.
	serverDisplay, err := x11.ParseDisplay(<-display)
	require.NoError(t, err)

	return serverDisplay
}

// x11EchoRequest sends a message to the serverDisplay and expects the
// server to echo the message back to it.
func x11EchoRequest(serverDisplay x11.Display) error {
	conn, err := serverDisplay.Dial()
	if err != nil {
		return err
	}
	defer conn.Close()

	msg := "msg"
	_, err = conn.Write([]byte(msg))
	if err != nil {
		return err
	}

	buf := make([]byte, 3)
	_, err = conn.Read(buf)
	if err != nil {
		return err
	}

	if string(buf) != msg {
		return trace.Errorf("x11 echo request returned a different message than expected")
	}

	return nil
}

func TestAllowedUsers(t *testing.T) {
	t.Parallel()
	f := newFixtureWithoutDiskBasedLogging(t)

	up, err := newUpack(f.testSrv, f.user, []string{f.user}, wildcardAllow)
	require.NoError(t, err)

	sshConfig := &ssh.ClientConfig{
		User:            f.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(f.signer.PublicKey()),
	}

	client, err := tracessh.Dial(context.Background(), "tcp", f.ssh.srv.Addr(), sshConfig)
	require.NoError(t, err)
	require.NoError(t, client.Close())

	// now remove OS user from valid principals
	up, err = newUpack(f.testSrv, f.user, []string{"otheruser"}, wildcardAllow)
	require.NoError(t, err)

	sshConfig = &ssh.ClientConfig{
		User:            f.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(f.signer.PublicKey()),
	}

	_, err = tracessh.Dial(context.Background(), "tcp", f.ssh.srv.Addr(), sshConfig)
	require.Error(t, err)
}

func TestAllowedLabels(t *testing.T) {
	t.Parallel()
	f := newFixtureWithoutDiskBasedLogging(t)

	tests := []struct {
		desc       string
		inLabelMap types.Labels
		outError   bool
	}{
		// Valid static label.
		{
			desc:       "Valid Static",
			inLabelMap: types.Labels{"foo": []string{"bar"}},
			outError:   false,
		},
		// Invalid static label.
		{
			desc:       "Invalid Static",
			inLabelMap: types.Labels{"foo": []string{"baz"}},
			outError:   true,
		},
		// Valid dynamic label.
		{
			desc:       "Valid Dynamic",
			inLabelMap: types.Labels{"baz": []string{"4"}},
			outError:   false,
		},
		// Invalid dynamic label.
		{
			desc:       "Invalid Dynamic",
			inLabelMap: types.Labels{"baz": []string{"5"}},
			outError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			up, err := newUpack(f.testSrv, f.user, []string{f.user}, tt.inLabelMap)
			require.NoError(t, err)

			sshConfig := &ssh.ClientConfig{
				User:            f.user,
				Auth:            []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
				HostKeyCallback: ssh.FixedHostKey(f.signer.PublicKey()),
			}

			_, err = tracessh.Dial(context.Background(), "tcp", f.ssh.srv.Addr(), sshConfig)
			if tt.outError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestInvalidSessionID(t *testing.T) {
	t.Parallel()
	f := newFixtureWithoutDiskBasedLogging(t)
	ctx := context.Background()

	session, err := f.ssh.clt.NewSession(ctx)
	require.NoError(t, err)

	err = session.Setenv(ctx, sshutils.SessionEnvVar, "foo")
	require.NoError(t, err)

	err = session.Shell(ctx)
	require.Error(t, err)
}

func TestSessionHijack(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	_, err := user.Lookup(teleportTestUser)
	if err != nil {
		t.Skipf("user %v is not found, skipping test", teleportTestUser)
	}

	f := newFixtureWithoutDiskBasedLogging(t)

	// user 1 has access to the server
	up, err := newUpack(f.testSrv, f.user, []string{f.user}, wildcardAllow)
	require.NoError(t, err)

	// login with first user
	sshConfig := &ssh.ClientConfig{
		User:            f.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(f.signer.PublicKey()),
	}

	client, err := tracessh.Dial(ctx, "tcp", f.ssh.srv.Addr(), sshConfig)
	require.NoError(t, err)
	defer func() {
		err := client.Close()
		require.NoError(t, err)
	}()

	se, err := client.NewSession(ctx)
	require.NoError(t, err)
	defer se.Close()

	firstSessionID := string(sess.NewID())
	err = se.Setenv(ctx, sshutils.SessionEnvVar, firstSessionID)
	require.NoError(t, err)

	err = se.Shell(ctx)
	require.NoError(t, err)

	// user 2 does not have s.user as a listed principal
	up2, err := newUpack(f.testSrv, teleportTestUser, []string{teleportTestUser}, wildcardAllow)
	require.NoError(t, err)

	sshConfig2 := &ssh.ClientConfig{
		User:            teleportTestUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(up2.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(f.signer.PublicKey()),
	}

	client2, err := tracessh.Dial(ctx, "tcp", f.ssh.srv.Addr(), sshConfig2)
	require.NoError(t, err)
	defer func() {
		err := client2.Close()
		require.NoError(t, err)
	}()

	se2, err := client2.NewSession(ctx)
	require.NoError(t, err)
	defer se2.Close()

	err = se2.Setenv(ctx, sshutils.SessionEnvVar, firstSessionID)
	require.NoError(t, err)

	// attempt to hijack, should return error
	err = se2.Shell(ctx)
	require.Error(t, err)
}

// testClient dials targetAddr via proxyAddr and executes 2+3 command
func testClient(t *testing.T, f *sshTestFixture, proxyAddr, targetAddr, remoteAddr string, sshConfig *ssh.ClientConfig) {
	ctx := context.Background()
	// Connect to node using registered address
	client, err := tracessh.Dial(ctx, "tcp", proxyAddr, sshConfig)
	require.NoError(t, err)
	defer client.Close()

	se, err := client.NewSession(ctx)
	require.NoError(t, err)
	defer se.Close()

	writer, err := se.StdinPipe()
	require.NoError(t, err)

	reader, err := se.StdoutPipe()
	require.NoError(t, err)

	// Request opening TCP connection to the remote host
	require.NoError(t, se.RequestSubsystem(ctx, fmt.Sprintf("proxy:%v", targetAddr)))

	local, err := utils.ParseAddr("tcp://" + proxyAddr)
	require.NoError(t, err)
	remote, err := utils.ParseAddr("tcp://" + remoteAddr)
	require.NoError(t, err)

	pipeNetConn := utils.NewPipeNetConn(
		reader,
		writer,
		se,
		local,
		remote,
	)

	defer pipeNetConn.Close()

	// Open SSH connection via TCP
	conn, chans, reqs, err := tracessh.NewClientConn(
		ctx,
		pipeNetConn,
		f.ssh.srv.Addr(),
		sshConfig,
	)
	require.NoError(t, err)
	defer conn.Close()

	// using this connection as regular SSH
	client2 := tracessh.NewClient(conn, chans, reqs)
	require.NoError(t, err)
	defer client2.Close()

	se2, err := client2.NewSession(ctx)
	require.NoError(t, err)
	defer se2.Close()

	out, err := se2.Output(ctx, "echo hello")
	require.NoError(t, err)

	require.Equal(t, "hello\n", string(out))
}

func mustListen(t *testing.T) (net.Listener, utils.NetAddr) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := utils.NetAddr{AddrNetwork: "tcp", Addr: l.Addr().String()}
	return l, addr
}

func noCache(clt authclient.ClientI, cacheName []string) (authclient.RemoteProxyAccessPoint, error) {
	return clt, nil
}

func TestProxyRoundRobin(t *testing.T) {
	t.Parallel()

	f := newFixtureWithoutDiskBasedLogging(t)
	ctx := context.Background()

	proxyClient, _ := newProxyClient(t, f.testSrv)
	nodeClient, _ := newNodeClient(t, f.testSrv)

	listener, reverseTunnelAddress := mustListen(t)
	defer listener.Close()
	lockWatcher := newLockWatcher(ctx, t, proxyClient)
	nodeWatcher := newNodeWatcher(ctx, t, proxyClient)
	caWatcher := newCertAuthorityWatcher(ctx, t, proxyClient)

	reverseTunnelServer, err := reversetunnel.NewServer(reversetunnel.Config{
		GetClientTLSCertificate: func() (*tls.Certificate, error) {
			return &proxyClient.TLSConfig().Certificates[0], nil
		},
		ClusterName:           f.testSrv.ClusterName(),
		ID:                    hostID,
		Listener:              listener,
		GetHostSigners:        sshutils.StaticHostSigners(f.signer),
		LocalAuthClient:       proxyClient,
		LocalAccessPoint:      proxyClient,
		NewCachingAccessPoint: noCache,
		DataDir:               t.TempDir(),
		Emitter:               proxyClient,
		LockWatcher:           lockWatcher,
		NodeWatcher:           nodeWatcher,
		GitServerWatcher:      newGitServerWatcher(ctx, t, proxyClient),
		CertAuthorityWatcher:  caWatcher,
		CircuitBreakerConfig:  breaker.NoopBreakerConfig(),
		EICESigner: func(ctx context.Context, target types.Server, integration types.Integration, login, token string, ap cryptosuites.AuthPreferenceGetter) (ssh.Signer, error) {
			return nil, errors.New("eice disabled in tests")
		},
		EICEDialer: func(ctx context.Context, target types.Server, integration types.Integration, token string) (net.Conn, error) {
			return nil, errors.New("eice disabled in tests")
		},
	})
	require.NoError(t, err)

	require.NoError(t, reverseTunnelServer.Start())
	defer reverseTunnelServer.Close()

	router, err := libproxy.NewRouter(libproxy.RouterConfig{
		ClusterName:      f.testSrv.ClusterName(),
		LocalAccessPoint: proxyClient,
		SiteGetter:       reverseTunnelServer,
		TracerProvider:   tracing.NoopProvider(),
	})
	require.NoError(t, err)

	sessionController, err := srv.NewSessionController(srv.SessionControllerConfig{
		Semaphores:   proxyClient,
		AccessPoint:  proxyClient,
		LockEnforcer: lockWatcher,
		Emitter:      proxyClient,
		Component:    teleport.ComponentNode,
		ServerID:     hostID,
	})
	require.NoError(t, err)

	proxy, err := New(
		ctx,
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		f.testSrv.ClusterName(),
		sshutils.StaticHostSigners(f.signer),
		proxyClient,
		t.TempDir(),
		"",
		utils.NetAddr{},
		proxyClient,
		SetProxyMode("", reverseTunnelServer, proxyClient, router),
		SetEmitter(nodeClient),
		SetNamespace(apidefaults.Namespace),
		SetPAMConfig(&servicecfg.PAMConfig{Enabled: false}),
		SetBPF(&bpf.NOP{}),
		SetClock(f.clock),
		SetLockWatcher(lockWatcher),
		SetSessionController(sessionController),
		SetConnectedProxyGetter(reversetunnel.NewConnectedProxyGetter()),
	)
	require.NoError(t, err)
	require.NoError(t, proxy.Start())
	defer proxy.Close()

	// set up SSH client using the user private key for signing
	up, err := newUpack(f.testSrv, f.user, []string{f.user}, wildcardAllow)
	require.NoError(t, err)

	resolver := func(context.Context) (*utils.NetAddr, types.ProxyListenerMode, error) {
		return &utils.NetAddr{Addr: reverseTunnelAddress.Addr, AddrNetwork: "tcp"}, types.ProxyListenerMode_Separate, nil
	}

	pool1, err := reversetunnel.NewAgentPool(ctx, reversetunnel.AgentPoolConfig{
		Resolver:    resolver,
		Client:      proxyClient,
		AccessPoint: proxyClient,
		AuthMethods: []ssh.AuthMethod{ssh.PublicKeys(f.signer)},
		HostUUID:    fmt.Sprintf("%v.%v", hostID, f.testSrv.ClusterName()),
		Cluster:     "remote",
	})
	require.NoError(t, err)

	err = pool1.Start()
	require.NoError(t, err)
	defer pool1.Stop()

	pool2, err := reversetunnel.NewAgentPool(ctx, reversetunnel.AgentPoolConfig{
		Resolver:    resolver,
		Client:      proxyClient,
		AccessPoint: proxyClient,
		AuthMethods: []ssh.AuthMethod{ssh.PublicKeys(f.signer)},
		HostUUID:    fmt.Sprintf("%v.%v", hostID, f.testSrv.ClusterName()),
		Cluster:     "remote",
	})
	require.NoError(t, err)

	err = pool2.Start()
	require.NoError(t, err)
	defer pool2.Stop()

	sshConfig := &ssh.ClientConfig{
		User:            f.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(f.signer.PublicKey()),
	}

	_, err = newUpack(f.testSrv, "user1", []string{f.user}, wildcardAllow)
	require.NoError(t, err)

	for range 3 {
		testClient(t, f, proxy.Addr(), f.ssh.srvAddress, f.ssh.srv.Addr(), sshConfig)
	}

	// close first connection, and test it again
	pool1.Stop()

	for range 3 {
		testClient(t, f, proxy.Addr(), f.ssh.srvAddress, f.ssh.srv.Addr(), sshConfig)
	}
}

// TestProxyDirectAccess tests direct access via proxy bypassing
// reverse tunnel
func TestProxyDirectAccess(t *testing.T) {
	t.Parallel()

	f := newFixtureWithoutDiskBasedLogging(t)
	ctx := context.Background()

	listener, _ := mustListen(t)
	proxyClient, _ := newProxyClient(t, f.testSrv)
	lockWatcher := newLockWatcher(ctx, t, proxyClient)
	nodeWatcher := newNodeWatcher(ctx, t, proxyClient)
	caWatcher := newCertAuthorityWatcher(ctx, t, proxyClient)

	reverseTunnelServer, err := reversetunnel.NewServer(reversetunnel.Config{
		GetClientTLSCertificate: func() (*tls.Certificate, error) {
			return &proxyClient.TLSConfig().Certificates[0], nil
		},
		ID:                    hostID,
		ClusterName:           f.testSrv.ClusterName(),
		Listener:              listener,
		GetHostSigners:        sshutils.StaticHostSigners(f.signer),
		LocalAuthClient:       proxyClient,
		LocalAccessPoint:      proxyClient,
		NewCachingAccessPoint: noCache,
		DataDir:               t.TempDir(),
		Emitter:               proxyClient,
		LockWatcher:           lockWatcher,
		NodeWatcher:           nodeWatcher,
		GitServerWatcher:      newGitServerWatcher(ctx, t, proxyClient),
		CertAuthorityWatcher:  caWatcher,
		CircuitBreakerConfig:  breaker.NoopBreakerConfig(),
		EICESigner: func(ctx context.Context, target types.Server, integration types.Integration, login, token string, ap cryptosuites.AuthPreferenceGetter) (ssh.Signer, error) {
			return nil, errors.New("eice disabled in tests")
		},
		EICEDialer: func(ctx context.Context, target types.Server, integration types.Integration, token string) (net.Conn, error) {
			return nil, errors.New("eice disabled in tests")
		},
	})
	require.NoError(t, err)

	require.NoError(t, reverseTunnelServer.Start())
	defer reverseTunnelServer.Close()

	nodeClient, _ := newNodeClient(t, f.testSrv)

	router, err := libproxy.NewRouter(libproxy.RouterConfig{
		ClusterName:      f.testSrv.ClusterName(),
		LocalAccessPoint: proxyClient,
		SiteGetter:       reverseTunnelServer,
		TracerProvider:   tracing.NoopProvider(),
	})
	require.NoError(t, err)

	sessionController, err := srv.NewSessionController(srv.SessionControllerConfig{
		Semaphores:   nodeClient,
		AccessPoint:  nodeClient,
		LockEnforcer: lockWatcher,
		Emitter:      nodeClient,
		Component:    teleport.ComponentNode,
		ServerID:     hostID,
	})
	require.NoError(t, err)

	proxy, err := New(
		ctx,
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		f.testSrv.ClusterName(),
		sshutils.StaticHostSigners(f.signer),
		proxyClient,
		t.TempDir(),
		"",
		utils.NetAddr{},
		proxyClient,
		SetProxyMode("", reverseTunnelServer, proxyClient, router),
		SetEmitter(nodeClient),
		SetNamespace(apidefaults.Namespace),
		SetPAMConfig(&servicecfg.PAMConfig{Enabled: false}),
		SetBPF(&bpf.NOP{}),
		SetClock(f.clock),
		SetLockWatcher(lockWatcher),
		SetSessionController(sessionController),
		SetConnectedProxyGetter(reversetunnel.NewConnectedProxyGetter()),
	)
	require.NoError(t, err)
	require.NoError(t, proxy.Start())
	defer proxy.Close()

	// set up SSH client using the user private key for signing
	up, err := newUpack(f.testSrv, f.user, []string{f.user}, wildcardAllow)
	require.NoError(t, err)

	sshConfig := &ssh.ClientConfig{
		User:            f.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(f.signer.PublicKey()),
	}

	_, err = newUpack(f.testSrv, "user1", []string{f.user}, wildcardAllow)
	require.NoError(t, err)

	testClient(t, f, proxy.Addr(), f.ssh.srvAddress, f.ssh.srv.Addr(), sshConfig)
}

// TestPTY requests PTY for an interactive session
func TestPTY(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	f := newFixtureWithoutDiskBasedLogging(t)

	se, err := f.ssh.clt.NewSession(ctx)
	require.NoError(t, err)
	defer se.Close()

	// request PTY with valid size
	require.NoError(t, se.RequestPty(ctx, "xterm", 30, 30, ssh.TerminalModes{}))

	// request PTY with invalid size, should still work (selects defaults)
	require.NoError(t, se.RequestPty(ctx, "xterm", 0, 0, ssh.TerminalModes{}))
}

// TestEnv requests setting environment variables via
// a "env" request.
func TestEnv(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	f := newFixtureWithoutDiskBasedLogging(t)

	se, err := f.ssh.clt.NewSession(ctx)
	require.NoError(t, err)
	defer se.Close()

	require.NoError(t, se.Setenv(ctx, "HOME_TEST", "/test"))
	output, err := se.Output(ctx, "env")
	require.NoError(t, err)
	require.Contains(t, string(output), "HOME_TEST=/test")
}

// TestEnvs requests setting environment variables via
// a "envs@goteleport.com" request.
func TestEnvs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	f := newFixtureWithoutDiskBasedLogging(t)

	se, err := f.ssh.clt.NewSession(ctx)
	require.NoError(t, err)
	defer se.Close()

	envs := map[string]string{
		"HOME_TEST": "/test",
		"LLAMA":     "ALPACA",
		"FISH":      "FROG",
	}

	require.NoError(t, se.SetEnvs(ctx, envs))
	output, err := se.Output(ctx, "env")
	require.NoError(t, err)

	for k, v := range envs {
		require.Contains(t, string(output), k+"="+v)
	}
}

// TestUnknownRequest validates that any unknown session
// requests do not terminate the session.
func TestUnknownRequest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	f := newFixtureWithoutDiskBasedLogging(t)

	se, err := f.ssh.clt.NewSession(ctx)
	require.NoError(t, err)
	defer se.Close()

	// send a random request that won't be handled
	ok, err := se.SendRequest(ctx, uuid.NewString(), true, nil)
	require.NoError(t, err)
	require.False(t, ok)

	// ensure the session is still active
	require.NoError(t, se.Setenv(ctx, "HOME_TEST", "/test"))
	output, err := se.Output(ctx, "env")
	require.NoError(t, err)
	require.Contains(t, string(output), "HOME_TEST=/test")
}

// TestNoAuth tries to log in with no auth methods and should be rejected
func TestNoAuth(t *testing.T) {
	t.Parallel()
	f := newFixtureWithoutDiskBasedLogging(t)

	_, err := tracessh.Dial(context.Background(), "tcp", f.ssh.srv.Addr(), &ssh.ClientConfig{})
	require.Error(t, err)
}

// TestPasswordAuth tries to log in with empty pass and should be rejected
func TestPasswordAuth(t *testing.T) {
	t.Parallel()
	f := newFixtureWithoutDiskBasedLogging(t)
	config := &ssh.ClientConfig{
		Auth:            []ssh.AuthMethod{ssh.Password("")},
		HostKeyCallback: ssh.FixedHostKey(f.signer.PublicKey()),
	}
	_, err := tracessh.Dial(context.Background(), "tcp", f.ssh.srv.Addr(), config)
	require.Error(t, err)
}

func TestClientDisconnect(t *testing.T) {
	t.Parallel()
	f := newFixtureWithoutDiskBasedLogging(t)
	ctx := context.Background()

	config := &ssh.ClientConfig{
		User:            f.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(f.up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(f.signer.PublicKey()),
	}
	clt, err := tracessh.Dial(ctx, "tcp", f.ssh.srv.Addr(), config)
	require.NoError(t, err)
	require.NotNil(t, clt)

	se, err := f.ssh.clt.NewSession(ctx)
	require.NoError(t, err)
	require.NoError(t, se.Shell(ctx))
	require.NoError(t, clt.Close())
}

func TestLimiter(t *testing.T) {
	t.Parallel()
	f := newFixtureWithoutDiskBasedLogging(t)
	ctx := context.Background()

	getConns := func(t *testing.T, limiter *limiter.Limiter, token string, num int64) func() bool {
		return func() bool {
			connNumber, err := limiter.GetNumConnection(token)
			return err == nil && connNumber == num
		}
	}

	limiter, err := limiter.NewLimiter(
		limiter.Config{
			Clock:          f.clock,
			MaxConnections: 2,
			Rates: []limiter.Rate{
				{
					Period:  10 * time.Second,
					Average: 1,
					Burst:   3,
				},
				{
					Period:  40 * time.Millisecond,
					Average: 10,
					Burst:   30,
				},
			},
		},
	)
	require.NoError(t, err)

	nodeClient, _ := newNodeClient(t, f.testSrv)

	lockWatcher := newLockWatcher(ctx, t, nodeClient)

	sessionController, err := srv.NewSessionController(srv.SessionControllerConfig{
		Semaphores:   nodeClient,
		AccessPoint:  nodeClient,
		LockEnforcer: lockWatcher,
		Emitter:      nodeClient,
		Component:    teleport.ComponentNode,
		ServerID:     hostID,
	})
	require.NoError(t, err)

	nodeStateDir := t.TempDir()
	srv, err := New(
		ctx,
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		f.testSrv.ClusterName(),
		sshutils.StaticHostSigners(f.signer),
		nodeClient,
		nodeStateDir,
		"",
		utils.NetAddr{},
		nodeClient,
		SetLimiter(limiter),
		SetEmitter(nodeClient),
		SetNamespace(apidefaults.Namespace),
		SetPAMConfig(&servicecfg.PAMConfig{Enabled: false}),
		SetBPF(&bpf.NOP{}),
		SetClock(f.clock),
		SetLockWatcher(lockWatcher),
		SetSessionController(sessionController),
		SetConnectedProxyGetter(reversetunnel.NewConnectedProxyGetter()),
	)
	require.NoError(t, err)
	require.NoError(t, srv.Start())

	defer srv.Close()

	config := &ssh.ClientConfig{
		User:            f.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(f.up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(f.signer.PublicKey()),
	}

	clt0, err := tracessh.Dial(ctx, "tcp", srv.Addr(), config)
	require.NoError(t, err)
	require.NotNil(t, clt0)

	se0, err := clt0.NewSession(ctx)
	require.NoError(t, err)
	require.NoError(t, se0.Shell(ctx))

	// current connections = 1
	clt, err := tracessh.Dial(ctx, "tcp", srv.Addr(), config)
	require.NoError(t, err)
	require.NotNil(t, clt)
	se, err := clt.NewSession(ctx)
	require.NoError(t, err)
	require.NoError(t, se.Shell(ctx))

	// current connections = 2
	_, err = tracessh.Dial(ctx, "tcp", srv.Addr(), config)
	require.Error(t, err)

	require.NoError(t, se.Close())
	se.Wait()
	require.NoError(t, clt.Close())
	require.ErrorIs(t, clt.Wait(), net.ErrClosed)

	require.Eventually(t, getConns(t, limiter, "127.0.0.1", 1), time.Second*10, time.Millisecond*100)

	// current connections = 1
	clt, err = tracessh.Dial(ctx, "tcp", srv.Addr(), config)
	require.NoError(t, err)
	require.NotNil(t, clt)
	se, err = clt.NewSession(ctx)
	require.NoError(t, err)
	require.NoError(t, se.Shell(ctx))

	// current connections = 2
	_, err = tracessh.Dial(ctx, "tcp", srv.Addr(), config)
	require.Error(t, err)

	require.NoError(t, se.Close())
	se.Wait()
	require.NoError(t, clt.Close())
	require.ErrorIs(t, clt.Wait(), net.ErrClosed)

	require.Eventually(t, getConns(t, limiter, "127.0.0.1", 1), time.Second*10, time.Millisecond*100)

	// current connections = 1
	// requests rate should exceed now
	clt, err = tracessh.Dial(ctx, "tcp", srv.Addr(), config)
	require.NoError(t, err)
	require.NotNil(t, clt)
	_, err = clt.NewSession(ctx)
	require.Error(t, err)

	clt.Close()
}

// TestServerAliveInterval simulates ServerAliveInterval and OpenSSH
// interoperability by sending a keepalive@openssh.com global request to the
// server and expecting a response in return.
func TestServerAliveInterval(t *testing.T) {
	t.Parallel()
	f := newFixtureWithoutDiskBasedLogging(t)

	ok, _, err := f.ssh.clt.SendRequest(context.Background(), teleport.KeepAliveReqType, true, nil)
	require.NoError(t, err)
	require.True(t, ok)
}

// TestGlobalRequestClusterDetails simulates sending a global out-of-band
// cluster-details@goteleport.com request.
func TestGlobalRequestClusterDetails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cases := []struct {
		name     string
		fips     bool
		mode     string
		expected sshutils.ClusterDetails
	}{
		{
			name: "node recording and fips",
			fips: true,
			mode: types.RecordAtNode,
			expected: sshutils.ClusterDetails{
				RecordingProxy: false,
				FIPSEnabled:    true,
			},
		},
		{
			name: "node recording and not fips",
			fips: false,
			mode: types.RecordAtNode,
			expected: sshutils.ClusterDetails{
				RecordingProxy: false,
				FIPSEnabled:    false,
			},
		},
		{
			name: "proxy recording and fips",
			fips: true,
			mode: types.RecordAtProxy,
			expected: sshutils.ClusterDetails{
				RecordingProxy: true,
				FIPSEnabled:    true,
			},
		},
		{
			name: "proxy recording and not fips",
			fips: false,
			mode: types.RecordAtProxy,
			expected: sshutils.ClusterDetails{
				RecordingProxy: true,
				FIPSEnabled:    false,
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newCustomFixture(t, func(*auth.TestServerConfig) {}, SetFIPS(tt.fips))
			recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{Mode: tt.mode})
			require.NoError(t, err)

			_, err = f.testSrv.Auth().UpsertSessionRecordingConfig(ctx, recConfig)
			require.NoError(t, err)

			ok, responseBytes, err := f.ssh.clt.SendRequest(ctx, teleport.ClusterDetailsReqType, true, nil)
			require.NoError(t, err)
			require.True(t, ok)

			var details sshutils.ClusterDetails
			require.NoError(t, ssh.Unmarshal(responseBytes, &details))
			require.Empty(t, cmp.Diff(tt.expected, details))
		})
	}
}

// rawNode is a basic non-teleport node which holds a
// valid teleport cert and allows any client to connect.
// useful for simulating basic behaviors of openssh nodes.
type rawNode struct {
	listener net.Listener
	cfg      ssh.ServerConfig
	addr     string
	errCh    chan error
}

// accept an incoming connection and perform a basic ssh handshake
func (r *rawNode) accept() (*ssh.ServerConn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	netConn, err := r.listener.Accept()
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	srvConn, chs, reqs, err := ssh.NewServerConn(netConn, &r.cfg)
	if err != nil {
		netConn.Close()
		return nil, nil, nil, trace.Wrap(err)
	}
	return srvConn, chs, reqs, nil
}

func (r *rawNode) Close() error {
	return trace.Wrap(r.listener.Close())
}

// newRawNode constructs a new raw node instance.
func newRawNode(t *testing.T, authSrv *auth.Server) *rawNode {
	hostname, err := os.Hostname()
	require.NoError(t, err)

	priv, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	tlsPub, err := auth.PrivateKeyToPublicKeyTLS(priv)
	require.NoError(t, err)

	// Create host key and certificate for node.
	certs, err := authSrv.GenerateHostCerts(context.Background(),
		&proto.HostCertsRequest{
			HostID:               "raw-node",
			NodeName:             "raw-node",
			Role:                 types.RoleNode,
			AdditionalPrincipals: []string{hostname},
			DNSNames:             []string{hostname},
			PublicSSHKey:         pub,
			PublicTLSKey:         tlsPub,
		})
	require.NoError(t, err)

	signer, err := sshutils.NewSigner(priv, certs.SSH)
	require.NoError(t, err)

	// configure a server which allows any client to connect
	cfg := ssh.ServerConfig{
		NoClientAuth: true,
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			return &ssh.Permissions{}, nil
		},
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			return &ssh.Permissions{}, nil
		},
	}
	cfg.AddHostKey(signer)

	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)

	addr := net.JoinHostPort(hostname, port)

	return &rawNode{
		listener: listener,
		cfg:      cfg,
		addr:     addr,
		errCh:    make(chan error),
	}
}

// startX11EchoServer starts a fake node which, for each incoming SSH connection, accepts an
// X11 forwarding request and then dials a single X11 channel which echoes all bytes written
// to it.  Used to verify the behavior of X11 forwarding in recording proxies. Returns a
// node and an error channel that can be monitored for asynchronous failures.
// x11Handler handles requests received by the x11 echo server
func x11Handler(ctx context.Context, conn *ssh.ServerConn, chs <-chan ssh.NewChannel) error {
	// expect client to open a session channel
	var nch ssh.NewChannel
	select {
	case nch = <-chs:
	case <-time.After(time.Second * 3):
		return trace.LimitExceeded("Timeout waiting for session channel")
	case <-ctx.Done():
		return nil
	}

	if nch.ChannelType() != teleport.ChanSession {
		return trace.BadParameter("Unexpected channel type: %q", nch.ChannelType())
	}

	sch, creqs, err := nch.Accept()
	if err != nil {
		return trace.Wrap(err)
	}
	defer sch.Close()

	// expect client to send an X11 forwarding request
	var req *ssh.Request
	select {
	case req = <-creqs:
	case <-time.After(time.Second * 3):
		return trace.LimitExceeded("Timeout waiting for X11 forwarding request")
	case <-ctx.Done():
		return nil
	}

	if req.Type != x11.ForwardRequest {
		return trace.BadParameter("Unexpected request type %q", req.Type)
	}

	if err = req.Reply(true, nil); err != nil {
		return trace.Wrap(err)
	}

	// start a fake X11 channel
	xch, _, err := conn.OpenChannel(x11.ChannelRequest, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	defer xch.Close()

	// echo all bytes back across the X11 channel
	_, err = io.Copy(xch, xch)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(xch.CloseWrite())
}

// startX11EchoServer starts a fake node which, for each incoming SSH connection, accepts an
// X11 forwarding request and then dials a single X11 channel which echoes all bytes written
// to it. Used to verify the behavior of X11 forwarding in recording proxies. Returns a
// node and an error channel that can be monitored for asynchronous failures.
func startX11EchoServer(ctx context.Context, t *testing.T, authSrv *auth.Server) (*rawNode, <-chan error) {
	node := newRawNode(t, authSrv)
	errorCh := make(chan error, 1)
	go func() {
		for {
			conn, chs, _, err := node.accept()
			if err != nil {
				return
			}
			go func() {
				if err := x11Handler(ctx, conn, chs); err != nil {
					errorCh <- err
				}
				conn.Close()
			}()
		}
	}()

	return node, errorCh
}

// startGatheringErrors starts a goroutine that pulls error values from a
// channel and aggregates them. Returns a channel where the goroutine will post
// the aggregated errors when the routine is stopped.
func startGatheringErrors(ctx context.Context, errCh <-chan error) <-chan []error {
	doneGathering := make(chan []error)
	go func() {
		errors := []error{}
		for {
			select {
			case err := <-errCh:
				errors = append(errors, err)

			case <-ctx.Done():
				doneGathering <- errors
				return
			}
		}
	}()
	return doneGathering
}

// requireNoErrors waits for any aggregated errors to appear on the supplied channel
// and asserts that the aggregation is empty
func requireNoErrors(t *testing.T, errsCh <-chan []error) {
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	select {
	case errs := <-errsCh:
		require.Empty(t, errs)

	case <-timeoutCtx.Done():
		require.Fail(t, "Timed out waiting for errors")
	}
}

// TestParseSubsystemRequest verifies parseSubsystemRequest accepts the correct subsystems in depending on the runtime configuration.
func TestParseSubsystemRequest(t *testing.T) {
	ctx := context.Background()

	// start a listener to accept connections; this will be needed for the proxy test to pass, otherwise nothing will be there to handle the call.
	agentlessListener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = agentlessListener.Close() })
	go func() {
		for {
			// accept connections, but don't do anything else except for closing the connection on cleanup.
			conn, err := agentlessListener.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() {
				_ = conn.Close()
			})
		}
	}()

	agentlessSrv := types.ServerV2{
		Kind:    types.KindNode,
		SubKind: types.SubKindOpenSSHNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: uuid.NewString(),
		},
		Spec: types.ServerSpecV2{
			Addr:     agentlessListener.Addr().String(),
			Hostname: "agentless",
		},
	}

	getNonProxySession := func() func() *tracessh.Session {
		f := newFixtureWithoutDiskBasedLogging(t, SetAllowFileCopying(true))
		return func() *tracessh.Session {
			se, err := f.ssh.clt.NewSession(context.Background())
			require.NoError(t, err)
			t.Cleanup(func() { _ = se.Close() })
			return se
		}
	}()

	getProxySession := func() func() *tracessh.Session {
		f := newFixtureWithoutDiskBasedLogging(t)
		listener, _ := mustListen(t)

		proxyClient, _ := newProxyClient(t, f.testSrv)
		lockWatcher := newLockWatcher(ctx, t, proxyClient)
		nodeWatcher := newNodeWatcher(ctx, t, proxyClient)
		caWatcher := newCertAuthorityWatcher(ctx, t, proxyClient)

		reverseTunnelServer, err := reversetunnel.NewServer(reversetunnel.Config{
			GetClientTLSCertificate: func() (*tls.Certificate, error) {
				return &proxyClient.TLSConfig().Certificates[0], nil
			},
			ID:                    hostID,
			ClusterName:           f.testSrv.ClusterName(),
			Listener:              listener,
			GetHostSigners:        sshutils.StaticHostSigners(f.signer),
			LocalAuthClient:       proxyClient,
			LocalAccessPoint:      proxyClient,
			NewCachingAccessPoint: noCache,
			DataDir:               t.TempDir(),
			Emitter:               proxyClient,
			LockWatcher:           lockWatcher,
			NodeWatcher:           nodeWatcher,
			GitServerWatcher:      newGitServerWatcher(ctx, t, proxyClient),
			CertAuthorityWatcher:  caWatcher,
			EICESigner: func(ctx context.Context, target types.Server, integration types.Integration, login, token string, ap cryptosuites.AuthPreferenceGetter) (ssh.Signer, error) {
				return nil, errors.New("eice disabled in tests")
			},
			EICEDialer: func(ctx context.Context, target types.Server, integration types.Integration, token string) (net.Conn, error) {
				return nil, errors.New("eice disabled in tests")
			},
		})
		require.NoError(t, err)

		require.NoError(t, reverseTunnelServer.Start())
		t.Cleanup(func() { _ = reverseTunnelServer.Close() })

		nodeClient, _ := newNodeClient(t, f.testSrv)

		_, err = nodeClient.UpsertNode(ctx, &agentlessSrv)
		require.NoError(t, err)

		router, err := libproxy.NewRouter(libproxy.RouterConfig{
			ClusterName:      f.testSrv.ClusterName(),
			LocalAccessPoint: proxyClient,
			SiteGetter:       reverseTunnelServer,
			TracerProvider:   tracing.NoopProvider(),
		})
		require.NoError(t, err)

		sessionController, err := srv.NewSessionController(srv.SessionControllerConfig{
			Semaphores:   proxyClient,
			AccessPoint:  proxyClient,
			LockEnforcer: lockWatcher,
			Emitter:      proxyClient,
			Component:    teleport.ComponentProxy,
			ServerID:     hostID,
		})
		require.NoError(t, err)

		proxy, err := New(
			ctx,
			utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
			f.testSrv.ClusterName(),
			sshutils.StaticHostSigners(f.signer),
			proxyClient,
			t.TempDir(),
			"",
			utils.NetAddr{},
			proxyClient,
			SetProxyMode("", reverseTunnelServer, proxyClient, router),
			SetEmitter(nodeClient),
			SetNamespace(apidefaults.Namespace),
			SetPAMConfig(&servicecfg.PAMConfig{Enabled: false}),
			SetBPF(&bpf.NOP{}),
			SetClock(f.clock),
			SetLockWatcher(lockWatcher),
			SetSessionController(sessionController),
			SetConnectedProxyGetter(reversetunnel.NewConnectedProxyGetter()),
		)
		require.NoError(t, err)
		require.NoError(t, proxy.Start())
		t.Cleanup(func() { _ = proxy.Close() })

		// set up SSH client using the user private key for signing
		up, err := newUpack(f.testSrv, f.user, []string{f.user}, wildcardAllow)
		require.NoError(t, err)

		return func() *tracessh.Session {
			sshConfig := &ssh.ClientConfig{
				User:            f.user,
				Auth:            []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
				HostKeyCallback: ssh.FixedHostKey(f.signer.PublicKey()),
			}

			// Connect SSH client to proxy
			client, err := tracessh.Dial(ctx, "tcp", proxy.Addr(), sshConfig)

			require.NoError(t, err)
			t.Cleanup(func() { _ = client.Close() })

			se, err := client.NewSession(ctx)
			require.NoError(t, err)

			return se
		}
	}()

	tests := []struct {
		name                 string
		subsystemOverride    string
		wantErrInProxyMode   bool
		wantErrInRegularMode bool
	}{
		{
			name:                 "invalid",
			wantErrInProxyMode:   true,
			wantErrInRegularMode: true,
		},
		{
			name:                 teleport.SFTPSubsystem,
			wantErrInProxyMode:   true,
			wantErrInRegularMode: false,
		},
		{
			name:                 teleport.GetHomeDirSubsystem,
			wantErrInProxyMode:   true,
			wantErrInRegularMode: false,
		},
		{
			name:                 "proxysites",
			wantErrInProxyMode:   false,
			wantErrInRegularMode: true,
		},
		{
			name:                 "proxy:agentlessServer",
			subsystemOverride:    "proxy:" + agentlessSrv.Spec.Addr,
			wantErrInProxyMode:   false,
			wantErrInRegularMode: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subsystem := tt.name
			if tt.subsystemOverride != "" {
				subsystem = tt.subsystemOverride
			}

			err := getProxySession().RequestSubsystem(ctx, subsystem)
			if tt.wantErrInProxyMode {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			err = getNonProxySession().RequestSubsystem(ctx, subsystem)
			if tt.wantErrInRegularMode {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestX11ProxySupport verifies that recording proxies correctly forward
// X11 request/channels.
func TestX11ProxySupport(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	ctx := t.Context()

	// set cluster config to record at the proxy
	recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
		Mode: types.RecordAtProxy,
	})
	require.NoError(t, err)
	_, err = f.testSrv.Auth().UpsertSessionRecordingConfig(ctx, recConfig)
	require.NoError(t, err)

	// verify that the proxy is in recording mode
	ok, responseBytes, err := f.ssh.clt.SendRequest(ctx, teleport.ClusterDetailsReqType, true, nil)
	require.NoError(t, err)
	require.True(t, ok)

	var details sshutils.ClusterDetails
	require.NoError(t, ssh.Unmarshal(responseBytes, &details))
	require.NoError(t, err)
	require.True(t, details.RecordingProxy)

	// setup our fake X11 echo server.
	x11Ctx, x11Cancel := context.WithCancel(ctx)
	node, errCh := startX11EchoServer(x11Ctx, t, f.testSrv.Auth())

	// start gathering errors from the X11 server
	doneGathering := startGatheringErrors(x11Ctx, errCh)
	defer requireNoErrors(t, doneGathering)

	// The error gathering routine needs this context to expire or it will wait
	// forever on the X11 server to exit. Hence we defer a call to the x11cancel
	// here rather than directly below the context creation
	defer x11Cancel()

	// Create a direct TCP/IP connection from proxy to our X11 test server.
	netConn, err := f.ssh.clt.DialContext(x11Ctx, "tcp", node.addr)
	require.NoError(t, err)
	defer netConn.Close()

	// make an insecure version of our client config (this test is only about X11 forwarding,
	// so we don't bother to verify recording proxy key generation here).
	cltConfig := *f.ssh.cltConfig
	cltConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	// Perform ssh handshake and setup client for X11 test server.
	cltConn, chs, reqs, err := tracessh.NewClientConn(ctx, netConn, node.addr, &cltConfig)
	require.NoError(t, err)
	clt := tracessh.NewClient(cltConn, chs, reqs)

	sess, err := clt.NewSession(ctx)
	require.NoError(t, err)

	// register X11 channel handler before requesting forwarding to avoid races
	xchs := clt.HandleChannelOpen(x11.ChannelRequest)
	require.NotNil(t, xchs)

	// Send an X11 forwarding request to the server
	ok, err = sess.SendRequest(ctx, x11.ForwardRequest, true, nil)
	require.NoError(t, err)
	require.True(t, ok)

	// wait for server to start an X11 channel
	var xnc ssh.NewChannel
	select {
	case xnc = <-xchs:
	case <-time.After(time.Second * 3):
		require.Fail(t, "Timeout waiting for X11 channel open from %v", node.addr)
	}
	require.NotNil(t, xnc)
	require.Equal(t, x11.ChannelRequest, xnc.ChannelType())

	xch, _, err := xnc.Accept()
	require.NoError(t, err)

	defer xch.Close()

	// write some data to the channel
	msg := []byte("testing!")
	_, err = xch.Write(msg)
	require.NoError(t, err)

	// send EOF
	require.NoError(t, xch.CloseWrite())

	// expect node to successfully echo the data
	rsp := make([]byte, len(msg))
	_, err = io.ReadFull(xch, rsp)
	require.NoError(t, err)
	require.Equal(t, string(msg), string(rsp))
}

// TestIgnorePuTTYSimpleChannel verifies that any request from the PuTTY SSH client for
// its "simple" mode is ignored and connections remain open.
func TestIgnorePuTTYSimpleChannel(t *testing.T) {
	t.Parallel()

	f := newFixtureWithoutDiskBasedLogging(t)
	ctx := context.Background()

	listener, _ := mustListen(t)
	proxyClient, _ := newProxyClient(t, f.testSrv)
	lockWatcher := newLockWatcher(ctx, t, proxyClient)
	nodeWatcher := newNodeWatcher(ctx, t, proxyClient)
	caWatcher := newCertAuthorityWatcher(ctx, t, proxyClient)

	reverseTunnelServer, err := reversetunnel.NewServer(reversetunnel.Config{
		GetClientTLSCertificate: func() (*tls.Certificate, error) {
			return &proxyClient.TLSConfig().Certificates[0], nil
		},
		ID:                    hostID,
		ClusterName:           f.testSrv.ClusterName(),
		Listener:              listener,
		GetHostSigners:        sshutils.StaticHostSigners(f.signer),
		LocalAuthClient:       proxyClient,
		LocalAccessPoint:      proxyClient,
		NewCachingAccessPoint: noCache,
		DataDir:               t.TempDir(),
		Emitter:               proxyClient,
		LockWatcher:           lockWatcher,
		NodeWatcher:           nodeWatcher,
		GitServerWatcher:      newGitServerWatcher(ctx, t, proxyClient),
		CertAuthorityWatcher:  caWatcher,
		EICESigner: func(ctx context.Context, target types.Server, integration types.Integration, login, token string, ap cryptosuites.AuthPreferenceGetter) (ssh.Signer, error) {
			return nil, errors.New("eice disabled in tests")
		},
		EICEDialer: func(ctx context.Context, target types.Server, integration types.Integration, token string) (net.Conn, error) {
			return nil, errors.New("eice disabled in tests")
		},
	})
	require.NoError(t, err)

	require.NoError(t, reverseTunnelServer.Start())
	defer reverseTunnelServer.Close()

	nodeClient, _ := newNodeClient(t, f.testSrv)

	router, err := libproxy.NewRouter(libproxy.RouterConfig{
		ClusterName:      f.testSrv.ClusterName(),
		LocalAccessPoint: proxyClient,
		SiteGetter:       reverseTunnelServer,
		TracerProvider:   tracing.NoopProvider(),
	})
	require.NoError(t, err)

	sessionController, err := srv.NewSessionController(srv.SessionControllerConfig{
		Semaphores:   proxyClient,
		AccessPoint:  proxyClient,
		LockEnforcer: lockWatcher,
		Emitter:      proxyClient,
		Component:    teleport.ComponentProxy,
		ServerID:     hostID,
	})
	require.NoError(t, err)

	proxy, err := New(
		ctx,
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		f.testSrv.ClusterName(),
		sshutils.StaticHostSigners(f.signer),
		proxyClient,
		t.TempDir(),
		"",
		utils.NetAddr{},
		proxyClient,
		SetProxyMode("", reverseTunnelServer, proxyClient, router),
		SetEmitter(nodeClient),
		SetNamespace(apidefaults.Namespace),
		SetPAMConfig(&servicecfg.PAMConfig{Enabled: false}),
		SetBPF(&bpf.NOP{}),
		SetClock(f.clock),
		SetLockWatcher(lockWatcher),
		SetSessionController(sessionController),
		SetConnectedProxyGetter(reversetunnel.NewConnectedProxyGetter()),
	)
	require.NoError(t, err)
	require.NoError(t, proxy.Start())
	defer proxy.Close()

	// set up SSH client using the user private key for signing
	up, err := newUpack(f.testSrv, f.user, []string{f.user}, wildcardAllow)
	require.NoError(t, err)

	sshConfig := &ssh.ClientConfig{
		User:            f.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(f.signer.PublicKey()),
	}

	_, err = newUpack(f.testSrv, "user1", []string{f.user}, wildcardAllow)
	require.NoError(t, err)

	// Connect SSH client to proxy
	client, err := tracessh.Dial(ctx, "tcp", proxy.Addr(), sshConfig)

	require.NoError(t, err)
	defer client.Close()

	se, err := client.NewSession(ctx)
	require.NoError(t, err)
	defer se.Close()

	writer, err := se.StdinPipe()
	require.NoError(t, err)

	reader, err := se.StdoutPipe()
	require.NoError(t, err)

	// Request the PuTTY-specific "simple@putty.projects.tartarus.org" channel type
	// This request should be ignored, and the connection should remain open.
	_, err = se.SendRequest(ctx, sshutils.PuTTYSimpleRequest, false, []byte{})
	require.NoError(t, err)

	// Request proxy subsystem routing TCP connection to the remote host
	require.NoError(t, se.RequestSubsystem(ctx, fmt.Sprintf("proxy:%v", f.ssh.srvAddress)))

	local, err := utils.ParseAddr("tcp://" + proxy.Addr())
	require.NoError(t, err)
	remote, err := utils.ParseAddr("tcp://" + f.ssh.srv.Addr())
	require.NoError(t, err)

	pipeNetConn := utils.NewPipeNetConn(
		reader,
		writer,
		se,
		local,
		remote,
	)

	defer pipeNetConn.Close()

	// Open SSH connection via proxy subsystem's TCP tunnel
	conn, chans, reqs, err := tracessh.NewClientConn(ctx, pipeNetConn,
		f.ssh.srv.Addr(), sshConfig)
	require.NoError(t, err)
	defer conn.Close()

	// Run commands over this connection like regular SSH
	client2 := tracessh.NewClient(conn, chans, reqs)
	require.NoError(t, err)
	defer client2.Close()

	se2, err := client2.NewSession(ctx)
	require.NoError(t, err)
	defer se2.Close()

	out, err := se2.Output(ctx, "echo hello again")
	require.NoError(t, err)

	require.Equal(t, "hello again\n", string(out))
}

// TestHandlePuTTYWinadj verifies that any request from the PuTTY SSH client for its "winadj"
// channel is correctly responded to with a failure message and connections remain open.
func TestHandlePuTTYWinadj(t *testing.T) {
	t.Parallel()
	f := newFixtureWithoutDiskBasedLogging(t)
	ctx := context.Background()

	se, err := f.ssh.clt.NewSession(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { se.Close() })

	// send a PuTTY winadj request to the server. it shouldn't error, but the response
	// should be a failure.
	ok, err := se.SendRequest(ctx, sshutils.PuTTYWinadjRequest, true, nil)
	require.NoError(t, err)
	require.False(t, ok)

	// echo something to make sure the connection is still alive following the request
	out, err := se.Output(ctx, "echo hello once more")
	require.NoError(t, err)
	require.Equal(t, "hello once more\n", string(out))
}

func TestTargetMetadata(t *testing.T) {
	ctx := context.Background()
	testServer, err := auth.NewTestServer(auth.TestServerConfig{
		Auth: auth.TestAuthServerConfig{
			ClusterName: "localhost",
			Dir:         t.TempDir(),
			Clock:       clockwork.NewFakeClock(),
		},
	})
	require.NoError(t, err)

	nodeID := uuid.New().String()
	nodeClient, err := testServer.NewClient(auth.TestIdentity{
		I: authz.BuiltinRole{
			Role:     types.RoleNode,
			Username: nodeID,
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, nodeClient.Close()) })

	lockWatcher := newLockWatcher(ctx, t, nodeClient)

	sessionController, err := srv.NewSessionController(srv.SessionControllerConfig{
		Semaphores:   nodeClient,
		AccessPoint:  nodeClient,
		LockEnforcer: lockWatcher,
		Emitter:      nodeClient,
		Component:    teleport.ComponentNode,
		ServerID:     nodeID,
	})
	require.NoError(t, err)

	nodeDir := t.TempDir()
	serverOptions := []ServerOption{
		SetUUID(nodeID),
		SetNamespace(apidefaults.Namespace),
		SetEmitter(nodeClient),
		SetPAMConfig(&servicecfg.PAMConfig{Enabled: false}),
		SetLabels(
			map[string]string{"foo": "bar"},
			services.CommandLabels{
				"baz": &types.CommandLabelV2{
					Period:  types.NewDuration(time.Second),
					Command: []string{"expr", "1", "+", "3"},
				},
			}, nil,
		),
		SetBPF(&bpf.NOP{}),
		SetLockWatcher(lockWatcher),
		SetX11ForwardingConfig(&x11.ServerConfig{}),
		SetSessionController(sessionController),
		SetConnectedProxyGetter(reversetunnel.NewConnectedProxyGetter()),
	}

	sshSrv, err := New(
		ctx,
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		testServer.ClusterName(),
		sshutils.StaticHostSigners(newSigner(t, ctx, testServer)),
		nodeClient,
		nodeDir,
		"",
		utils.NetAddr{},
		nodeClient,
		serverOptions...)
	require.NoError(t, err)

	metadata := sshSrv.TargetMetadata()
	require.Equal(t, nodeID, metadata.ServerID)
	require.Equal(t, apidefaults.Namespace, metadata.ServerNamespace)
	require.Empty(t, metadata.ServerAddr)
	require.Equal(t, "localhost", metadata.ServerHostname)

	require.Contains(t, metadata.ServerLabels, "foo")
	require.Contains(t, metadata.ServerLabels, "baz")
}

// upack holds all ssh signing artifacts needed for signing and checking user keys
type upack struct {
	// key is a raw private user key
	key []byte

	// pkey is parsed private SSH key
	pkey any

	// pub is a public user key
	pub []byte

	// cert is a certificate signed by user CA
	cert []byte
	// pcert is a parsed ssh Certificae
	pcert *ssh.Certificate

	// signer is a signer that answers signing challenges using private key
	signer ssh.Signer

	// certSigner is a signer that answers signing challenges using private
	// key and a certificate issued by user certificate authority
	certSigner ssh.Signer
}

func newUpack(testSvr *auth.TestServer, username string, allowedLogins []string, allowedLabels types.Labels) (*upack, error) {
	ctx := context.Background()
	auth := testSvr.Auth()
	upriv, upub, err := testauthority.New().GenerateKeyPair()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := types.NewUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	role := services.RoleForUser(user)
	rules := role.GetRules(types.Allow)
	rules = append(rules, types.NewRule(types.Wildcard, services.RW()))
	role.SetRules(types.Allow, rules)
	opts := role.GetOptions()
	opts.PermitX11Forwarding = types.NewBool(true)
	//nolint:staticcheck // this field is preserved for existing deployments, but shouldn't be used going forward
	opts.CreateHostUser = types.NewBoolOption(true)
	role.SetOptions(opts)
	role.SetLogins(types.Allow, allowedLogins)
	role.SetNodeLabels(types.Allow, allowedLabels)
	_, err = auth.UpsertRole(ctx, role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.AddRole(role.GetName())
	user, err = auth.UpsertUser(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ucert, err := testSvr.AuthServer.GenerateUserCert(upub, user.GetName(), 5*time.Minute, constants.CertificateFormatStandard)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	upkey, err := ssh.ParseRawPrivateKey(upriv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	usigner, err := ssh.NewSignerFromKey(upkey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pcert, _, _, _, err := ssh.ParseAuthorizedKey(ucert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ucertSigner, err := ssh.NewCertSigner(pcert.(*ssh.Certificate), usigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &upack{
		key:        upriv,
		pkey:       upkey,
		pub:        upub,
		cert:       ucert,
		pcert:      pcert.(*ssh.Certificate),
		signer:     usigner,
		certSigner: ucertSigner,
	}, nil
}

func newLockWatcher(ctx context.Context, t testing.TB, client types.Events) *services.LockWatcher {
	lockWatcher, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: "test",
			Client:    client,
		},
	})
	require.NoError(t, err)
	t.Cleanup(lockWatcher.Close)
	return lockWatcher
}

func newNodeWatcher(ctx context.Context, t *testing.T, client *authclient.Client) *services.GenericWatcher[types.Server, readonly.Server] {
	nodeWatcher, err := services.NewNodeWatcher(ctx, services.NodeWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: "test",
			Client:    client,
		},
		NodesGetter: client,
	})
	require.NoError(t, err)
	t.Cleanup(nodeWatcher.Close)
	return nodeWatcher
}

func newGitServerWatcher(ctx context.Context, t *testing.T, client *authclient.Client) *services.GenericWatcher[types.Server, readonly.Server] {
	watcher, err := services.NewGitServerWatcher(ctx, services.GitServerWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: "test",
			Client:    client,
		},
		GitServerGetter: client.GitServerReadOnlyClient(),
	})
	require.NoError(t, err)
	t.Cleanup(watcher.Close)
	return watcher
}

func newCertAuthorityWatcher(ctx context.Context, t *testing.T, client types.Events) *services.CertAuthorityWatcher {
	caWatcher, err := services.NewCertAuthorityWatcher(ctx, services.CertAuthorityWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: "test",
			Client:    client,
		},
		Types: []types.CertAuthType{types.HostCA, types.UserCA},
	})
	require.NoError(t, err)
	t.Cleanup(caWatcher.Close)
	return caWatcher
}

// newSigner creates a new SSH signer that can be used by the Server.
func newSigner(t testing.TB, ctx context.Context, testServer *auth.TestServer) ssh.Signer {
	t.Helper()

	priv, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	tlsPub, err := auth.PrivateKeyToPublicKeyTLS(priv)
	require.NoError(t, err)

	certs, err := testServer.Auth().GenerateHostCerts(ctx,
		&proto.HostCertsRequest{
			HostID:       hostID,
			NodeName:     testServer.ClusterName(),
			Role:         types.RoleNode,
			PublicSSHKey: pub,
			PublicTLSKey: tlsPub,
		})
	require.NoError(t, err)

	// set up user CA and set up a user that has access to the server
	signer, err := sshutils.NewSigner(priv, certs.SSH)
	require.NoError(t, err)
	return signer
}

// maxPipeSize is one larger than the maximum pipe size for most operating
// systems which appears to be 65536 bytes.
//
// The maximum pipe size for Linux could potentially be obtained, however
// getting it for macOS is much harder, and unclear if even possible. Therefor
// just hard code it.
//
// See the following links for more details.
//
//	https://man7.org/linux/man-pages/man7/pipe.7.html
//	https://github.com/afborchert/pipebuf
//	https://unix.stackexchange.com/questions/11946/how-big-is-the-pipe-buffer
const maxPipeSize = 65536 + 1

func TestHostUserCreationProxy(t *testing.T) {
	f := newFixtureWithoutDiskBasedLogging(t)
	ctx := context.Background()

	proxyClient, _ := newProxyClient(t, f.testSrv)
	nodeClient, _ := newNodeClient(t, f.testSrv)

	listener, _ := mustListen(t)
	defer listener.Close()
	lockWatcher := newLockWatcher(ctx, t, proxyClient)
	nodeWatcher := newNodeWatcher(ctx, t, proxyClient)
	caWatcher := newCertAuthorityWatcher(ctx, t, proxyClient)

	reverseTunnelServer, err := reversetunnel.NewServer(reversetunnel.Config{
		GetClientTLSCertificate: func() (*tls.Certificate, error) {
			return &proxyClient.TLSConfig().Certificates[0], nil
		},
		ClusterName:           f.testSrv.ClusterName(),
		ID:                    hostID,
		Listener:              listener,
		GetHostSigners:        sshutils.StaticHostSigners(f.signer),
		LocalAuthClient:       proxyClient,
		LocalAccessPoint:      proxyClient,
		NewCachingAccessPoint: noCache,
		DataDir:               t.TempDir(),
		Emitter:               proxyClient,
		LockWatcher:           lockWatcher,
		NodeWatcher:           nodeWatcher,
		GitServerWatcher:      newGitServerWatcher(ctx, t, proxyClient),
		CertAuthorityWatcher:  caWatcher,
		CircuitBreakerConfig:  breaker.NoopBreakerConfig(),
		EICESigner: func(ctx context.Context, target types.Server, integration types.Integration, login, token string, ap cryptosuites.AuthPreferenceGetter) (ssh.Signer, error) {
			return nil, errors.New("eice disabled in tests")
		},
		EICEDialer: func(ctx context.Context, target types.Server, integration types.Integration, token string) (net.Conn, error) {
			return nil, errors.New("eice disabled in tests")
		},
	})
	require.NoError(t, err)

	require.NoError(t, reverseTunnelServer.Start())
	defer reverseTunnelServer.Close()

	router, err := libproxy.NewRouter(libproxy.RouterConfig{
		ClusterName:      f.testSrv.ClusterName(),
		LocalAccessPoint: proxyClient,
		SiteGetter:       reverseTunnelServer,
		TracerProvider:   tracing.NoopProvider(),
	})
	require.NoError(t, err)

	sessionController, err := srv.NewSessionController(srv.SessionControllerConfig{
		Semaphores:   proxyClient,
		AccessPoint:  proxyClient,
		LockEnforcer: lockWatcher,
		Emitter:      proxyClient,
		Component:    teleport.ComponentNode,
		ServerID:     hostID,
	})
	require.NoError(t, err)

	proxy, err := New(
		ctx,
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		f.testSrv.ClusterName(),
		sshutils.StaticHostSigners(f.signer),
		proxyClient,
		t.TempDir(),
		"",
		utils.NetAddr{},
		proxyClient,
		SetProxyMode("", reverseTunnelServer, proxyClient, router),
		SetEmitter(nodeClient),
		SetNamespace(apidefaults.Namespace),
		SetPAMConfig(&servicecfg.PAMConfig{Enabled: false}),
		SetBPF(&bpf.NOP{}),
		SetClock(f.clock),
		SetLockWatcher(lockWatcher),
		SetSessionController(sessionController),
		SetConnectedProxyGetter(reversetunnel.NewConnectedProxyGetter()),
	)
	require.NoError(t, err)

	sudoers := &fakeHostSudoers{}
	proxy.sudoers = sudoers

	usersBackend := &fakeHostUsersBackend{}
	proxy.users = usersBackend

	// Explicitly enabled host user creation on the proxy, even though this
	// should never happen, to test that the proxy will not honor this setting.
	proxy.createHostUser = true
	proxy.proxyMode = true

	reg, err := srv.NewSessionRegistry(srv.SessionRegistryConfig{Srv: proxy, SessionTrackerService: proxyClient})
	require.NoError(t, err)

	_, err = reg.WriteSudoersFile(srv.IdentityContext{
		AccessPermit: &decisionpb.SSHAccessPermit{
			HostSudoers: []string{"test1", "test2", "test3"},
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, 0, sudoers.writeAttempts)

	_, _, err = reg.UpsertHostUser(srv.IdentityContext{
		AccessPermit: &decisionpb.SSHAccessPermit{
			HostSudoers: []string{"test1", "test2", "test3"},
		},
	}, srv.ObtainFallbackUIDFunc(nil))
	assert.NoError(t, err)
	assert.Empty(t, usersBackend.calls, 0)
}

func TestObtainFallbackUID(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	type testCase struct {
		config     *types.StableUNIXUserConfig
		obtainFunc func(username string) (int32, error)
		username   string
		required   func(uid int32, ok bool, err error)
	}

	for _, tc := range []testCase{{
		config: nil,
		obtainFunc: func(username string) (int32, error) {
			require.FailNow(t, "called obtainFunc")
			panic("unreachable")
		},
		username: "",
		required: func(uid int32, ok bool, err error) {
			require.NoError(t, err)
			require.False(t, ok)
		},
	}, {
		config: &types.StableUNIXUserConfig{
			Enabled: false,
		},
		obtainFunc: func(username string) (int32, error) {
			require.FailNow(t, "called obtainFunc")
			panic("unreachable")
		},
		username: "",
		required: func(uid int32, ok bool, err error) {
			require.NoError(t, err)
			require.False(t, ok)
		},
	}, {
		config: &types.StableUNIXUserConfig{
			Enabled: true,
		},
		obtainFunc: func(username string) (int32, error) {
			require.Equal(t, "foo", username)
			return 325872, nil
		},
		username: "foo",
		required: func(uid int32, ok bool, err error) {
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, int32(325872), uid)
		},
	}, {
		config: &types.StableUNIXUserConfig{
			Enabled: true,
		},
		obtainFunc: func(username string) (int32, error) {
			require.Equal(t, "bar", username)
			return 0, trace.LimitExceeded("no UIDs or something")
		},
		username: "bar",
		required: func(uid int32, ok bool, err error) {
			require.ErrorIs(t, err, &trace.LimitExceededError{Message: "no UIDs or something"})
		},
	}, {
		config: &types.StableUNIXUserConfig{
			Enabled: true,
		},
		obtainFunc: func(username string) (int32, error) {
			require.Equal(t, "baz", username)
			return 0, trace.BadParameter("some bug, idk")
		},
		username: "baz",
		required: func(uid int32, ok bool, err error) {
			require.ErrorIs(t, err, &trace.BadParameterError{Message: "some bug, idk"})
		},
	}} {
		srv := &Server{
			authService: getAuthPreferenceAccessPoint{
				authPreference: &types.AuthPreferenceV2{Spec: types.AuthPreferenceSpecV2{
					StableUnixUserConfig: tc.config,
				}},
			},
			stableUnixUsers: obtainUIDForUsernameFunc(tc.obtainFunc),
		}
		tc.required(srv.obtainFallbackUID(ctx, tc.username))
	}
}

type getAuthPreferenceAccessPoint struct {
	srv.AccessPoint
	authPreference types.AuthPreference
}

func (a getAuthPreferenceAccessPoint) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	return a.authPreference, nil
}

type obtainUIDForUsernameFunc func(username string) (int32, error)

func (f obtainUIDForUsernameFunc) ObtainUIDForUsername(ctx context.Context, in *stableunixusersv1.ObtainUIDForUsernameRequest, opts ...grpc.CallOption) (*stableunixusersv1.ObtainUIDForUsernameResponse, error) {
	uid, err := f(in.GetUsername())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &stableunixusersv1.ObtainUIDForUsernameResponse{Uid: uid}, nil
}

func (f obtainUIDForUsernameFunc) ListStableUNIXUsers(ctx context.Context, in *stableunixusersv1.ListStableUNIXUsersRequest, opts ...grpc.CallOption) (*stableunixusersv1.ListStableUNIXUsersResponse, error) {
	return nil, trace.NotImplemented("ListStableUNIXUsers")
}

type fakeHostSudoers struct {
	writeAttempts int
}

func (f *fakeHostSudoers) WriteSudoers(name string, sudoers []string) error {
	f.writeAttempts++
	return nil
}

func (f *fakeHostSudoers) RemoveSudoers(name string) error {
	return nil
}

type fakeHostUsersBackend struct {
	srv.HostUsers

	calls map[string]int
}

func (f *fakeHostUsersBackend) functionCalled(name string) {
	if f.calls == nil {
		f.calls = make(map[string]int)
	}

	f.calls[name]++
}

func (f *fakeHostUsersBackend) UpsertUser(name string, hostRoleInfo *decisionpb.HostUsersInfo, opts ...srv.UpsertHostUserOption) (io.Closer, error) {
	f.functionCalled("UpsertUser")
	return nil, trace.NotImplemented("")
}

func (f *fakeHostUsersBackend) DeleteUser(name, gid string) error {
	f.functionCalled("DeleteUser")
	return trace.NotImplemented("")
}

func (f *fakeHostUsersBackend) DeleteAllUsers() error {
	f.functionCalled("DeleteAllUsers")
	return trace.NotImplemented("")
}

func (f *fakeHostUsersBackend) UserCleanup() {
	f.functionCalled("UserCleanup")
}

func (f *fakeHostUsersBackend) Shutdown() {
	f.functionCalled("ShutDown")
}

func (f *fakeHostUsersBackend) UserExists(name string) error {
	f.functionCalled("UserExists")
	return trace.NotImplemented("")
}

func (f *fakeHostUsersBackend) SetHostUserDeletionGrace(grace time.Duration) {
	f.functionCalled("SetHostUserDeletionGrace")
}
