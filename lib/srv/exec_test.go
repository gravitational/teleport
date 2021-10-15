// +build linux

/*
Copyright 2015-2018 Gravitational, Inc.

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
	"fmt"
	"io"
	"net"
	"os"
	os_exec "os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/pam"
	restricted "github.com/gravitational/teleport/lib/restrictedsession"
	"github.com/gravitational/teleport/lib/services"
	rsession "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/jonboulle/clockwork"
	"github.com/moby/term"
	"gopkg.in/check.v1"

	"github.com/gravitational/trace"
)

// ExecSuite also implements ssh.ConnMetadata
type ExecSuite struct {
	usr        *user.User
	ctx        *ServerContext
	localAddr  net.Addr
	remoteAddr net.Addr
	a          *auth.Server
}

var _ = check.Suite(&ExecSuite{})

// TestMain will re-execute Teleport to run a command if "exec" is passed to
// it as an argument. Otherwise it will run tests as normal.
func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	// If the test is re-executing itself, execute the command that comes over
	// the pipe.
	if len(os.Args) == 2 && os.Args[1] == teleport.ExecSubCommand {
		RunAndExit(teleport.ExecSubCommand)
		return
	}

	// Otherwise run tests as normal.
	code := m.Run()
	os.Exit(code)
}

func (s *ExecSuite) SetUpSuite(c *check.C) {
	ctx := context.TODO()
	bk, err := lite.NewWithConfig(ctx, lite.Config{Path: c.MkDir()})
	c.Assert(err, check.IsNil)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "localhost",
	})
	c.Assert(err, check.IsNil)

	c.Assert(err, check.IsNil)
	s.a, err = auth.NewServer(&auth.InitConfig{
		Backend:     bk,
		Authority:   authority.New(),
		ClusterName: clusterName,
	})
	c.Assert(err, check.IsNil)
	err = s.a.SetClusterName(clusterName)
	c.Assert(err, check.IsNil)

	// set static tokens
	staticTokens, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{},
	})
	c.Assert(err, check.IsNil)
	err = s.a.SetStaticTokens(staticTokens)
	c.Assert(err, check.IsNil)

	dir := c.MkDir()
	f, err := os.Create(filepath.Join(dir, "fake"))
	c.Assert(err, check.IsNil)

	s.usr, _ = user.Current()
	cert, err := apisshutils.ParseCertificate([]byte(fixtures.UserCertificateStandard))
	c.Assert(err, check.IsNil)
	s.ctx = &ServerContext{
		ConnectionContext: &sshutils.ConnectionContext{
			ServerConn: &ssh.ServerConn{Conn: s},
		},
		IsTestStub:  true,
		ClusterName: "localhost",
		srv: &fakeServer{
			accessPoint: s.a,
			auditLog:    &fakeLog{},
			id:          "00000000-0000-0000-0000-000000000000",
		},
		Identity: IdentityContext{
			Login:        s.usr.Username,
			TeleportUser: "galt",
			Certificate:  cert,
		},
		session:     &session{id: "xxx", term: &fakeTerminal{f: f}},
		ExecRequest: &localExec{Ctx: s.ctx},
		request: &ssh.Request{
			Type: sshutils.ExecRequest,
		},
	}

	term, err := newLocalTerminal(s.ctx)
	c.Assert(err, check.IsNil)
	term.SetTermType("xterm")
	s.ctx.session.term = term

	s.localAddr, _ = utils.ParseAddr("127.0.0.1:3022")
	s.remoteAddr, _ = utils.ParseAddr("10.0.0.5:4817")
}

func (s *ExecSuite) TearDownSuite(c *check.C) {
	s.ctx.session.term.Close()
}

func (s *ExecSuite) TestOSCommandPrep(c *check.C) {
	expectedEnv := []string{
		"LANG=en_US.UTF-8",
		getDefaultEnvPath(strconv.Itoa(os.Geteuid()), defaultLoginDefsPath),
		fmt.Sprintf("HOME=%s", s.usr.HomeDir),
		fmt.Sprintf("USER=%s", s.usr.Username),
		"SHELL=/bin/sh",
		"SSH_CLIENT=10.0.0.5 4817 3022",
		"SSH_CONNECTION=10.0.0.5 4817 127.0.0.1 3022",
		"TERM=xterm",
		fmt.Sprintf("SSH_TTY=%v", s.ctx.session.term.TTY().Name()),
		"SSH_SESSION_ID=xxx",
		"SSH_SESSION_WEBPROXY_ADDR=<proxyhost>:3080",
		"SSH_TELEPORT_HOST_UUID=00000000-0000-0000-0000-000000000000",
		"SSH_TELEPORT_CLUSTER_NAME=localhost",
		"SSH_TELEPORT_USER=galt",
	}

	// Empty command (simple shell).
	execCmd, err := s.ctx.ExecCommand()
	c.Assert(err, check.IsNil)
	cmd, err := buildCommand(execCmd, nil, nil, nil)
	c.Assert(err, check.IsNil)
	c.Assert(cmd, check.NotNil)
	c.Assert(cmd.Path, check.Equals, "/bin/sh")
	c.Assert(cmd.Args, check.DeepEquals, []string{"-sh"})
	c.Assert(cmd.Dir, check.Equals, s.usr.HomeDir)
	c.Assert(cmd.Env, check.DeepEquals, expectedEnv)
	c.Assert(cmd.SysProcAttr.Pdeathsig, check.Equals, syscall.SIGKILL)

	// Non-empty command (exec a prog).
	s.ctx.ExecRequest.SetCommand("ls -lh /etc")
	execCmd, err = s.ctx.ExecCommand()
	c.Assert(err, check.IsNil)
	cmd, err = buildCommand(execCmd, nil, nil, nil)
	c.Assert(err, check.IsNil)
	c.Assert(cmd, check.NotNil)
	c.Assert(cmd.Path, check.Equals, "/bin/sh")
	c.Assert(cmd.Args, check.DeepEquals, []string{"/bin/sh", "-c", "ls -lh /etc"})
	c.Assert(cmd.Dir, check.Equals, s.usr.HomeDir)
	c.Assert(cmd.Env, check.DeepEquals, expectedEnv)
	c.Assert(cmd.SysProcAttr.Pdeathsig, check.Equals, syscall.SIGKILL)

	// Command without args.
	s.ctx.ExecRequest.SetCommand("top")
	execCmd, err = s.ctx.ExecCommand()
	c.Assert(err, check.IsNil)
	cmd, err = buildCommand(execCmd, nil, nil, nil)
	c.Assert(err, check.IsNil)
	c.Assert(cmd.Path, check.Equals, "/bin/sh")
	c.Assert(cmd.Args, check.DeepEquals, []string{"/bin/sh", "-c", "top"})
	c.Assert(cmd.SysProcAttr.Pdeathsig, check.Equals, syscall.SIGKILL)
}

func (s *ExecSuite) TestLoginDefsParser(c *check.C) {
	expectedEnvSuPath := "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/bar"
	expectedSuPath := "PATH=/usr/local/bin:/usr/bin:/bin:/foo"

	c.Assert(getDefaultEnvPath("0", "../../fixtures/login.defs"), check.Equals, expectedEnvSuPath)
	c.Assert(getDefaultEnvPath("1000", "../../fixtures/login.defs"), check.Equals, expectedSuPath)
	c.Assert(getDefaultEnvPath("1000", "bad/file"), check.Equals, defaultEnvPath)
}

// TestEmitExecAuditEvent make sure the full command and exit code for a
// command is always recorded.
func (s *ExecSuite) TestEmitExecAuditEvent(c *check.C) {
	fakeServer, ok := s.ctx.srv.(*fakeServer)
	c.Assert(ok, check.Equals, true)

	var tests = []struct {
		inCommand  string
		inError    error
		outCommand string
		outCode    string
	}{
		// Successful execution.
		{
			inCommand:  "exit 0",
			inError:    nil,
			outCommand: "exit 0",
			outCode:    strconv.Itoa(teleport.RemoteCommandSuccess),
		},
		// Exited with error.
		{
			inCommand:  "exit 255",
			inError:    fmt.Errorf("unknown error"),
			outCommand: "exit 255",
			outCode:    strconv.Itoa(teleport.RemoteCommandFailure),
		},
		// Command injection.
		{
			inCommand:  "/bin/teleport scp --remote-addr=127.0.0.1:50862 --local-addr=127.0.0.1:54895 -f ~/file.txt && touch /tmp/new.txt",
			inError:    fmt.Errorf("unknown error"),
			outCommand: "/bin/teleport scp --remote-addr=127.0.0.1:50862 --local-addr=127.0.0.1:54895 -f ~/file.txt && touch /tmp/new.txt",
			outCode:    strconv.Itoa(teleport.RemoteCommandFailure),
		},
	}
	for _, tt := range tests {
		emitExecAuditEvent(s.ctx, tt.inCommand, tt.inError)
		execEvent := fakeServer.LastEvent().(*apievents.Exec)
		c.Assert(execEvent.Command, check.Equals, tt.outCommand)
		c.Assert(execEvent.ExitCode, check.Equals, tt.outCode)
	}
}

// TestContinue tests if the process hangs if a continue signal is not sent
// and makes sure the process continues once it has been sent.
func (s *ExecSuite) TestContinue(c *check.C) {
	var err error

	lsPath, err := os_exec.LookPath("ls")
	c.Assert(err, check.IsNil)

	// Create a fake context that will be used to configure a command that will
	// re-exec "ls".
	ctx := &ServerContext{
		ConnectionContext: &sshutils.ConnectionContext{},
		IsTestStub:        true,
		srv: &fakeServer{
			accessPoint: s.a,
			auditLog:    &fakeLog{},
			id:          "00000000-0000-0000-0000-000000000000",
		},
	}
	ctx.Identity.Login = s.usr.Username
	ctx.Identity.TeleportUser = "galt"
	ctx.ServerConn = &ssh.ServerConn{Conn: s}
	ctx.ExecRequest = &localExec{
		Ctx:     ctx,
		Command: lsPath,
	}
	ctx.cmdr, ctx.cmdw, err = os.Pipe()
	c.Assert(err, check.IsNil)
	ctx.contr, ctx.contw, err = os.Pipe()
	c.Assert(err, check.IsNil)
	ctx.request = &ssh.Request{
		Type: sshutils.ExecRequest,
	}

	// Create an exec.Cmd to execute through Teleport.
	cmd, err := ConfigureCommand(ctx)
	c.Assert(err, check.IsNil)

	// Create a channel that will be used to signal that execution is complete.
	cmdDone := make(chan error, 1)

	// Re-execute Teleport and run "ls". Signal over the context when execution
	// is complete.
	go func() {
		cmdDone <- cmd.Run()
	}()

	// Wait for the process. Since the continue pipe has not been closed, the
	// process should not have exited yet.
	select {
	case err := <-cmdDone:
		c.Fatalf("Process exited before continue with error %v", err)
	case <-time.After(5 * time.Second):
	}

	// Close the continue pipe to signal to Teleport to now execute the
	// requested program.
	err = ctx.contw.Close()
	c.Assert(err, check.IsNil)

	// Program should have executed now. If the complete signal has not come
	// over the context, something failed.
	select {
	case <-time.After(5 * time.Second):
		c.Fatalf("Timed out waiting for process to finish.")
	case err := <-cmdDone:
		c.Assert(err, check.IsNil)
	}
}

// Implementation of ssh.Conn interface.
func (s *ExecSuite) User() string                                           { return s.usr.Username }
func (s *ExecSuite) SessionID() []byte                                      { return []byte{1, 2, 3} }
func (s *ExecSuite) ClientVersion() []byte                                  { return []byte{1} }
func (s *ExecSuite) ServerVersion() []byte                                  { return []byte{1} }
func (s *ExecSuite) RemoteAddr() net.Addr                                   { return s.remoteAddr }
func (s *ExecSuite) LocalAddr() net.Addr                                    { return s.localAddr }
func (s *ExecSuite) Close() error                                           { return nil }
func (s *ExecSuite) SendRequest(string, bool, []byte) (bool, []byte, error) { return false, nil, nil }
func (s *ExecSuite) OpenChannel(string, []byte) (ssh.Channel, <-chan *ssh.Request, error) {
	return nil, nil, nil
}
func (s *ExecSuite) Wait() error { return nil }

type fakeTerminal struct {
	f *os.File
}

// AddParty adds another participant to this terminal. We will keep the
// Terminal open until all participants have left.
func (f *fakeTerminal) AddParty(delta int) {
}

// Run will run the terminal.
func (f *fakeTerminal) Run() error {
	return nil
}

// Wait will block until the terminal is complete.
func (f *fakeTerminal) Wait() (*ExecResult, error) {
	return nil, nil
}

// Continue will resume execution of the process after it completes its
// pre-processing routine (placed in a cgroup).
func (f *fakeTerminal) Continue() {}

// Kill will force kill the terminal.
func (f *fakeTerminal) Kill() error {
	return nil
}

// PTY returns the PTY backing the terminal.
func (f *fakeTerminal) PTY() io.ReadWriter {
	return nil
}

// TTY returns the TTY backing the terminal.
func (f *fakeTerminal) TTY() *os.File {
	return f.f
}

// PID returns the PID of the Teleport process that was re-execed.
func (f *fakeTerminal) PID() int {
	return 1
}

// Close will free resources associated with the terminal.
func (f *fakeTerminal) Close() error {
	return f.f.Close()
}

// GetWinSize returns the window size of the terminal.
func (f *fakeTerminal) GetWinSize() (*term.Winsize, error) {
	return &term.Winsize{}, nil
}

// SetWinSize sets the window size of the terminal.
func (f *fakeTerminal) SetWinSize(params rsession.TerminalParams) error {
	return nil
}

// GetTerminalParams is a fast call to get cached terminal parameters
// and avoid extra system call.
func (f *fakeTerminal) GetTerminalParams() rsession.TerminalParams {
	return rsession.TerminalParams{}
}

// SetTerminalModes sets the terminal modes from "pty-req"
func (f *fakeTerminal) SetTerminalModes(ssh.TerminalModes) {}

// GetTermType gets the terminal type set in "pty-req"
func (f *fakeTerminal) GetTermType() string {
	return "xterm"
}

// SetTermType sets the terminal type from "pty-req"
func (f *fakeTerminal) SetTermType(string) {
}

// fakeServer is stub for tests
type fakeServer struct {
	auditLog events.IAuditLog
	events.MockEmitter
	accessPoint auth.AccessPoint
	id          string
}

func (f *fakeServer) Context() context.Context {
	return context.TODO()
}

func (f *fakeServer) ID() string {
	return f.id
}

func (f *fakeServer) HostUUID() string {
	return f.id
}

func (f *fakeServer) GetNamespace() string {
	return ""
}

func (f *fakeServer) AdvertiseAddr() string {
	return ""
}

func (f *fakeServer) Component() string {
	return ""
}

func (f *fakeServer) PermitUserEnvironment() bool {
	return true
}

func (f *fakeServer) GetAccessPoint() auth.AccessPoint {
	return f.accessPoint
}

func (f *fakeServer) GetSessionServer() rsession.Service {
	return nil
}

func (f *fakeServer) GetDataDir() string {
	return ""
}

func (f *fakeServer) GetPAM() (*pam.Config, error) {
	return nil, nil
}

func (f *fakeServer) GetClock() clockwork.Clock {
	return nil
}

func (f *fakeServer) GetInfo() types.Server {
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

func (f *fakeServer) GetUtmpPath() (string, string) {
	return "", ""
}

func (f *fakeServer) UseTunnel() bool {
	return false
}

func (f *fakeServer) GetBPF() bpf.BPF {
	return &bpf.NOP{}
}

func (f *fakeServer) GetRestrictedSessionManager() restricted.Manager {
	return &restricted.NOP{}
}

func (f *fakeServer) GetLockWatcher() *services.LockWatcher {
	return nil
}

// fakeLog is used in tests to obtain the last event emit to the Audit Log.
type fakeLog struct {
}

func (a *fakeLog) EmitAuditEventLegacy(e events.Event, f events.EventFields) error {
	return trace.NotImplemented("not implemented")
}

func (a *fakeLog) EmitAuditEvent(ctx context.Context, e apievents.AuditEvent) error {
	return trace.NotImplemented("not implemented")
}

func (a *fakeLog) PostSessionSlice(s events.SessionSlice) error {
	return trace.NotImplemented("not implemented")
}

func (a *fakeLog) UploadSessionRecording(r events.SessionRecording) error {
	return trace.NotImplemented("not implemented")
}

func (a *fakeLog) GetSessionChunk(namespace string, sid rsession.ID, offsetBytes int, maxBytes int) ([]byte, error) {
	return nil, trace.NotFound("")
}

func (a *fakeLog) GetSessionEvents(namespace string, sid rsession.ID, after int, includePrintEvents bool) ([]events.EventFields, error) {
	return nil, trace.NotFound("")
}

func (a *fakeLog) SearchEvents(fromUTC, toUTC time.Time, namespace string, eventTypes []string, limit int, order types.EventOrder, startKey string) ([]apievents.AuditEvent, string, error) {
	return nil, "", trace.NotFound("")
}

func (a *fakeLog) SearchSessionEvents(fromUTC, toUTC time.Time, limit int, order types.EventOrder, startKey string, cond *types.WhereExpr) ([]apievents.AuditEvent, string, error) {
	return nil, "", trace.NotFound("")
}

func (a *fakeLog) StreamSessionEvents(ctx context.Context, sessionID rsession.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	c, e := make(chan apievents.AuditEvent), make(chan error, 1)
	e <- trace.NotImplemented("not implemented")
	return c, e
}

func (a *fakeLog) WaitForDelivery(context.Context) error {
	return trace.NotImplemented("not implemented")
}

func (a *fakeLog) Close() error {
	return trace.NotFound("")
}
