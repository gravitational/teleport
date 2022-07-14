/*
Copyright 2015-2020 Gravitational, Inc.

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

package regular

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/mailgun/timetools"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/pam"
	restricted "github.com/gravitational/teleport/lib/restrictedsession"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	sess "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/sshutils/x11"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
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
	clt            *ssh.Client
	cltConfig      *ssh.ClientConfig
	assertCltClose require.ErrorAssertionFunc
}

type sshTestFixture struct {
	ssh     sshInfo
	up      *upack
	signer  ssh.Signer
	user    string
	clock   clockwork.FakeClock
	testSrv *auth.TestServer
}

func newFixture(t *testing.T) *sshTestFixture {
	return newCustomFixture(t, func(*auth.TestServerConfig) {})
}

func newFixtureWithoutDiskBasedLogging(t *testing.T) *sshTestFixture {
	t.Helper()

	f := newCustomFixture(t, func(cfg *auth.TestServerConfig) {
		cfg.Auth.AuditLog = events.NewDiscardAuditLog()
	})

	// use a sync recording mode because the disk-based uploader
	// that runs in the background introduces races with test cleanup
	recConfig := types.DefaultSessionRecordingConfig()
	recConfig.SetMode(types.RecordAtNodeSync)
	err := f.testSrv.Auth().SetSessionRecordingConfig(context.Background(), recConfig)
	require.NoError(t, err)

	return f
}

func newCustomFixture(t *testing.T, mutateCfg func(*auth.TestServerConfig), sshOpts ...ServerOption) *sshTestFixture {
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

	priv, pub, err := native.GenerateKeyPair()
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

	nodeID := uuid.New().String()
	nodeClient, err := testServer.NewClient(auth.TestIdentity{
		I: auth.BuiltinRole{
			Role:     types.RoleNode,
			Username: nodeID,
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, nodeClient.Close()) })

	nodeDir := t.TempDir()
	serverOptions := []ServerOption{
		SetUUID(nodeID),
		SetNamespace(apidefaults.Namespace),
		SetEmitter(nodeClient),
		SetShell("/bin/sh"),
		SetSessionServer(nodeClient),
		SetPAMConfig(&pam.Config{Enabled: false}),
		SetLabels(
			map[string]string{"foo": "bar"},
			services.CommandLabels{
				"baz": &types.CommandLabelV2{
					Period:  types.NewDuration(time.Millisecond),
					Command: []string{"expr", "1", "+", "3"},
				},
			}, nil,
		),
		SetBPF(&bpf.NOP{}),
		SetRestrictedSessionManager(&restricted.NOP{}),
		SetClock(clock),
		SetLockWatcher(newLockWatcher(ctx, t, nodeClient)),
		SetX11ForwardingConfig(&x11.ServerConfig{}),
	}

	serverOptions = append(serverOptions, sshOpts...)

	sshSrv, err := New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		testServer.ClusterName(),
		[]ssh.Signer{signer},
		nodeClient,
		nodeDir,
		"",
		utils.NetAddr{},
		nodeClient,
		serverOptions...)
	require.NoError(t, err)
	require.NoError(t, auth.CreateUploaderDir(nodeDir))
	require.NoError(t, sshSrv.Start())
	t.Cleanup(func() {
		require.NoError(t, sshSrv.Close())
		sshSrv.Wait()
	})

	require.NoError(t, sshSrv.heartbeat.ForceSend(time.Second))

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

	client, err := ssh.Dial("tcp", sshSrv.Addr(), cltConfig)
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
	require.NoError(t, agent.ForwardToAgent(client, keyring))
	return f
}

func newProxyClient(t *testing.T, testSvr *auth.TestServer) (*auth.Client, string) {
	// create proxy client used in some tests
	proxyID := uuid.New().String()
	proxyClient, err := testSvr.NewClient(auth.TestIdentity{
		I: auth.BuiltinRole{
			Role:     types.RoleProxy,
			Username: proxyID,
		},
	})
	require.NoError(t, err)
	return proxyClient, proxyID
}

func newNodeClient(t *testing.T, testSvr *auth.TestServer) (*auth.Client, string) {
	nodeID := uuid.New().String()
	nodeClient, err := testSvr.NewClient(auth.TestIdentity{
		I: auth.BuiltinRole{
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
	f := newCustomFixture(t, mutateCfg)

	// If all goes well, the client will be closed by the time cleanup happens,
	// so change the assertion on closing the client to expect it to fail
	f.ssh.assertCltClose = require.Error

	se, err := f.ssh.clt.NewSession()
	require.NoError(t, err)
	defer se.Close()

	stderr, err := se.StderrPipe()
	require.NoError(t, err)
	stdErrCh := startReadAll(stderr)

	endCh := make(chan error)
	go func() { endCh <- f.ssh.clt.Wait() }()

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

func TestLockInForce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newFixture(t)

	// If all goes well, the client will be closed by the time cleanup happens,
	// so change the assertion on closing the client to expect it to fail.
	f.ssh.assertCltClose = require.Error

	se, err := f.ssh.clt.NewSession()
	require.NoError(t, err)

	stderr, err := se.StderrPipe()
	require.NoError(t, err)
	stdErrCh := startReadAll(stderr)

	endCh := make(chan error)
	go func() { endCh <- f.ssh.clt.Wait() }()

	lock, err := types.NewLock("test-lock", types.LockSpecV2{
		Target: types.LockTarget{Login: f.user},
	})
	require.NoError(t, err)
	require.NoError(t, f.testSrv.Auth().UpsertLock(ctx, lock))

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
	require.Eventually(t, sessionHasFinished, 1*time.Second, 100*time.Millisecond,
		"Timed out waiting for session to finish")

	// Expect the lock-in-force message to have been delivered via stderr.
	lockInForceMsg := services.LockInForceAccessDenied(lock).Error()
	text, err := waitForBytes(stdErrCh)
	require.NoError(t, err)
	require.Equal(t, lockInForceMsg, string(text))

	// As long as the lock is in force, new sessions cannot be opened.
	newClient, err := ssh.Dial("tcp", f.ssh.srvAddress, f.ssh.cltConfig)
	require.NoError(t, err)
	t.Cleanup(func() {
		// The client is expected to be closed by the lock monitor therefore expect
		// an error on this second attempt.
		require.Error(t, newClient.Close())
	})
	_, err = newClient.NewSession()
	require.Error(t, err)
	require.Contains(t, err.Error(), lockInForceMsg)

	// Once the lock is lifted, new sessions should go through without error.
	require.NoError(t, f.testSrv.Auth().DeleteLock(ctx, "test-lock"))
	newClient2, err := ssh.Dial("tcp", f.ssh.srvAddress, f.ssh.cltConfig)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, newClient2.Close()) })
	_, err = newClient2.NewSession()
	require.NoError(t, err)
}

// TestDirectTCPIP ensures that the server can create a "direct-tcpip"
// channel to the target address. The "direct-tcpip" channel is what port
// forwarding is built upon.
func TestDirectTCPIP(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	// Startup a test server that will reply with "hello, world\n"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello, world")
	}))
	defer ts.Close()

	// Extract the host:port the test HTTP server is running on.
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)

	// Build a http.Client that will dial through the server to establish the
	// connection. That's why a custom dialer is used and the dialer uses
	// s.clt.Dial (which performs the "direct-tcpip" request).
	httpClient := http.Client{
		Transport: &http.Transport{
			Dial: func(network string, addr string) (net.Conn, error) {
				return f.ssh.clt.Dial("tcp", u.Host)
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
}

func TestAdvertiseAddr(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

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
	require.NoError(t, f.testSrv.Auth().UpsertRole(ctx, role))

	se, err := f.ssh.clt.NewSession()
	require.NoError(t, err)
	t.Cleanup(func() { se.Close() })

	// to interoperate with OpenSSH, requests for agent forwarding always succeed.
	// however that does not mean the users agent will actually be forwarded.
	require.NoError(t, agent.RequestAgentForwarding(se))

	// the output of env, we should not see SSH_AUTH_SOCK in the output
	output, err := se.Output("env")
	require.NoError(t, err)
	require.NotContains(t, string(output), "SSH_AUTH_SOCK")
}

// TestMaxSesssions makes sure that MaxSessions RBAC rules prevent
// too many concurrent sessions.
func TestMaxSessions(t *testing.T) {
	t.Parallel()

	const maxSessions int64 = 2
	f := newFixture(t)
	ctx := context.Background()
	// make sure the role does not allow agent forwarding
	roleName := services.RoleNameForUser(f.user)
	role, err := f.testSrv.Auth().GetRole(ctx, roleName)
	require.NoError(t, err)
	roleOptions := role.GetOptions()
	roleOptions.MaxSessions = maxSessions
	role.SetOptions(roleOptions)
	err = f.testSrv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	for i := int64(0); i < maxSessions; i++ {
		se, err := f.ssh.clt.NewSession()
		require.NoError(t, err)
		defer se.Close()
	}

	_, err = f.ssh.clt.NewSession()
	require.Error(t, err)
	require.Contains(t, err.Error(), "too many session channels")

	// verify that max sessions does not affect max connections.
	for i := int64(0); i <= maxSessions; i++ {
		clt, err := ssh.Dial("tcp", f.ssh.srv.Addr(), f.ssh.cltConfig)
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
	f := newFixture(t)

	// Get the path to where the "echo" command is on disk.
	echoPath, err := exec.LookPath("echo")
	require.NoError(t, err)

	se, err := f.ssh.clt.NewSession()
	require.NoError(t, err)
	defer se.Close()

	// Write a message that larger than the maximum pipe size.
	_, err = se.Output(fmt.Sprintf("%v %v", echoPath, strings.Repeat("a", maxPipeSize)))
	require.NoError(t, err)
}

// TestOpenExecSessionSetsSession tests that OpenExecSession()
// sets ServerContext session.
func TestOpenExecSessionSetsSession(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	se, err := f.ssh.clt.NewSession()
	require.NoError(t, err)
	defer se.Close()

	// This will trigger an exec request, which will start a non-interactive session,
	// which then triggers setting env for SSH_SESSION_ID.
	output, err := se.Output("env")
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
	err = f.testSrv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	se, err := f.ssh.clt.NewSession()
	require.NoError(t, err)
	t.Cleanup(func() { se.Close() })

	err = agent.RequestAgentForwarding(se)
	require.NoError(t, err)

	// prepare to send virtual "keyboard input" into the shell:
	keyboard, err := se.StdinPipe()
	require.NoError(t, err)
	t.Cleanup(func() { keyboard.Close() })

	// start interactive SSH session (new shell):
	err = se.Shell()
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
	}, 5*time.Second, 10*time.Millisecond, "failed to read socket path")

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

	client, err := ssh.Dial("tcp", f.ssh.srv.Addr(), sshConfig)
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
		_, err := net.Dial("unix", socketPath)
		return err != nil
	},
		150*time.Millisecond, 50*time.Millisecond,
		"expected socket to be closed, still could dial")
}

// TestX11Forward tests x11 forwarding via unix sockets
func TestX11Forward(t *testing.T) {
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

	ctx := context.Background()
	roleName := services.RoleNameForUser(f.user)
	role, err := f.testSrv.Auth().GetRole(ctx, roleName)
	require.NoError(t, err)
	roleOptions := role.GetOptions()
	roleOptions.PermitX11Forwarding = types.NewBool(true)
	role.SetOptions(roleOptions)
	err = f.testSrv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	// Open two x11 sessions, the server should handle multiple
	// concurrent X11 sessions.
	serverDisplay := x11EchoSession(ctx, t, f.ssh.clt)

	client2, err := ssh.Dial("tcp", f.ssh.srv.Addr(), f.ssh.cltConfig)
	require.NoError(t, err)
	serverDisplay2 := x11EchoSession(ctx, t, client2)

	// Create multiple XServer requests, the server should
	// handle multiple concurrent XServer requests.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		x11EchoRequest(t, serverDisplay)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		x11EchoRequest(t, serverDisplay)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		x11EchoRequest(t, serverDisplay2)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		x11EchoRequest(t, serverDisplay2)
	}()
	wg.Wait()
}

// x11EchoSession creates a new ssh session and handles x11 forwarding for the session,
// echoing XServer requests received back to the client. Returns the Display opened on the
// session, which is set in $DISPLAY.
func x11EchoSession(ctx context.Context, t *testing.T, clt *ssh.Client) x11.Display {
	se, err := clt.NewSession()
	require.NoError(t, err)
	t.Cleanup(func() { se.Close() })

	// Create a fake client XServer listener which echos
	// back whatever it receives.
	fakeClientDisplay, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	go func() {
		for {
			conn, err := fakeClientDisplay.Accept()
			if err != nil {
				return
			}
			_, err = io.Copy(conn, conn)
			require.NoError(t, err)
			conn.Close()
		}
	}()
	t.Cleanup(func() { fakeClientDisplay.Close() })

	// Handle any x11 channel requests received from the server
	// and start x11 forwarding to the client display.
	err = x11.ServeChannelRequests(ctx, clt, func(ctx context.Context, nch ssh.NewChannel) {
		sch, sin, err := nch.Accept()
		require.NoError(t, err)
		defer sch.Close()

		clientConn, err := net.Dial("tcp", fakeClientDisplay.Addr().String())
		require.NoError(t, err)
		clientXConn, ok := clientConn.(*net.TCPConn)
		require.True(t, ok)
		defer clientConn.Close()

		go func() {
			err := sshutils.ForwardRequests(ctx, sin, se)
			if err != nil {
				log.WithError(err).Debug("Failed to forward ssh request from server during X11 forwarding")
			}
		}()

		err = x11.Forward(ctx, clientXConn, sch)
		require.NoError(t, err)
	})
	require.NoError(t, err)

	// Client requests x11 forwarding for the server session.
	clientXAuthEntry, err := x11.NewFakeXAuthEntry(x11.Display{})
	require.NoError(t, err)
	err = x11.RequestForwarding(se, clientXAuthEntry)
	require.NoError(t, err)

	// prepare to send virtual "keyboard input" into the shell:
	keyboard, err := se.StdinPipe()
	require.NoError(t, err)

	// start interactive SSH session with x11 forwarding enabled (new shell):
	err = se.Shell()
	require.NoError(t, err)

	// create a temp file to collect the shell output into:
	tmpFile, err := os.CreateTemp(os.TempDir(), "teleport-x11-forward-test")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.Remove(tmpFile.Name())
	})

	// type 'printenv DISPLAY > /path/to/tmp/file' into the session (dumping the value of DISPLAY into the temp file)
	_, err = keyboard.Write([]byte(fmt.Sprintf("printenv %v >> %s\n\r", x11.DisplayEnv, tmpFile.Name())))
	require.NoError(t, err)

	// wait for the output
	var display string
	require.Eventually(t, func() bool {
		output, err := os.ReadFile(tmpFile.Name())
		if err == nil && len(output) != 0 {
			display = strings.TrimSpace(string(output))
			return true
		}
		return false
	}, 5*time.Second, 10*time.Millisecond, "failed to read display")

	// Make a new connection to the XServer proxy, the client
	// XServer should echo back anything written on it.
	serverDisplay, err := x11.ParseDisplay(display)
	require.NoError(t, err)

	return serverDisplay
}

// x11EchoRequest sends a message to the serverDisplay and expects the
// server to echo the message back to it.
func x11EchoRequest(t *testing.T, serverDisplay x11.Display) {
	conn, err := serverDisplay.Dial()
	require.NoError(t, err)
	defer conn.Close()

	msg := "msg"
	_, err = conn.Write([]byte(msg))
	require.NoError(t, err)
	err = conn.CloseWrite()
	require.NoError(t, err)

	echo, err := io.ReadAll(conn)
	require.NoError(t, err)
	require.Equal(t, msg, string(echo))
}

func TestAllowedUsers(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	up, err := newUpack(f.testSrv, f.user, []string{f.user}, wildcardAllow)
	require.NoError(t, err)

	sshConfig := &ssh.ClientConfig{
		User:            f.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(f.signer.PublicKey()),
	}

	client, err := ssh.Dial("tcp", f.ssh.srv.Addr(), sshConfig)
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

	_, err = ssh.Dial("tcp", f.ssh.srv.Addr(), sshConfig)
	require.Error(t, err)
}

func TestAllowedLabels(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

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

			_, err = ssh.Dial("tcp", f.ssh.srv.Addr(), sshConfig)
			if tt.outError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestKeyAlgorithms makes sure Teleport does not accept invalid user
// certificates. The main check is the certificate algorithms.
func TestKeyAlgorithms(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	_, ellipticSigner, err := utils.CreateEllipticCertificate("foo", ssh.UserCert)
	require.NoError(t, err)

	sshConfig := &ssh.ClientConfig{
		User:            f.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(ellipticSigner)},
		HostKeyCallback: ssh.FixedHostKey(f.signer.PublicKey()),
	}

	_, err = ssh.Dial("tcp", f.ssh.srv.Addr(), sshConfig)
	require.Error(t, err)
}

func TestInvalidSessionID(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	session, err := f.ssh.clt.NewSession()
	require.NoError(t, err)

	err = session.Setenv(sshutils.SessionEnvVar, "foo")
	require.NoError(t, err)

	err = session.Shell()
	require.Error(t, err)
}

func TestSessionHijack(t *testing.T) {
	t.Parallel()
	_, err := user.Lookup(teleportTestUser)
	if err != nil {
		t.Skipf("user %v is not found, skipping test", teleportTestUser)
	}

	f := newFixture(t)

	// user 1 has access to the server
	up, err := newUpack(f.testSrv, f.user, []string{f.user}, wildcardAllow)
	require.NoError(t, err)

	// login with first user
	sshConfig := &ssh.ClientConfig{
		User:            f.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(f.signer.PublicKey()),
	}

	client, err := ssh.Dial("tcp", f.ssh.srv.Addr(), sshConfig)
	require.NoError(t, err)
	defer func() {
		err := client.Close()
		require.NoError(t, err)
	}()

	se, err := client.NewSession()
	require.NoError(t, err)
	defer se.Close()

	firstSessionID := string(sess.NewID())
	err = se.Setenv(sshutils.SessionEnvVar, firstSessionID)
	require.NoError(t, err)

	err = se.Shell()
	require.NoError(t, err)

	// user 2 does not have s.user as a listed principal
	up2, err := newUpack(f.testSrv, teleportTestUser, []string{teleportTestUser}, wildcardAllow)
	require.NoError(t, err)

	sshConfig2 := &ssh.ClientConfig{
		User:            teleportTestUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(up2.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(f.signer.PublicKey()),
	}

	client2, err := ssh.Dial("tcp", f.ssh.srv.Addr(), sshConfig2)
	require.NoError(t, err)
	defer func() {
		err := client2.Close()
		require.NoError(t, err)
	}()

	se2, err := client2.NewSession()
	require.NoError(t, err)
	defer se2.Close()

	err = se2.Setenv(sshutils.SessionEnvVar, firstSessionID)
	require.NoError(t, err)

	// attempt to hijack, should return error
	err = se2.Shell()
	require.Error(t, err)
}

// testClient dials targetAddr via proxyAddr and executes 2+3 command
func testClient(t *testing.T, f *sshTestFixture, proxyAddr, targetAddr, remoteAddr string, sshConfig *ssh.ClientConfig) {
	// Connect to node using registered address
	client, err := ssh.Dial("tcp", proxyAddr, sshConfig)
	require.NoError(t, err)
	defer client.Close()

	se, err := client.NewSession()
	require.NoError(t, err)
	defer se.Close()

	writer, err := se.StdinPipe()
	require.NoError(t, err)

	reader, err := se.StdoutPipe()
	require.NoError(t, err)

	// Request opening TCP connection to the remote host
	require.NoError(t, se.RequestSubsystem(fmt.Sprintf("proxy:%v", targetAddr)))

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
	conn, chans, reqs, err := ssh.NewClientConn(pipeNetConn,
		f.ssh.srv.Addr(), sshConfig)
	require.NoError(t, err)
	defer conn.Close()

	// using this connection as regular SSH
	client2 := ssh.NewClient(conn, chans, reqs)
	require.NoError(t, err)
	defer client2.Close()

	se2, err := client2.NewSession()
	require.NoError(t, err)
	defer se2.Close()

	out, err := se2.Output("echo hello")
	require.NoError(t, err)

	require.Equal(t, "hello\n", string(out))
}

func mustListen(t *testing.T) (net.Listener, utils.NetAddr) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := utils.NetAddr{AddrNetwork: "tcp", Addr: l.Addr().String()}
	return l, addr
}

func noCache(clt auth.ClientI, cacheName []string) (auth.RemoteProxyAccessPoint, error) {
	return clt, nil
}

func TestProxyRoundRobin(t *testing.T) {
	t.Parallel()

	f := newFixtureWithoutDiskBasedLogging(t)
	ctx := context.Background()

	proxyClient, _ := newProxyClient(t, f.testSrv)
	nodeClient, _ := newNodeClient(t, f.testSrv)

	logger := logrus.WithField("test", "TestProxyRoundRobin")
	listener, reverseTunnelAddress := mustListen(t)
	defer listener.Close()
	lockWatcher := newLockWatcher(ctx, t, proxyClient)
	nodeWatcher := newNodeWatcher(ctx, t, proxyClient)
	caWatcher := newCertAuthorityWatcher(ctx, t, proxyClient)

	reverseTunnelServer, err := reversetunnel.NewServer(reversetunnel.Config{
		ClusterName:                   f.testSrv.ClusterName(),
		ClientTLS:                     proxyClient.TLSConfig(),
		ID:                            hostID,
		Listener:                      listener,
		HostSigners:                   []ssh.Signer{f.signer},
		LocalAuthClient:               proxyClient,
		LocalAccessPoint:              proxyClient,
		NewCachingAccessPoint:         noCache,
		NewCachingAccessPointOldProxy: noCache,
		DataDir:                       t.TempDir(),
		Emitter:                       proxyClient,
		Log:                           logger,
		LockWatcher:                   lockWatcher,
		NodeWatcher:                   nodeWatcher,
		CertAuthorityWatcher:          caWatcher,
		CircuitBreakerConfig:          breaker.NoopBreakerConfig(),
	})
	require.NoError(t, err)
	logger.WithField("tun-addr", reverseTunnelAddress.String()).Info("Created reverse tunnel server.")

	require.NoError(t, reverseTunnelServer.Start())
	defer reverseTunnelServer.Close()

	proxy, err := New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		f.testSrv.ClusterName(),
		[]ssh.Signer{f.signer},
		proxyClient,
		t.TempDir(),
		"",
		utils.NetAddr{},
		proxyClient,
		SetProxyMode("", reverseTunnelServer, proxyClient),
		SetSessionServer(proxyClient),
		SetEmitter(nodeClient),
		SetNamespace(apidefaults.Namespace),
		SetPAMConfig(&pam.Config{Enabled: false}),
		SetBPF(&bpf.NOP{}),
		SetRestrictedSessionManager(&restricted.NOP{}),
		SetClock(f.clock),
		SetLockWatcher(lockWatcher),
		SetNodeWatcher(nodeWatcher),
	)
	require.NoError(t, err)
	require.NoError(t, proxy.Start())
	defer proxy.Close()

	// set up SSH client using the user private key for signing
	up, err := newUpack(f.testSrv, f.user, []string{f.user}, wildcardAllow)
	require.NoError(t, err)

	resolver := func(context.Context) (*utils.NetAddr, error) {
		return &utils.NetAddr{Addr: reverseTunnelAddress.Addr, AddrNetwork: "tcp"}, nil
	}

	pool1, err := reversetunnel.NewAgentPool(ctx, reversetunnel.AgentPoolConfig{
		Resolver:    resolver,
		Client:      proxyClient,
		AccessPoint: proxyClient,
		HostSigner:  f.signer,
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
		HostSigner:  f.signer,
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

	for i := 0; i < 3; i++ {
		testClient(t, f, proxy.Addr(), f.ssh.srvAddress, f.ssh.srv.Addr(), sshConfig)
	}

	// close first connection, and test it again
	pool1.Stop()

	for i := 0; i < 3; i++ {
		testClient(t, f, proxy.Addr(), f.ssh.srvAddress, f.ssh.srv.Addr(), sshConfig)
	}
}

// TestProxyDirectAccess tests direct access via proxy bypassing
// reverse tunnel
func TestProxyDirectAccess(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := context.Background()

	listener, _ := mustListen(t)
	logger := logrus.WithField("test", "TestProxyDirectAccess")
	proxyClient, _ := newProxyClient(t, f.testSrv)
	lockWatcher := newLockWatcher(ctx, t, proxyClient)
	nodeWatcher := newNodeWatcher(ctx, t, proxyClient)
	caWatcher := newCertAuthorityWatcher(ctx, t, proxyClient)

	reverseTunnelServer, err := reversetunnel.NewServer(reversetunnel.Config{
		ClientTLS:                     proxyClient.TLSConfig(),
		ID:                            hostID,
		ClusterName:                   f.testSrv.ClusterName(),
		Listener:                      listener,
		HostSigners:                   []ssh.Signer{f.signer},
		LocalAuthClient:               proxyClient,
		LocalAccessPoint:              proxyClient,
		NewCachingAccessPoint:         noCache,
		NewCachingAccessPointOldProxy: noCache,
		DataDir:                       t.TempDir(),
		Emitter:                       proxyClient,
		Log:                           logger,
		LockWatcher:                   lockWatcher,
		NodeWatcher:                   nodeWatcher,
		CertAuthorityWatcher:          caWatcher,
		CircuitBreakerConfig:          breaker.NoopBreakerConfig(),
	})
	require.NoError(t, err)

	require.NoError(t, reverseTunnelServer.Start())
	defer reverseTunnelServer.Close()

	nodeClient, _ := newNodeClient(t, f.testSrv)

	proxy, err := New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		f.testSrv.ClusterName(),
		[]ssh.Signer{f.signer},
		proxyClient,
		t.TempDir(),
		"",
		utils.NetAddr{},
		proxyClient,
		SetProxyMode("", reverseTunnelServer, proxyClient),
		SetSessionServer(proxyClient),
		SetEmitter(nodeClient),
		SetNamespace(apidefaults.Namespace),
		SetPAMConfig(&pam.Config{Enabled: false}),
		SetBPF(&bpf.NOP{}),
		SetRestrictedSessionManager(&restricted.NOP{}),
		SetClock(f.clock),
		SetLockWatcher(lockWatcher),
		SetNodeWatcher(nodeWatcher),
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

	f := newFixture(t)
	se, err := f.ssh.clt.NewSession()
	require.NoError(t, err)
	defer se.Close()

	// request PTY with valid size
	require.NoError(t, se.RequestPty("xterm", 30, 30, ssh.TerminalModes{}))

	// request PTY with invalid size, should still work (selects defaults)
	require.NoError(t, se.RequestPty("xterm", 0, 0, ssh.TerminalModes{}))
}

// TestEnv requests setting environment variables. (We are currently ignoring these requests)
func TestEnv(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	se, err := f.ssh.clt.NewSession()
	require.NoError(t, err)
	defer se.Close()

	require.NoError(t, se.Setenv("HOME", "/"))
}

// TestNoAuth tries to log in with no auth methods and should be rejected
func TestNoAuth(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	_, err := ssh.Dial("tcp", f.ssh.srv.Addr(), &ssh.ClientConfig{})
	require.Error(t, err)
}

// TestPasswordAuth tries to log in with empty pass and should be rejected
func TestPasswordAuth(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	config := &ssh.ClientConfig{
		Auth:            []ssh.AuthMethod{ssh.Password("")},
		HostKeyCallback: ssh.FixedHostKey(f.signer.PublicKey()),
	}
	_, err := ssh.Dial("tcp", f.ssh.srv.Addr(), config)
	require.Error(t, err)
}

func TestClientDisconnect(t *testing.T) {
	t.Parallel()
	f := newFixtureWithoutDiskBasedLogging(t)

	config := &ssh.ClientConfig{
		User:            f.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(f.up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(f.signer.PublicKey()),
	}
	clt, err := ssh.Dial("tcp", f.ssh.srv.Addr(), config)
	require.NoError(t, err)
	require.NotNil(t, clt)

	se, err := f.ssh.clt.NewSession()
	require.NoError(t, err)
	require.NoError(t, se.Shell())
	require.NoError(t, clt.Close())
}

func getWaitForNumberOfConnsFunc(t *testing.T, limiter *limiter.Limiter, token string, num int64) func() bool {
	return func() bool {
		connNumber, err := limiter.GetNumConnection(token)
		require.NoError(t, err)
		return connNumber == num
	}
}

// fakeClock is a wrapper around clockwork.FakeClock that satisfies the timetoools.TimeProvider interface.
// We are wrapping this so we can use the same mocked clock across the server and rate limiter.
type fakeClock struct {
	clock clockwork.FakeClock
}

func (fc fakeClock) UtcNow() time.Time {
	return fc.clock.Now().UTC()
}

func (fc fakeClock) Sleep(d time.Duration) {
	fc.clock.Advance(d)
}

func (fc fakeClock) After(d time.Duration) <-chan time.Time {
	return fc.clock.After(d)
}

var _ timetools.TimeProvider = (*fakeClock)(nil)

func TestLimiter(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()

	fClock := &fakeClock{
		clock: f.clock,
	}
	limiter, err := limiter.NewLimiter(
		limiter.Config{
			Clock:          fClock,
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
	nodeStateDir := t.TempDir()
	srv, err := New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		f.testSrv.ClusterName(),
		[]ssh.Signer{f.signer},
		nodeClient,
		nodeStateDir,
		"",
		utils.NetAddr{},
		nodeClient,
		SetLimiter(limiter),
		SetShell("/bin/sh"),
		SetSessionServer(nodeClient),
		SetEmitter(nodeClient),
		SetNamespace(apidefaults.Namespace),
		SetPAMConfig(&pam.Config{Enabled: false}),
		SetBPF(&bpf.NOP{}),
		SetRestrictedSessionManager(&restricted.NOP{}),
		SetClock(f.clock),
		SetLockWatcher(newLockWatcher(ctx, t, nodeClient)),
	)
	require.NoError(t, err)
	require.NoError(t, srv.Start())

	require.NoError(t, auth.CreateUploaderDir(nodeStateDir))
	defer srv.Close()

	config := &ssh.ClientConfig{
		User:            f.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(f.up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(f.signer.PublicKey()),
	}

	clt0, err := ssh.Dial("tcp", srv.Addr(), config)
	require.NoError(t, err)
	require.NotNil(t, clt0)

	se0, err := clt0.NewSession()
	require.NoError(t, err)
	require.NoError(t, se0.Shell())

	// current connections = 1
	clt, err := ssh.Dial("tcp", srv.Addr(), config)
	require.NoError(t, err)
	require.NotNil(t, clt)
	se, err := clt.NewSession()
	require.NoError(t, err)
	require.NoError(t, se.Shell())

	// current connections = 2
	_, err = ssh.Dial("tcp", srv.Addr(), config)
	require.Error(t, err)

	require.NoError(t, se.Close())
	se.Wait()
	require.NoError(t, clt.Close())
	require.ErrorIs(t, clt.Wait(), net.ErrClosed)

	require.Eventually(t, getWaitForNumberOfConnsFunc(t, limiter, "127.0.0.1", 1), time.Second*10, time.Millisecond*100)

	// current connections = 1
	clt, err = ssh.Dial("tcp", srv.Addr(), config)
	require.NoError(t, err)
	require.NotNil(t, clt)
	se, err = clt.NewSession()
	require.NoError(t, err)
	require.NoError(t, se.Shell())

	// current connections = 2
	_, err = ssh.Dial("tcp", srv.Addr(), config)
	require.Error(t, err)

	require.NoError(t, se.Close())
	se.Wait()
	require.NoError(t, clt.Close())
	require.ErrorIs(t, clt.Wait(), net.ErrClosed)

	require.Eventually(t, getWaitForNumberOfConnsFunc(t, limiter, "127.0.0.1", 1), time.Second*10, time.Millisecond*100)

	// current connections = 1
	// requests rate should exceed now
	clt, err = ssh.Dial("tcp", srv.Addr(), config)
	require.NoError(t, err)
	require.NotNil(t, clt)
	_, err = clt.NewSession()
	require.Error(t, err)

	clt.Close()
}

// TestServerAliveInterval simulates ServerAliveInterval and OpenSSH
// interoperability by sending a keepalive@openssh.com global request to the
// server and expecting a response in return.
func TestServerAliveInterval(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ok, _, err := f.ssh.clt.SendRequest(teleport.KeepAliveReqType, true, nil)
	require.NoError(t, err)
	require.True(t, ok)
}

// TestGlobalRequestRecordingProxy simulates sending a global out-of-band
// recording-proxy@teleport.com request.
func TestGlobalRequestRecordingProxy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newFixture(t)

	// set cluster config to record at the node
	recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
		Mode: types.RecordAtNode,
	})
	require.NoError(t, err)
	err = f.testSrv.Auth().SetSessionRecordingConfig(ctx, recConfig)
	require.NoError(t, err)

	// send the request again, we have cluster config and when we parse the
	// response, it should be false because recording is occurring at the node.
	ok, responseBytes, err := f.ssh.clt.SendRequest(teleport.RecordingProxyReqType, true, nil)
	require.NoError(t, err)
	require.True(t, ok)
	response, err := strconv.ParseBool(string(responseBytes))
	require.NoError(t, err)
	require.False(t, response)

	// set cluster config to record at the proxy
	recConfig, err = types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
		Mode: types.RecordAtProxy,
	})
	require.NoError(t, err)
	err = f.testSrv.Auth().SetSessionRecordingConfig(ctx, recConfig)
	require.NoError(t, err)

	// send request again, now that we have cluster config and it's set to record
	// at the proxy, we should return true and when we parse the payload it should
	// also be true
	ok, responseBytes, err = f.ssh.clt.SendRequest(teleport.RecordingProxyReqType, true, nil)
	require.NoError(t, err)
	require.True(t, ok)
	response, err = strconv.ParseBool(string(responseBytes))
	require.NoError(t, err)
	require.True(t, response)
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

	priv, pub, err := native.GenerateKeyPair()
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
func startX11EchoServer(ctx context.Context, t *testing.T, authSrv *auth.Server) (*rawNode, <-chan error) {
	node := newRawNode(t, authSrv)

	sessionMain := func(ctx context.Context, conn *ssh.ServerConn, chs <-chan ssh.NewChannel) error {
		defer conn.Close()

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

		if req.Type != sshutils.X11ForwardRequest {
			return trace.BadParameter("Unexpected request type %q", req.Type)
		}

		if err = req.Reply(true, nil); err != nil {
			return trace.Wrap(err)
		}

		// start a fake X11 channel
		xch, _, err := conn.OpenChannel(sshutils.X11ChannelRequest, nil)
		if err != nil {
			return trace.Wrap(err)
		}
		defer xch.Close()

		// echo all bytes back across the X11 channel
		_, err = io.Copy(xch, xch)
		if err == nil {
			xch.CloseWrite()
		} else {
			log.Errorf("X11 channel error: %v", err)
		}

		return nil
	}

	errorCh := make(chan error, 1)

	nodeMain := func() {
		for {
			conn, chs, _, err := node.accept()
			if err != nil {
				log.Warnf("X11 echo server closing: %v", err)
				return
			}
			go func() {
				if err := sessionMain(ctx, conn, chs); err != nil {
					errorCh <- err
				}
			}()
		}
	}

	go nodeMain()

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

// TestX11ProxySupport verifies that recording proxies correctly forward
// X11 request/channels.
func TestX11ProxySupport(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// set cluster config to record at the proxy
	recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
		Mode: types.RecordAtProxy,
	})
	require.NoError(t, err)
	err = f.testSrv.Auth().SetSessionRecordingConfig(ctx, recConfig)
	require.NoError(t, err)

	// verify that the proxy is in recording mode
	ok, responseBytes, err := f.ssh.clt.SendRequest(teleport.RecordingProxyReqType, true, nil)
	require.NoError(t, err)
	require.True(t, ok)
	response, err := strconv.ParseBool(string(responseBytes))
	require.NoError(t, err)
	require.True(t, response)

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
	netConn, err := f.ssh.clt.Dial("tcp", node.addr)
	require.NoError(t, err)
	defer netConn.Close()

	// make an insecure version of our client config (this test is only about X11 forwarding,
	// so we don't bother to verify recording proxy key generation here).
	cltConfig := *f.ssh.cltConfig
	cltConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	// Perform ssh handshake and setup client for X11 test server.
	cltConn, chs, reqs, err := ssh.NewClientConn(netConn, node.addr, &cltConfig)
	require.NoError(t, err)
	clt := ssh.NewClient(cltConn, chs, reqs)

	sess, err := clt.NewSession()
	require.NoError(t, err)

	// register X11 channel handler before requesting forwarding to avoid races
	xchs := clt.HandleChannelOpen(sshutils.X11ChannelRequest)
	require.NotNil(t, xchs)

	// Send an X11 forwarding request to the server
	ok, err = sess.SendRequest(sshutils.X11ForwardRequest, true, nil)
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
	require.Equal(t, sshutils.X11ChannelRequest, xnc.ChannelType())

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
	logger := logrus.WithField("test", "TestIgnorePuTTYSimpleChannel")
	proxyClient, _ := newProxyClient(t, f.testSrv)
	lockWatcher := newLockWatcher(ctx, t, proxyClient)
	nodeWatcher := newNodeWatcher(ctx, t, proxyClient)
	caWatcher := newCertAuthorityWatcher(ctx, t, proxyClient)

	reverseTunnelServer, err := reversetunnel.NewServer(reversetunnel.Config{
		ClientTLS:                     proxyClient.TLSConfig(),
		ID:                            hostID,
		ClusterName:                   f.testSrv.ClusterName(),
		Listener:                      listener,
		HostSigners:                   []ssh.Signer{f.signer},
		LocalAuthClient:               proxyClient,
		LocalAccessPoint:              proxyClient,
		NewCachingAccessPoint:         noCache,
		NewCachingAccessPointOldProxy: noCache,
		DataDir:                       t.TempDir(),
		Emitter:                       proxyClient,
		Log:                           logger,
		LockWatcher:                   lockWatcher,
		NodeWatcher:                   nodeWatcher,
		CertAuthorityWatcher:          caWatcher,
	})
	require.NoError(t, err)

	require.NoError(t, reverseTunnelServer.Start())
	defer reverseTunnelServer.Close()

	nodeClient, _ := newNodeClient(t, f.testSrv)

	proxy, err := New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		f.testSrv.ClusterName(),
		[]ssh.Signer{f.signer},
		proxyClient,
		t.TempDir(),
		"",
		utils.NetAddr{},
		proxyClient,
		SetProxyMode("", reverseTunnelServer, proxyClient),
		SetSessionServer(proxyClient),
		SetEmitter(nodeClient),
		SetNamespace(apidefaults.Namespace),
		SetPAMConfig(&pam.Config{Enabled: false}),
		SetBPF(&bpf.NOP{}),
		SetRestrictedSessionManager(&restricted.NOP{}),
		SetClock(f.clock),
		SetLockWatcher(lockWatcher),
		SetNodeWatcher(nodeWatcher),
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
	client, err := ssh.Dial("tcp", proxy.Addr(), sshConfig)

	require.NoError(t, err)
	defer client.Close()

	se, err := client.NewSession()
	require.NoError(t, err)
	defer se.Close()

	writer, err := se.StdinPipe()
	require.NoError(t, err)

	reader, err := se.StdoutPipe()
	require.NoError(t, err)

	// Request the PuTTY-specific "simple@putty.projects.tartarus.org" channel type
	// This request should be ignored, and the connection should remain open.
	_, err = se.SendRequest(sshutils.PuTTYSimpleRequest, false, []byte{})
	require.NoError(t, err)

	// Request proxy subsystem routing TCP connection to the remote host
	require.NoError(t, se.RequestSubsystem(fmt.Sprintf("proxy:%v", f.ssh.srvAddress)))

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
	conn, chans, reqs, err := ssh.NewClientConn(pipeNetConn,
		f.ssh.srv.Addr(), sshConfig)
	require.NoError(t, err)
	defer conn.Close()

	// Run commands over this connection like regular SSH
	client2 := ssh.NewClient(conn, chans, reqs)
	require.NoError(t, err)
	defer client2.Close()

	se2, err := client2.NewSession()
	require.NoError(t, err)
	defer se2.Close()

	out, err := se2.Output("echo hello again")
	require.NoError(t, err)

	require.Equal(t, "hello again\n", string(out))
}

// upack holds all ssh signing artifacts needed for signing and checking user keys
type upack struct {
	// key is a raw private user key
	key []byte

	// pkey is parsed private SSH key
	pkey interface{}

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
	upriv, upub, err := native.GenerateKeyPair()
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
	role.SetOptions(opts)
	role.SetLogins(types.Allow, allowedLogins)
	role.SetNodeLabels(types.Allow, allowedLabels)
	err = auth.UpsertRole(ctx, role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.AddRole(role.GetName())
	err = auth.UpsertUser(user)
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

func newLockWatcher(ctx context.Context, t *testing.T, client types.Events) *services.LockWatcher {
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

func newNodeWatcher(ctx context.Context, t *testing.T, client types.Events) *services.NodeWatcher {
	nodeWatcher, err := services.NewNodeWatcher(ctx, services.NodeWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: "test",
			Client:    client,
		},
	})
	require.NoError(t, err)
	t.Cleanup(nodeWatcher.Close)
	return nodeWatcher
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

// maxPipeSize is one larger than the maximum pipe size for most operating
// systems which appears to be 65536 bytes.
//
// The maximum pipe size for Linux could potentially be obtained, however
// getting it for macOS is much harder, and unclear if even possible. Therefor
// just hard code it.
//
// See the following links for more details.
//
//   https://man7.org/linux/man-pages/man7/pipe.7.html
//   https://github.com/afborchert/pipebuf
//   https://unix.stackexchange.com/questions/11946/how-big-is-the-pipe-buffer
const maxPipeSize = 65536 + 1
