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

package session

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/mailgun/timetools"
	. "gopkg.in/check.v1"
)

func TestSessions(t *testing.T) { TestingT(t) }

type BoltSuite struct {
	dir   string
	srv   *server
	bk    *boltbk.BoltBackend
	clock *timetools.FreezedTime
}

var _ = Suite(&BoltSuite{})

func (s *BoltSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

func (s *BoltSuite) SetUpTest(c *C) {
	s.clock = &timetools.FreezedTime{
		CurrentTime: time.Date(2016, 9, 8, 7, 6, 5, 0, time.UTC),
	}
	s.dir = c.MkDir()

	var err error
	s.bk, err = boltbk.New(filepath.Join(s.dir, "db"), boltbk.Clock(s.clock))
	c.Assert(err, IsNil)

	srv, err := New(s.bk, Clock(s.clock))
	s.srv = srv.(*server)
	c.Assert(err, IsNil)
}

func (s *BoltSuite) TearDownTest(c *C) {
	c.Assert(s.bk.Close(), IsNil)
}

func (s *BoltSuite) TestID(c *C) {
	id := NewID()
	id2, err := ParseID(id.String())
	c.Assert(err, IsNil)
	c.Assert(id, Equals, *id2)

	for _, val := range []string{"garbage", "", "   ", string(id) + "extra"} {
		id := ID(val)
		c.Assert(id.Check(), NotNil)
	}
}

func (s *BoltSuite) TestSessionsCRUD(c *C) {
	out, err := s.srv.GetSessions(Filter{})
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	sess := Session{
		ID:             NewID(),
		Active:         true,
		TerminalParams: TerminalParams{W: 100, H: 100},
		Login:          "bob",
		LastActive:     s.clock.UtcNow(),
		Created:        s.clock.UtcNow(),
	}
	c.Assert(s.srv.CreateSession(sess), IsNil)

	out, err = s.srv.GetSessions(Filter{})
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []Session{sess})

	s2, err := s.srv.GetSession(sess.ID)
	c.Assert(err, IsNil)
	c.Assert(s2, DeepEquals, &sess)

	// Mark session inactive
	err = s.srv.UpdateSession(UpdateRequest{
		ID:     sess.ID,
		Active: Bool(false),
	})
	c.Assert(err, IsNil)

	sess.Active = false
	s2, err = s.srv.GetSession(sess.ID)
	c.Assert(err, IsNil)
	c.Assert(s2, DeepEquals, &sess)

	// Update session terminal parameter
	err = s.srv.UpdateSession(UpdateRequest{
		ID:             sess.ID,
		TerminalParams: &TerminalParams{W: 101, H: 101},
	})
	c.Assert(err, IsNil)

	sess.TerminalParams = TerminalParams{W: 101, H: 101}
	s2, err = s.srv.GetSession(sess.ID)
	c.Assert(err, IsNil)
	c.Assert(s2, DeepEquals, &sess)

	// ask for active-only sessions, should get nothing
	out, _ = s.srv.GetSessions(Filter{State: SessionStateActive})
	c.Assert(out, HasLen, 0)

	// create another active session (created 24 hours ago)
	sid2 := NewID()
	utcnow := time.Now().In(time.UTC)
	c.Assert(s.srv.CreateSession(Session{
		ID:             sid2,
		Active:         true,
		TerminalParams: TerminalParams{W: 100, H: 100},
		Login:          "bob",
		LastActive:     utcnow.Add(-time.Hour * 24),
		Created:        utcnow.Add(-time.Hour * 24),
	}), IsNil)

	// ask for active-only sessions, should get just one
	out, _ = s.srv.GetSessions(Filter{State: SessionStateActive})
	c.Assert(out, HasLen, 1)

	// ask for active-only sessions created no earlier than 5 minutes ago, should get nothing
	out, _ = s.srv.GetSessions(Filter{State: SessionStateActive, Start: utcnow.Add(-time.Minute * 5)})
	c.Assert(out, HasLen, 0)

	// ask for any sessions, should get two
	out, _ = s.srv.GetSessions(Filter{State: SessionStateAny})
	c.Assert(out, HasLen, 2)
}

// TestSessionsInactivity makes sure that session will be marked
// as inactive after period of inactivity
func (s *BoltSuite) TestSessionsInactivity(c *C) {
	sess := Session{
		ID:             NewID(),
		Active:         true,
		TerminalParams: TerminalParams{W: 100, H: 100},
		Login:          "bob",
		LastActive:     s.clock.UtcNow(),
		Created:        s.clock.UtcNow(),
	}
	c.Assert(s.srv.CreateSession(sess), IsNil)

	s.clock.Sleep(defaults.ActiveSessionTTL + time.Second)

	sess.Active = false
	s2, err := s.srv.GetSession(sess.ID)
	c.Assert(err, IsNil)
	c.Assert(s2, DeepEquals, &sess)
}

func (s *BoltSuite) TestPartiesCRUD(c *C) {
	sess := Session{
		ID:             NewID(),
		Active:         true,
		TerminalParams: TerminalParams{W: 100, H: 100},
		Login:          "bob",
		LastActive:     s.clock.UtcNow(),
		Created:        s.clock.UtcNow(),
	}
	c.Assert(s.srv.CreateSession(sess), IsNil)

	p1 := Party{
		ID:         NewID(),
		User:       "bob",
		RemoteAddr: "example.com",
		ServerID:   "id-1",
		LastActive: s.clock.UtcNow(),
	}
	c.Assert(s.srv.UpsertParty(sess.ID, p1, defaults.ActivePartyTTL), IsNil)

	out, err := s.srv.GetSession(sess.ID)
	c.Assert(err, IsNil)
	sess.Parties = []Party{p1}
	c.Assert(out, DeepEquals, &sess)

	// add one more party
	p2 := Party{
		ID:         NewID(),
		User:       "alice",
		RemoteAddr: "example.com",
		ServerID:   "id-2",
		LastActive: s.clock.UtcNow(),
	}
	c.Assert(s.srv.UpsertParty(sess.ID, p2, defaults.ActivePartyTTL), IsNil)

	out, err = s.srv.GetSession(sess.ID)
	c.Assert(err, IsNil)
	sess.Parties = []Party{p1, p2}
	c.Assert(out, DeepEquals, &sess)

	// Update session party
	s.clock.Sleep(time.Second)
	p1.LastActive = s.clock.UtcNow()
	c.Assert(s.srv.UpsertParty(sess.ID, p1, defaults.ActivePartyTTL), IsNil)

	out, err = s.srv.GetSession(sess.ID)
	c.Assert(err, IsNil)
	sess.Parties = []Party{p1, p2}
	c.Assert(out, DeepEquals, &sess)
}
