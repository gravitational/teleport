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

	"github.com/gravitational/teleport/lib/utils"
	"gopkg.in/check.v1"
	//	"golang.org/x/crypto/ssh"
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
	utils.InitLoggerCLI()
	s.usr, _ = user.Current()
	s.ctx = &ctx{isTestStub: true}
	s.ctx.login = s.usr.Username
	s.ctx.info = s
	s.ctx.session = &session{id: "xxx"}

	s.localAddr, _ = utils.ParseAddr("127.0.0.1:3022")
	s.remoteAddr, _ = utils.ParseAddr("10.0.0.5:4817")
}

func (s *ExecSuite) TestGetShell(c *check.C) {
	shell, err := getLoginShell("root")
	c.Assert(err, check.IsNil)
	c.Assert(shell, check.Equals, "/bin/bash")

	shell, err = getLoginShell("non-existent-user")
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Matches, ".*cannot determine shell for.*")
}

func (s *ExecSuite) TestOSCommandPrep(c *check.C) {
	expectedEnv := []string{
		"TERM=xterm",
		"LANG=en_US.UTF-8",
		fmt.Sprintf("HOME=%s", s.usr.HomeDir),
		fmt.Sprintf("USER=%s", s.usr.Username),
		"SHELL=/bin/sh",
		"SSH_CLIENT=10.0.0.5 4817 3022",
		"SSH_CONNECTION=10.0.0.5 4817 127.0.0.1 3022",
		"SSH_SESSION_ID=xxx",
	}

	// empty command (simple shell)
	cmd, err := prepareOSCommand(s.ctx)
	c.Assert(err, check.IsNil)
	c.Assert(cmd, check.NotNil)
	c.Assert(cmd.Path, check.Equals, "/bin/sh")
	c.Assert(cmd.Args, check.DeepEquals, []string{"-sh"})
	c.Assert(cmd.Dir, check.Equals, s.usr.HomeDir)
	c.Assert(cmd.Env, check.DeepEquals, expectedEnv)

	// non-empty command (exec a prog)
	cmd, err = prepareOSCommand(s.ctx, "ls -lh")
	c.Assert(err, check.IsNil)
	c.Assert(cmd, check.NotNil)
	c.Assert(cmd.Path, check.Equals, "/bin/sh")
	c.Assert(cmd.Args, check.DeepEquals, []string{"-sh", "-c", "ls -lh"})
	c.Assert(cmd.Dir, check.Equals, s.usr.HomeDir)
	c.Assert(cmd.Env, check.DeepEquals, expectedEnv)
}

// implementation of ssh.ConnMetadata interface
func (s *ExecSuite) User() string          { return s.usr.Username }
func (s *ExecSuite) SessionID() []byte     { return []byte{1, 2, 3} }
func (s *ExecSuite) ClientVersion() []byte { return []byte{1} }
func (s *ExecSuite) ServerVersion() []byte { return []byte{1} }
func (s *ExecSuite) RemoteAddr() net.Addr  { return s.remoteAddr }
func (s *ExecSuite) LocalAddr() net.Addr   { return s.localAddr }
