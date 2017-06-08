/*
Copyright 2015 Gravitational, Inc.

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
	"fmt"
	"net"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/check.v1"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/auth"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/utils"
)

// ExecSuite also implements ssh.ConnMetadata
type ExecSuite struct {
	usr        *user.User
	ctx        *ctx
	localAddr  net.Addr
	remoteAddr net.Addr
}

var _ = check.Suite(&ExecSuite{})
var _ = fmt.Printf

func (s *ExecSuite) SetUpSuite(c *check.C) {
	bk, err := boltbk.New(backend.Params{"path": c.MkDir()})
	c.Assert(err, check.IsNil)

	a := auth.NewAuthServer(&auth.InitConfig{
		Backend:    bk,
		Authority:  authority.New(),
		DomainName: "localhost",
	})

	utils.InitLoggerForTests()
	s.usr, _ = user.Current()
	s.ctx = &ctx{isTestStub: true}
	s.ctx.login = s.usr.Username
	s.ctx.session = &session{id: "xxx"}
	s.ctx.teleportUser = "galt"
	s.ctx.conn = &ssh.ServerConn{Conn: s}
	s.ctx.exec = &execResponse{ctx: s.ctx}
	s.ctx.srv = &Server{authService: a, uuid: "00000000-0000-0000-0000-000000000000"}
	s.localAddr, _ = utils.ParseAddr("127.0.0.1:3022")
	s.remoteAddr, _ = utils.ParseAddr("10.0.0.5:4817")
}

func (s *ExecSuite) TestOSCommandPrep(c *check.C) {
	expectedEnv := []string{
		"LANG=en_US.UTF-8",
		getDefaultEnvPath(""),
		fmt.Sprintf("HOME=%s", s.usr.HomeDir),
		fmt.Sprintf("USER=%s", s.usr.Username),
		"SHELL=/bin/sh",
		"SSH_TELEPORT_USER=galt",
		"SSH_SESSION_WEBPROXY_ADDR=<proxyhost>:3080",
		"SSH_TELEPORT_HOST_UUID=00000000-0000-0000-0000-000000000000",
		"SSH_TELEPORT_CLUSTER_NAME=localhost",
		"TERM=xterm",
		"SSH_CLIENT=10.0.0.5 4817 3022",
		"SSH_CONNECTION=10.0.0.5 4817 127.0.0.1 3022",
		"SSH_SESSION_ID=xxx",
	}

	// empty command (simple shell)
	cmd, err := prepInteractiveCommand(s.ctx)
	c.Assert(err, check.IsNil)
	c.Assert(cmd, check.NotNil)
	c.Assert(cmd.Path, check.Equals, "/bin/sh")
	c.Assert(cmd.Args, check.DeepEquals, []string{"-sh"})
	c.Assert(cmd.Dir, check.Equals, s.usr.HomeDir)
	c.Assert(cmd.Env, check.DeepEquals, expectedEnv)

	// non-empty command (exec a prog)
	s.ctx.isTestStub = true
	s.ctx.exec.cmdName = "ls -lh /etc"
	cmd, err = prepareCommand(s.ctx)
	c.Assert(err, check.IsNil)
	c.Assert(cmd, check.NotNil)
	c.Assert(cmd.Path, check.Equals, "/bin/sh")
	c.Assert(cmd.Args, check.DeepEquals, []string{"/bin/sh", "-c", "ls -lh /etc"})
	c.Assert(cmd.Dir, check.Equals, s.usr.HomeDir)
	c.Assert(cmd.Env, check.DeepEquals, expectedEnv)

	// command without args
	s.ctx.exec.cmdName = "top"
	cmd, err = prepareCommand(s.ctx)
	c.Assert(err, check.IsNil)
	c.Assert(cmd.Path, check.Equals, "/bin/sh")
	c.Assert(cmd.Args, check.DeepEquals, []string{"/bin/sh", "-c", "top"})
}

func (s *ExecSuite) TestLoginDefsParser(c *check.C) {
	c.Assert(getDefaultEnvPath("../../fixtures/login.defs"), check.Equals, "PATH=/usr/local/bin:/usr/bin:/bin:/foo")
	c.Assert(getDefaultEnvPath("bad/file"), check.Equals, "PATH="+defaultPath)
}

func (s *ExecSuite) TestShellEscaping(c *check.C) {
	c.Assert(strings.Join(quoteShellWildcards([]string{"one", "two"}), " "),
		check.Equals,
		"one two")
	c.Assert(strings.Join(quoteShellWildcards([]string{"o*ne", "two?"}), " "),
		check.Equals,
		"'o*ne' 'two?'")
	c.Assert(strings.Join(quoteShellWildcards([]string{"'o*ne'", "'two?"}), " "),
		check.Equals,
		"'o*ne' ''two?'")
}

// implementation of ssh.Conn interface
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

// findExecutable helper finds a given executable name (like 'ls') in $PATH
// and returns the full path
func findExecutable(execName string) string {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		fp := path.Join(dir, execName)
		if utils.IsFile(fp) {
			return fp
		}
	}
	return "not found in $PATH: " + execName
}
