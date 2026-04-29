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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/user"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh/agent"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/lib/sshagent"
	"github.com/gravitational/teleport/lib/utils/testutils"
	"github.com/gravitational/teleport/session/host"
	"github.com/gravitational/teleport/session/networking"
	"github.com/gravitational/teleport/session/networking/x11"
	"github.com/gravitational/teleport/session/reexec/reexecconstants"
)

func newHTTPTestServer(t *testing.T, listener net.Listener) *httptest.Server {
	var err error
	if listener == nil {
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
	}
	tsrv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hello, world")
	}))
	tsrv.Listener = listener
	tsrv.Start()
	t.Cleanup(tsrv.Close)
	return tsrv
}

func TestNetworkingCommand(t *testing.T) {
	t.Parallel()
	testNetworkingCommand(t, "")
}

// TestRootRemotePortForwardCommand tests that networking commands work
// for a user different than the one running a node (which we need to run
// as root to create).
func TestRootNetworkingCommand(t *testing.T) {
	testutils.RequireRoot(t)

	login := testutils.GenerateLocalUsername(t)
	_, err := host.UserAdd(login, nil, host.UserOpts{})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := host.UserDel(login)
		require.NoError(t, err)
	})

	testNetworkingCommand(t, login)
}

func testNetworkingCommand(t *testing.T, login string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := newMockServer(t)

	scx := newTestServerContext(t, srv, nil, &decisionpb.SSHAccessPermit{})
	scx.ExecType = reexecconstants.NetworkingSubCommand
	if login != "" {
		scx.Identity.Login = login
	}

	// Start networking subprocess.
	command, err := scx.ConfigureCommand(nil)
	require.NoError(t, err)
	proc, err := networking.NewProcess(ctx, command.Cmd)
	require.NoError(t, err)
	t.Cleanup(func() { proc.Close() })

	t.Run("local port forward", func(t *testing.T) {
		testLocalPortForward(ctx, t, proc)
	})

	t.Run("remote port forward", func(t *testing.T) {
		testRemotePortForward(ctx, t, proc)
	})

	t.Run("agent forward", func(t *testing.T) {
		testAgentForward(ctx, t, proc)
	})

	t.Run("x11 forward", func(t *testing.T) {
		testX11Forward(ctx, t, proc, login)
	})
}

func testLocalPortForward(ctx context.Context, t *testing.T, proc *networking.Process) {
	// Create a client that will dial via the networking process.
	httpClient := http.Client{
		Transport: &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) {
				return proc.Dial(ctx, network, addr)
			},
		},
	}

	// Test the dialer on an http server.
	tsrv := newHTTPTestServer(t, nil)
	resp, err := httpClient.Get(tsrv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "Hello, world", string(body))
}

func testRemotePortForward(ctx context.Context, t *testing.T, proc *networking.Process) {
	// Request a listener from the networking process.
	listener, err := proc.Listen(ctx, "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	// Test the listener on an http server.
	tsrv := newHTTPTestServer(t, listener)
	resp, err := http.Get(tsrv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "Hello, world", string(body))
}

func testAgentForward(ctx context.Context, t *testing.T, proc *networking.Process) {
	// Create an agent keyring with a test key.
	keyring, ok := agent.NewKeyring().(agent.ExtendedAgent)
	require.True(t, ok)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	testKey := agent.AddedKey{
		PrivateKey: key,
		Comment:    "test-key",
	}

	err = keyring.Add(testKey)
	require.NoError(t, err)

	agentServer := sshagent.NewServer(sshagent.NewStaticClientGetter(keyring))

	// Forward the agent over the networking process.
	listener, err := proc.ListenAgent(ctx)
	require.NoError(t, err)
	agentServer.SetListener(listener)

	go agentServer.Serve()
	t.Cleanup(func() {
		agentServer.Close()
	})

	agentConn, err := net.Dial(listener.Addr().Network(), listener.Addr().String())
	require.NoError(t, err)

	agentClient := agent.NewClient(agentConn)
	keys, err := agentClient.List()
	require.NoError(t, err)
	require.Len(t, keys, 1)
}

func testX11Forward(ctx context.Context, t *testing.T, proc *networking.Process, login string) {
	if os.Getenv("TELEPORT_XAUTH_TEST") == "" {
		t.Skip("Skipping test as xauth is not enabled")
	}

	localUser, err := user.Current()
	if login != "" {
		localUser, err = user.Lookup(login)
	}
	require.NoError(t, err)

	cred, err := host.GetHostUserCredential(localUser)
	require.NoError(t, err)

	// Create a temporary xauth file path belonging to the user.
	tempDir, err := os.MkdirTemp("", "xauth-temp")
	require.NoError(t, err)
	err = os.Chown(tempDir, int(cred.Uid), int(cred.Gid))
	require.NoError(t, err)
	xauthTempFilePath := filepath.Join(tempDir, ".Xauthority")

	fakeXauthEntry, err := x11.NewFakeXAuthEntry(x11.Display{})
	require.NoError(t, err)

	// Request a listener from the networking process.
	listener, err := proc.ListenX11(ctx, networking.X11Request{
		XauthFile: xauthTempFilePath,
		ForwardRequestPayload: x11.ForwardRequestPayload{
			AuthProtocol: fakeXauthEntry.Proto,
			AuthCookie:   fakeXauthEntry.Cookie,
		},
		DisplayOffset: x11.DefaultDisplayOffset,
		MaxDisplay:    x11.DefaultMaxDisplays,
	})
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	// Make the listener an echo server, since testing with an
	// actual X client and server is not feasible
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_, _ = io.Copy(conn, conn)
			conn.Close()
		}
	}()

	display, err := x11.ParseDisplayFromUnixSocket(listener.Addr().String())
	require.NoError(t, err)

	// Try connecting to the x11 listener to ensure it's working.
	conn, err := display.Dial()
	require.NoError(t, err)
	echoMsg := []byte("echo")
	_, err = conn.Write(echoMsg)
	require.NoError(t, err)
	buf := make([]byte, 4)
	_, err = conn.Read(buf)
	require.NoError(t, err)
	require.Equal(t, echoMsg, buf)

	// Check that the xauth entry was stored for the listener's corresponding x11 display
	// in the user's xauth file.
	fakeXauthEntry.Display = display
	xauthCmd := x11.NewXAuthCommand(ctx, xauthTempFilePath)
	xauthCmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:     true,
		Credential: cred,
	}
	readXauthEntry, err := xauthCmd.ReadEntry(display)
	require.NoError(t, err)
	require.Equal(t, fakeXauthEntry, readXauthEntry)
}
