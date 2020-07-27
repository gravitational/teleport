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
	"context"
	"fmt"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"gopkg.in/check.v1"
)

func TestSSHUtils(t *testing.T) { check.TestingT(t) }

type ServerSuite struct {
	signer ssh.Signer
}

var _ = fmt.Printf
var _ = check.Suite(&ServerSuite{})

func (s *ServerSuite) SetUpSuite(c *check.C) {
	var err error

	utils.InitLoggerForTests()

	_, s.signer, err = utils.CreateCertificate("foo", ssh.HostCert)
	c.Assert(err, check.IsNil)
}

func (s *ServerSuite) TestStartStop(c *check.C) {
	called := false
	fn := NewChanHandlerFunc(func(_ context.Context, _ *ConnectionContext, nch ssh.NewChannel) {
		called = true
		err := nch.Reject(ssh.Prohibited, "nothing to see here")
		c.Assert(err, check.IsNil)
	})

	srv, err := NewServer(
		"test",
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		fn,
		[]ssh.Signer{s.signer},
		AuthMethods{Password: pass("abc123")},
	)
	c.Assert(err, check.IsNil)
	c.Assert(srv.Start(), check.IsNil)

	clientConfig := &ssh.ClientConfig{
		Auth:            []ssh.AuthMethod{ssh.Password("abc123")},
		HostKeyCallback: ssh.FixedHostKey(s.signer.PublicKey()),
	}
	clt, err := ssh.Dial("tcp", srv.Addr(), clientConfig)
	c.Assert(err, check.IsNil)
	defer clt.Close()

	// Call new session to initiate opening new channel. This should get
	// rejected and fail.
	_, err = clt.NewSession()
	c.Assert(err, check.NotNil)

	c.Assert(srv.Close(), check.IsNil)
	wait(c, srv)
	c.Assert(called, check.Equals, true)
}

// TestShutdown tests graceul shutdown feature
func (s *ServerSuite) TestShutdown(c *check.C) {
	closeContext, cancel := context.WithCancel(context.TODO())
	fn := NewChanHandlerFunc(func(_ context.Context, ccx *ConnectionContext, nch ssh.NewChannel) {
		ch, _, err := nch.Accept()
		c.Assert(err, check.IsNil)
		defer ch.Close()

		<-closeContext.Done()
		ccx.ServerConn.Close()
	})

	srv, err := NewServer(
		"test",
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		fn,
		[]ssh.Signer{s.signer},
		AuthMethods{Password: pass("abc123")},
		SetShutdownPollPeriod(10*time.Millisecond),
	)
	c.Assert(err, check.IsNil)
	c.Assert(srv.Start(), check.IsNil)

	clientConfig := &ssh.ClientConfig{
		Auth:            []ssh.AuthMethod{ssh.Password("abc123")},
		HostKeyCallback: ssh.FixedHostKey(s.signer.PublicKey()),
	}
	clt, err := ssh.Dial("tcp", srv.Addr(), clientConfig)
	c.Assert(err, check.IsNil)
	defer clt.Close()

	// call new session to initiate opening new channel
	_, err = clt.NewSession()
	c.Assert(err, check.IsNil)

	// context will timeout because there is a connection around
	ctx, ctxc := context.WithTimeout(context.TODO(), 50*time.Millisecond)
	defer ctxc()
	c.Assert(trace.IsConnectionProblem(srv.Shutdown(ctx)), check.Equals, true)

	// now shutdown will return
	cancel()
	ctx2, ctxc2 := context.WithTimeout(context.TODO(), time.Second)
	defer ctxc2()
	c.Assert(srv.Shutdown(ctx2), check.IsNil)

	// shutdown is re-entrable
	ctx3, ctxc3 := context.WithTimeout(context.TODO(), time.Second)
	defer ctxc3()
	c.Assert(srv.Shutdown(ctx3), check.IsNil)
}

func (s *ServerSuite) TestConfigureCiphers(c *check.C) {
	fn := NewChanHandlerFunc(func(_ context.Context, _ *ConnectionContext, nch ssh.NewChannel) {
		err := nch.Reject(ssh.Prohibited, "nothing to see here")
		c.Assert(err, check.IsNil)
	})

	// create a server that only speaks aes128-ctr
	srv, err := NewServer(
		"test",
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		fn,
		[]ssh.Signer{s.signer},
		AuthMethods{Password: pass("abc123")},
		SetCiphers([]string{"aes128-ctr"}),
	)
	c.Assert(err, check.IsNil)
	c.Assert(srv.Start(), check.IsNil)

	// client only speaks aes256-ctr, should fail
	cc := ssh.ClientConfig{
		Config: ssh.Config{
			Ciphers: []string{"aes256-ctr"},
		},
		Auth:            []ssh.AuthMethod{ssh.Password("abc123")},
		HostKeyCallback: ssh.FixedHostKey(s.signer.PublicKey()),
	}
	_, err = ssh.Dial("tcp", srv.Addr(), &cc)
	c.Assert(err, check.NotNil, check.Commentf("cipher mismatch, should fail, got nil"))

	// client only speaks aes128-ctr, should succeed
	cc = ssh.ClientConfig{
		Config: ssh.Config{
			Ciphers: []string{"aes128-ctr"},
		},
		Auth:            []ssh.AuthMethod{ssh.Password("abc123")},
		HostKeyCallback: ssh.FixedHostKey(s.signer.PublicKey()),
	}
	clt, err := ssh.Dial("tcp", srv.Addr(), &cc)
	c.Assert(err, check.IsNil, check.Commentf("cipher match, should not fail, got error: %v", err))
	defer clt.Close()
}

// TestHostSigner makes sure Teleport can not be started with a invalid host
// certificate. The main check is the certificate algorithms.
func (s *ServerSuite) TestHostSignerFIPS(c *check.C) {
	_, ellipticSigner, err := utils.CreateEllipticCertificate("foo", ssh.HostCert)
	c.Assert(err, check.IsNil)

	newChanHandler := NewChanHandlerFunc(func(_ context.Context, _ *ConnectionContext, nch ssh.NewChannel) {
		err := nch.Reject(ssh.Prohibited, "nothing to see here")
		c.Assert(err, check.IsNil)
	})

	var tests = []struct {
		inSigner ssh.Signer
		inFIPS   bool
		outError bool
	}{
		// ECDSA when in FIPS mode should fail.
		{
			inSigner: ellipticSigner,
			inFIPS:   true,
			outError: true,
		},
		// RSA when in FIPS mode is okay.
		{
			inSigner: s.signer,
			inFIPS:   true,
			outError: false,
		},
		// ECDSA when in not FIPS mode should succeed.
		{
			inSigner: ellipticSigner,
			inFIPS:   false,
			outError: false,
		},
		// RSA when in not FIPS mode should succeed.
		{
			inSigner: s.signer,
			inFIPS:   false,
			outError: false,
		},
	}
	for _, tt := range tests {
		_, err := NewServer(
			"test",
			utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
			newChanHandler,
			[]ssh.Signer{tt.inSigner},
			AuthMethods{Password: pass("abc123")},
			SetCiphers([]string{"aes128-ctr"}),
			SetFIPS(tt.inFIPS),
		)
		c.Assert(err != nil, check.Equals, tt.outError)
	}
}

func wait(c *check.C, srv *Server) {
	s := make(chan struct{})
	go func() {
		srv.Wait(context.TODO())
		s <- struct{}{}
	}()
	select {
	case <-time.After(time.Second):
		c.Assert(false, check.Equals, true, check.Commentf("exceeded waiting timeout"))
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
