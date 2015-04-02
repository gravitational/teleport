package session

import (
	"testing"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1" // note that we don't vendor libraries dependencies, only end daemons deps are vendored
)

func TestSession(t *testing.T) { TestingT(t) }

type SessionSuite struct {
	srv *secret.Service
}

var _ = Suite(&SessionSuite{})

func (s *SessionSuite) SetUpSuite(c *C) {
	key, err := secret.NewKey()
	c.Assert(err, IsNil)
	srv, err := secret.New(&secret.Config{KeyBytes: key})
	c.Assert(err, IsNil)
	s.srv = srv
}

func (s *SessionSuite) TestDecodeOK(c *C) {
	p, err := NewID(s.srv)
	c.Assert(err, IsNil)

	pid, err := DecodeSID(p.SID, s.srv)
	c.Assert(err, IsNil)
	c.Assert(string(pid), Equals, string(p.PID))
}

func (s *SessionSuite) TestTamperNotOK(c *C) {
	p, err := NewID(s.srv)
	c.Assert(err, IsNil)

	tc := []SecureID{
		p.SID[:len(p.SID)-1],
		"_" + p.SID,
		"",
		"blabla",
		p.SID + "a",
		p.SID[0:len(p.SID)-1] + "_",
	}

	for _, t := range tc {
		_, err = DecodeSID(t, s.srv)
		c.Assert(err, FitsTypeOf, &MalformedSessionError{})
	}
}
