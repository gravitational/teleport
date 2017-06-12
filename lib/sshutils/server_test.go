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
package sshutils

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/utils"

	"golang.org/x/crypto/ssh"
	. "gopkg.in/check.v1"
)

func TestSSHUtils(t *testing.T) { TestingT(t) }

type ServerSuite struct {
	signers []ssh.Signer
}

var _ = Suite(&ServerSuite{})

func (s *ServerSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()

	pk, err := ssh.ParsePrivateKey(suite.PEMBytes["ecdsa"])
	c.Assert(err, IsNil)
	s.signers = []ssh.Signer{pk}
}

func (s *ServerSuite) TestStartStop(c *C) {
	called := false
	fn := NewChanHandlerFunc(func(_ net.Conn, conn *ssh.ServerConn, nch ssh.NewChannel) {
		called = true
		nch.Reject(ssh.Prohibited, "nothing to see here")
	})

	srv, err := NewServer(
		"test",
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		fn,
		s.signers,
		AuthMethods{Password: pass("abc123")},
	)
	c.Assert(err, IsNil)
	c.Assert(srv.Start(), IsNil)

	clt, err := ssh.Dial("tcp", srv.Addr(), &ssh.ClientConfig{Auth: []ssh.AuthMethod{ssh.Password("abc123")}})
	c.Assert(err, IsNil)
	defer clt.Close()

	// call new session to initiate opening new channel
	clt.NewSession()

	c.Assert(srv.Close(), IsNil)
	wait(c, srv)
	c.Assert(called, Equals, true)
}

func (s *ServerSuite) TestConfigureCiphers(c *C) {
	called := false
	fn := NewChanHandlerFunc(func(_ net.Conn, conn *ssh.ServerConn, nch ssh.NewChannel) {
		called = true
		nch.Reject(ssh.Prohibited, "nothing to see here")
	})

	// create a server that only speaks aes128-ctr
	srv, err := NewServer(
		"test",
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		fn,
		s.signers,
		AuthMethods{Password: pass("abc123")},
		SetCiphers([]string{"aes128-ctr"}),
	)
	c.Assert(err, IsNil)
	c.Assert(srv.Start(), IsNil)

	// client only speaks aes256-ctr, should fail
	cc := ssh.ClientConfig{
		Config: ssh.Config{
			Ciphers: []string{"aes256-ctr"},
		},
		Auth: []ssh.AuthMethod{ssh.Password("abc123")},
	}
	clt, err := ssh.Dial("tcp", srv.Addr(), &cc)
	c.Assert(err, NotNil, Commentf("cipher mismatch, should fail, got nil"))

	// client only speaks aes128-ctr, should succeed
	cc = ssh.ClientConfig{
		Config: ssh.Config{
			Ciphers: []string{"aes128-ctr"},
		},
		Auth: []ssh.AuthMethod{ssh.Password("abc123")},
	}
	clt, err = ssh.Dial("tcp", srv.Addr(), &cc)
	c.Assert(err, IsNil, Commentf("cipher match, should not fail, got error: %v", err))
	defer clt.Close()
}

func wait(c *C, srv *Server) {
	s := make(chan struct{})
	go func() {
		srv.Wait()
		s <- struct{}{}
	}()
	select {
	case <-time.After(time.Second):
		c.Assert(false, Equals, true, Commentf("exceeded waiting timeout"))
	case <-s:
	}
}

func pass(need string) PasswordFunc {
	return func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if string(password) == need {
			return nil, nil
		}
		return nil, fmt.Errorf("passwords don't match")
	}
}
