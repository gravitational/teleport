package session

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/backend/boltbk"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestSessions(t *testing.T) { TestingT(t) }

type BoltSuite struct {
	bk  *boltbk.BoltBackend
	dir string
	srv SessionServer
}

var _ = Suite(&BoltSuite{})

func (s *BoltSuite) SetUpTest(c *C) {
	s.dir = c.MkDir()

	var err error
	s.bk, err = boltbk.New(filepath.Join(s.dir, "db"))
	c.Assert(err, IsNil)

	s.srv = New(s.bk)
}

func (s *BoltSuite) TearDownTest(c *C) {
	c.Assert(s.bk.Close(), IsNil)
}

func (s *BoltSuite) TestSessionsCRUD(c *C) {
	out, err := s.srv.GetSessions()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []Session{})

	c.Assert(s.srv.UpsertSession("sid1", 10*time.Second), IsNil)

	out, err = s.srv.GetSessions()
	c.Assert(err, IsNil)
	sess := Session{
		ID:      "sid1",
		Parties: []Party{},
	}
	c.Assert(out, DeepEquals, []Session{sess})
}

func (s *BoltSuite) TestPartiesCRUD(c *C) {
	out, err := s.srv.GetSessions()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []Session{})

	p1 := Party{
		ID:         "p1",
		User:       "bob",
		Site:       "example.com",
		ServerAddr: "localhost:1",
		LastActive: time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC),
	}
	c.Assert(s.srv.UpsertParty("s1", p1, 0), IsNil)

	out, err = s.srv.GetSessions()
	c.Assert(err, IsNil)
	sess := Session{
		ID:      "s1",
		Parties: []Party{p1},
	}
	c.Assert(out, DeepEquals, []Session{sess})

	// add one more party
	p2 := Party{
		ID:         "p2",
		User:       "alice",
		Site:       "example.com",
		ServerAddr: "localhost:2",
		LastActive: time.Date(2009, time.November, 10, 23, 1, 0, 0, time.UTC),
	}
	c.Assert(s.srv.UpsertParty("s1", p2, 0), IsNil)

	out, err = s.srv.GetSessions()
	c.Assert(err, IsNil)
	sess = Session{
		ID:      "s1",
		Parties: []Party{p1, p2},
	}
	c.Assert(out, DeepEquals, []Session{sess})

	// Update session party
	p1.LastActive = time.Date(2009, time.November, 10, 23, 4, 0, 0, time.UTC)
	c.Assert(s.srv.UpsertParty("s1", p1, 0), IsNil)
	out, err = s.srv.GetSessions()
	c.Assert(err, IsNil)
	sess = Session{
		ID:      "s1",
		Parties: []Party{p1, p2},
	}
	c.Assert(out, DeepEquals, []Session{sess})

	// Delete session
	c.Assert(s.srv.DeleteSession("s1"), IsNil)
	c.Assert(s.srv.DeleteSession("s1"), FitsTypeOf, &teleport.NotFoundError{})

	_, err = s.srv.GetSession("s1")
	c.Assert(err, FitsTypeOf, &teleport.NotFoundError{})
}
