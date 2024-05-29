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
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"os/user"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
)

func newTestServerContext(t *testing.T, srv Server, roleSet services.RoleSet) *ServerContext {
	usr, err := user.Current()
	require.NoError(t, err)

	cert, err := apisshutils.ParseCertificate([]byte(fixtures.UserCertificateStandard))
	require.NoError(t, err)

	sshConn := &mockSSHConn{}
	sshConn.localAddr, _ = utils.ParseAddr("127.0.0.1:3022")
	sshConn.remoteAddr, _ = utils.ParseAddr("10.0.0.5:4817")

	ctx, cancel := context.WithCancel(context.Background())
	recConfig := types.DefaultSessionRecordingConfig()
	recConfig.SetMode(types.RecordOff)
	clusterName := "localhost"
	scx := &ServerContext{
		Entry: logrus.NewEntry(logrus.StandardLogger()),
		ConnectionContext: &sshutils.ConnectionContext{
			ServerConn: &ssh.ServerConn{Conn: sshConn},
		},
		env:                    make(map[string]string),
		SessionRecordingConfig: recConfig,
		IsTestStub:             true,
		ClusterName:            clusterName,
		srv:                    srv,
		Identity: IdentityContext{
			Login:        usr.Username,
			TeleportUser: "teleportUser",
			Certificate:  cert,
			// roles do not actually exist in mock backend, just need a non-nil
			// access checker to avoid panic
			AccessChecker: services.NewAccessCheckerWithRoleSet(
				&services.AccessInfo{Roles: roleSet.RoleNames()}, clusterName, roleSet),
		},
		cancelContext: ctx,
		cancel:        cancel,
	}

	err = scx.SetExecRequest(&localExec{Ctx: scx})
	require.NoError(t, err)

	scx.cmdr, scx.cmdw, err = os.Pipe()
	require.NoError(t, err)

	scx.contr, scx.contw, err = os.Pipe()
	require.NoError(t, err)

	scx.readyr, scx.readyw, err = os.Pipe()
	require.NoError(t, err)

	scx.killShellr, scx.killShellw, err = os.Pipe()
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, scx.Close()) })

	return scx
}

func newMockServer(t *testing.T) *mockServer {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	bk, err := lite.NewWithConfig(ctx, lite.Config{
		Path:  t.TempDir(),
		Clock: clock,
	})
	require.NoError(t, err)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "localhost",
	})
	require.NoError(t, err)

	staticTokens, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{},
	})
	require.NoError(t, err)

	authCfg := &auth.InitConfig{
		Backend:      bk,
		Authority:    testauthority.New(),
		ClusterName:  clusterName,
		StaticTokens: staticTokens,
		KeyStoreConfig: keystore.Config{
			Software: keystore.SoftwareConfig{
				RSAKeyPairSource: testauthority.New().GenerateKeyPair,
			},
		},
	}

	authServer, err := auth.NewServer(authCfg, auth.WithClock(clock))
	require.NoError(t, err)

	return &mockServer{
		auth:                authServer,
		datadir:             t.TempDir(),
		MockRecorderEmitter: &eventstest.MockRecorderEmitter{},
		clock:               clock,
	}
}

type mockServer struct {
	*eventstest.MockRecorderEmitter
	datadir   string
	auth      *auth.Server
	component string
	clock     clockwork.FakeClock
	bpf       bpf.BPF
}

// ID is the unique ID of the server.
func (m *mockServer) ID() string {
	return "testID"
}

// HostUUID is the UUID of the underlying host. For the forwarding
// server this is the proxy the forwarding server is running in.
func (m *mockServer) HostUUID() string {
	return "testHostUUID"
}

// GetNamespace returns the namespace the server was created in.
func (m *mockServer) GetNamespace() string {
	return "testNamespace"
}

// AdvertiseAddr is the publicly addressable address of this server.
func (m *mockServer) AdvertiseAddr() string {
	return "testAdvertiseAddr"
}

// Component is the type of server, forwarding or regular.
func (m *mockServer) Component() string {
	return m.component
}

// PermitUserEnvironment returns if reading environment variables upon
// startup is allowed.
func (m *mockServer) PermitUserEnvironment() bool {
	return false
}

// GetAccessPoint returns an AccessPoint for this cluster.
func (m *mockServer) GetAccessPoint() AccessPoint {
	return m.auth
}

// GetDataDir returns data directory of the server
func (m *mockServer) GetDataDir() string {
	return m.datadir
}

// GetPAM returns PAM configuration for this server.
func (m *mockServer) GetPAM() (*servicecfg.PAMConfig, error) {
	return &servicecfg.PAMConfig{}, nil
}

// GetClock returns a clock setup for the server
func (m *mockServer) GetClock() clockwork.Clock {
	if m.clock != nil {
		return m.clock
	}
	return clockwork.NewRealClock()
}

// GetInfo returns a services.Server that represents this server.
func (m *mockServer) GetInfo() types.Server {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}

	return &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      "",
			Namespace: "",
			Labels:    make(map[string]string),
		},
		Spec: types.ServerSpecV2{
			CmdLabels: make(map[string]types.CommandLabelV2),
			Addr:      "",
			Hostname:  hostname,
			UseTunnel: false,
			Version:   teleport.Version,
		},
	}
}

func (m *mockServer) TargetMetadata() apievents.ServerMetadata {
	return apievents.ServerMetadata{}
}

// UseTunnel used to determine if this node has connected to this cluster
// using reverse tunnel.
func (m *mockServer) UseTunnel() bool {
	return false
}

// GetBPF returns the BPF service used for enhanced session recording.
func (m *mockServer) GetBPF() bpf.BPF {
	if m.bpf != nil {
		return m.bpf
	}

	return &bpf.NOP{}
}

// Context returns server shutdown context
func (m *mockServer) Context() context.Context {
	return context.Background()
}

// GetUserAccountingPaths returns the path of the user accounting database and log. Returns empty for system defaults.
func (m *mockServer) GetUserAccountingPaths() (utmp, wtmp, btmp string) {
	return "test", "test", "test"
}

// GetLockWatcher gets the server's lock watcher.
func (m *mockServer) GetLockWatcher() *services.LockWatcher {
	return nil
}

// GetCreateHostUser gets whether the server allows host user creation
// or not
func (m *mockServer) GetCreateHostUser() bool {
	return false
}

// GetHostUsers
func (m *mockServer) GetHostUsers() HostUsers {
	return nil
}

// GetHostSudoers
func (m *mockServer) GetHostSudoers() HostSudoers {
	return &HostSudoersNotImplemented{}
}

// Implementation of ssh.Conn interface.
type mockSSHConn struct {
	remoteAddr net.Addr
	localAddr  net.Addr
}

func (c *mockSSHConn) User() string {
	return ""
}

func (c *mockSSHConn) SessionID() []byte {
	return []byte{1, 2, 3}
}

func (c *mockSSHConn) ClientVersion() []byte {
	return []byte{1}
}

func (c *mockSSHConn) ServerVersion() []byte {
	return []byte{1}
}

func (c *mockSSHConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *mockSSHConn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *mockSSHConn) Close() error {
	return nil
}

func (c *mockSSHConn) SendRequest(string, bool, []byte) (bool, []byte, error) {
	return false, nil, nil
}

func (c *mockSSHConn) OpenChannel(string, []byte) (ssh.Channel, <-chan *ssh.Request, error) {
	return nil, nil, nil
}

func (c *mockSSHConn) Wait() error {
	return nil
}

type mockSSHChannel struct {
	stdIn  io.ReadCloser
	stdOut io.WriteCloser
	stdErr io.ReadWriter
}

func newMockSSHChannel() ssh.Channel {
	stdIn, stdOut := io.Pipe()
	return &mockSSHChannel{
		stdIn:  stdIn,
		stdOut: stdOut,
		stdErr: new(bytes.Buffer),
	}
}

// Read reads up to len(data) bytes from the channel.
func (c *mockSSHChannel) Read(data []byte) (int, error) {
	return c.stdIn.Read(data)
}

// Write writes len(data) bytes to the channel.
func (c *mockSSHChannel) Write(data []byte) (int, error) {
	return c.stdOut.Write(data)
}

// Close signals end of channel use. No data may be sent after this
// call.
func (c *mockSSHChannel) Close() error {
	return trace.NewAggregate(c.stdIn.Close(), c.stdOut.Close())
}

// CloseWrite signals the end of sending in-band
// data. Requests may still be sent, and the other side may
// still send data
func (c *mockSSHChannel) CloseWrite() error {
	return trace.NewAggregate(c.stdOut.Close())
}

// SendRequest sends a channel request.  If wantReply is true,
// it will wait for a reply and return the result as a
// boolean, otherwise the return value will be false. Channel
// requests are out-of-band messages so they may be sent even
// if the data stream is closed or blocked by flow control.
// If the channel is closed before a reply is returned, io.EOF
// is returned.
func (c *mockSSHChannel) SendRequest(name string, wantReply bool, payload []byte) (bool, error) {
	return true, nil
}

// Stderr returns an io.ReadWriter that writes to this channel
// with the extended data type set to stderr. Stderr may
// safely be read and written from a different goroutine than
// Read and Write respectively.
func (c *mockSSHChannel) Stderr() io.ReadWriter {
	return c.stdErr
}

type fakeBPF struct {
	bpf bpf.NOP
}

func (f fakeBPF) OpenSession(ctx *bpf.SessionContext) (uint64, error) {
	return f.bpf.OpenSession(ctx)
}

func (f fakeBPF) CloseSession(ctx *bpf.SessionContext) error {
	return f.bpf.CloseSession(ctx)
}

func (f fakeBPF) Close(restarting bool) error {
	return f.bpf.Close(restarting)
}

func (f fakeBPF) Enabled() bool {
	return true
}
