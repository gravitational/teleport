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
	"os/user"

	"gopkg.in/check.v1"

	"golang.org/x/crypto/ssh"

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

func (s *ExecSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
	s.usr, _ = user.Current()
	s.ctx = &ctx{isTestStub: true}
	s.ctx.login = s.usr.Username
	s.ctx.session = &session{id: "xxx"}
	s.ctx.teleportUser = "galt"
	s.ctx.conn = &ssh.ServerConn{Conn: s}
	s.localAddr, _ = utils.ParseAddr("127.0.0.1:3022")
	s.remoteAddr, _ = utils.ParseAddr("10.0.0.5:4817")
}

func (s *ExecSuite) TestOSCommandPrep(c *check.C) {
	expectedEnv := []string{
		"TERM=xterm",
		"LANG=en_US.UTF-8",
		fmt.Sprintf("HOME=%s", s.usr.HomeDir),
		fmt.Sprintf("USER=%s", s.usr.Username),
		"SHELL=/bin/sh",
		"SSH_TELEPORT_USER=galt",
		"SSH_SESSION_WEBPROXY_ADDR=<proxyhost>:3080",
		"SSH_CLIENT=10.0.0.5 4817 3022",
		"SSH_CONNECTION=10.0.0.5 4817 127.0.0.1 3022",
		"SSH_SESSION_ID=xxx",
	}

	// empty command (simple shell)
	cmd, err := prepareShell(s.ctx)
	c.Assert(err, check.IsNil)
	c.Assert(cmd, check.NotNil)
	c.Assert(cmd.Path, check.Equals, "/bin/sh")
	c.Assert(cmd.Args, check.DeepEquals, []string{"-sh"})
	c.Assert(cmd.Dir, check.Equals, s.usr.HomeDir)
	c.Assert(cmd.Env, check.DeepEquals, expectedEnv)

	// non-empty command (exec a prog)
	s.ctx.isTestStub = true
	cmd, err = prepareCommand(s.ctx, "ls -lh /etc")
	c.Assert(err, check.IsNil)
	c.Assert(cmd, check.NotNil)
	c.Assert(cmd.Path, check.Equals, "/bin/sh")
	c.Assert(cmd.Args, check.DeepEquals, []string{"/bin/sh", "-c", "ls -lh /etc"})
	c.Assert(cmd.Dir, check.Equals, s.usr.HomeDir)
	c.Assert(cmd.Env, check.DeepEquals, expectedEnv)

	// command without args
	cmd, err = prepareCommand(s.ctx, "top")
	c.Assert(err, check.IsNil)
	c.Assert(cmd.Path, check.Equals, "/usr/bin/top")
	c.Assert(cmd.Args, check.DeepEquals, []string{"top"})
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
