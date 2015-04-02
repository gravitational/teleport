package sshutils

import (
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh/testdata"
	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestSSHUtils(t *testing.T) { TestingT(t) }

type ServerSuite struct {
	signers []ssh.Signer
}

var _ = Suite(&ServerSuite{})

func (s *ServerSuite) SetUpSuite(c *C) {
	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})

	pk, err := ssh.ParsePrivateKey(testdata.PEMBytes["ecdsa"])
	c.Assert(err, IsNil)
	s.signers = []ssh.Signer{pk}
}

func (s *ServerSuite) TestStartStop(c *C) {
	called := false
	fn := NewChanHandlerFunc(func(conn *ssh.ServerConn, nch ssh.NewChannel) {
		called = true
		nch.Reject(ssh.Prohibited, "nothing to see here")
	})
	srv, err := NewServer(
		Addr{"tcp", "localhost:0"},
		fn,
		s.signers,
		AuthMethods{Password: pass("abc123")})
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
