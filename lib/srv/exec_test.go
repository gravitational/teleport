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

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/auth"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

// ExecSuite also implements ssh.ConnMetadata
type ExecSuite struct {
	usr        *user.User
	ctx        *ServerContext
	localAddr  net.Addr
	remoteAddr net.Addr
}

var _ = check.Suite(&ExecSuite{})
var _ = fmt.Printf

func (s *ExecSuite) SetUpSuite(c *check.C) {
	bk, err := boltbk.New(backend.Params{"path": c.MkDir()})
	c.Assert(err, check.IsNil)

	c.Assert(err, check.IsNil)
	a := auth.NewAuthServer(&auth.InitConfig{
		Backend:   bk,
		Authority: authority.New(),
	})

	// set cluster name
	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: "localhost",
	})
	c.Assert(err, check.IsNil)
	err = a.SetClusterName(clusterName)
	c.Assert(err, check.IsNil)

	// set static tokens
	staticTokens, err := services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: []services.ProvisionToken{},
	})
	c.Assert(err, check.IsNil)
	err = a.SetStaticTokens(staticTokens)
	c.Assert(err, check.IsNil)

	utils.InitLoggerForTests()
	s.usr, _ = user.Current()
	s.ctx = &ServerContext{IsTestStub: true}
	s.ctx.Identity.Login = s.usr.Username
	s.ctx.session = &session{id: "xxx"}
	s.ctx.Identity.TeleportUser = "galt"
	s.ctx.Conn = &ssh.ServerConn{Conn: s}
	s.ctx.ExecRequest = &localExec{Ctx: s.ctx}
	s.localAddr, _ = utils.ParseAddr("127.0.0.1:3022")
	s.remoteAddr, _ = utils.ParseAddr("10.0.0.5:4817")
}

func (s *ExecSuite) TestOSCommandPrep(c *check.C) {
	expectedEnv := []string{
		"LANG=en_US.UTF-8",
		getDefaultEnvPath("1000", defaultLoginDefsPath),
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
	cmd, err := prepareInteractiveCommand(s.ctx)
	c.Assert(err, check.IsNil)
	c.Assert(cmd, check.NotNil)
	c.Assert(cmd.Path, check.Equals, "/bin/sh")
	c.Assert(cmd.Args, check.DeepEquals, []string{"-sh"})
	c.Assert(cmd.Dir, check.Equals, s.usr.HomeDir)
	c.Assert(cmd.Env, check.DeepEquals, expectedEnv)

	// non-empty command (exec a prog)
	s.ctx.IsTestStub = true
	s.ctx.ExecRequest.SetCommand("ls -lh /etc")
	cmd, err = prepareCommand(s.ctx)
	c.Assert(err, check.IsNil)
	c.Assert(cmd, check.NotNil)
	c.Assert(cmd.Path, check.Equals, "/bin/sh")
	c.Assert(cmd.Args, check.DeepEquals, []string{"/bin/sh", "-c", "ls -lh /etc"})
	c.Assert(cmd.Dir, check.Equals, s.usr.HomeDir)
	c.Assert(cmd.Env, check.DeepEquals, expectedEnv)

	// command without args
	s.ctx.ExecRequest.SetCommand("top")
	cmd, err = prepareCommand(s.ctx)
	c.Assert(err, check.IsNil)
	c.Assert(cmd.Path, check.Equals, "/bin/sh")
	c.Assert(cmd.Args, check.DeepEquals, []string{"/bin/sh", "-c", "top"})
}

func (s *ExecSuite) TestLoginDefsParser(c *check.C) {
	expectedEnvSuPath := "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/bar"
	expectedSuPath := "PATH=/usr/local/bin:/usr/bin:/bin:/foo"

	c.Assert(getDefaultEnvPath("0", "../../fixtures/login.defs"), check.Equals, expectedEnvSuPath)
	c.Assert(getDefaultEnvPath("1000", "../../fixtures/login.defs"), check.Equals, expectedSuPath)
	c.Assert(getDefaultEnvPath("1000", "bad/file"), check.Equals, defaultEnvPath)
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
