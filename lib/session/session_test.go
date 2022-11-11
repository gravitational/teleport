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
	"context"
	"os"
	"testing"
	"time"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/trace"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestSessions(t *testing.T) {
	s := newsessionSuite(t)
	t.Cleanup(func() { s.TearDown(t) })

	t.Run("TestID", s.TestID)
	t.Run("TestSessionsCRUD", s.TestSessionsCRUD)
	t.Run("TestSessionsInactivity", s.TestSessionsInactivity)
	t.Run("TestPartiesCRUD", s.TestPartiesCRUD)
}

type sessionSuite struct {
	dir   string
	srv   *server
	bk    backend.Backend
	clock clockwork.FakeClock
}

func newsessionSuite(t *testing.T) *sessionSuite {
	var err error
	s := &sessionSuite{}

	s.clock = clockwork.NewFakeClockAt(time.Date(2016, 9, 8, 7, 6, 5, 0, time.UTC))
	s.dir = t.TempDir()
	s.bk, err = lite.NewWithConfig(context.TODO(),
		lite.Config{
			Path:  s.dir,
			Clock: s.clock,
		},
	)
	require.NoError(t, err)

	srv, err := New(s.bk)
	require.NoError(t, err)
	srv.(*server).clock = s.clock
	s.srv = srv.(*server)
	return s
}

func (s *sessionSuite) TearDown(t *testing.T) {
	require.NoError(t, s.bk.Close())
}

func (s *sessionSuite) TestID(t *testing.T) {
	id := NewID()
	id2, err := ParseID(id.String())
	require.NoError(t, err)
	require.Equal(t, id, *id2)

	for _, val := range []string{"garbage", "", "   ", string(id) + "extra"} {
		id := ID(val)
		require.Error(t, id.Check())
	}
}

func (s *sessionSuite) TestSessionsCRUD(t *testing.T) {
	ctx := context.Background()
	out, err := s.srv.GetSessions(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, out)

	// Create session.
	sess := Session{
		ID:             NewID(),
		Namespace:      apidefaults.Namespace,
		TerminalParams: TerminalParams{W: 100, H: 100},
		Login:          "bob",
		LastActive:     s.clock.Now().UTC(),
		Created:        s.clock.Now().UTC(),
	}
	require.NoError(t, s.srv.CreateSession(ctx, sess))

	// Make sure only one session exists.
	out, err = s.srv.GetSessions(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Equal(t, out, []Session{sess})

	// Make sure the session is the one created above.
	s2, err := s.srv.GetSession(ctx, apidefaults.Namespace, sess.ID)
	require.NoError(t, err)
	require.Equal(t, s2, &sess)

	// Update session terminal parameter
	err = s.srv.UpdateSession(ctx, UpdateRequest{
		ID:             sess.ID,
		Namespace:      apidefaults.Namespace,
		TerminalParams: &TerminalParams{W: 101, H: 101},
	})
	require.NoError(t, err)

	// Verify update was applied.
	sess.TerminalParams = TerminalParams{W: 101, H: 101}
	s2, err = s.srv.GetSession(ctx, apidefaults.Namespace, sess.ID)
	require.NoError(t, err)
	require.Equal(t, s2, &sess)

	// Remove the session.
	err = s.srv.DeleteSession(ctx, apidefaults.Namespace, sess.ID)
	require.NoError(t, err)

	// Make sure session no longer exists.
	_, err = s.srv.GetSession(ctx, apidefaults.Namespace, sess.ID)
	require.Error(t, err)
}

// TestSessionsInactivity makes sure that session will be marked
// as inactive after period of inactivity
func (s *sessionSuite) TestSessionsInactivity(t *testing.T) {
	ctx := context.Background()
	sess := Session{
		ID:             NewID(),
		Namespace:      apidefaults.Namespace,
		TerminalParams: TerminalParams{W: 100, H: 100},
		Login:          "bob",
		LastActive:     s.clock.Now().UTC(),
		Created:        s.clock.Now().UTC(),
	}
	require.NoError(t, s.srv.CreateSession(ctx, sess))

	// move forward in time:
	s.clock.Advance(defaults.ActiveSessionTTL + time.Second)

	// should not be in active sessions:
	s2, err := s.srv.GetSession(ctx, apidefaults.Namespace, sess.ID)
	require.IsType(t, trace.NotFound(""), err)
	require.Nil(t, s2)
}

func (s *sessionSuite) TestPartiesCRUD(t *testing.T) {
	ctx := context.Background()

	// create session:
	sess := Session{
		ID:             NewID(),
		Namespace:      apidefaults.Namespace,
		TerminalParams: TerminalParams{W: 100, H: 100},
		Login:          "vincent",
		LastActive:     s.clock.Now().UTC(),
		Created:        s.clock.Now().UTC(),
	}
	err := s.srv.CreateSession(ctx, sess)
	require.NoError(t, err)
	// add two people:
	parties := []Party{
		{
			ID:         NewID(),
			RemoteAddr: "1_remote_addr",
			User:       "first",
			ServerID:   "luna",
			LastActive: s.clock.Now().UTC(),
		},
		{
			ID:         NewID(),
			RemoteAddr: "2_remote_addr",
			User:       "second",
			ServerID:   "luna",
			LastActive: s.clock.Now().UTC(),
		},
	}
	err = s.srv.UpdateSession(ctx, UpdateRequest{
		ID:        sess.ID,
		Namespace: apidefaults.Namespace,
		Parties:   &parties,
	})
	require.NoError(t, err)
	// verify they're in the session:
	copy, err := s.srv.GetSession(ctx, apidefaults.Namespace, sess.ID)
	require.NoError(t, err)
	require.Len(t, copy.Parties, 2)

	// empty update (list of parties must not change)
	err = s.srv.UpdateSession(ctx, UpdateRequest{ID: sess.ID, Namespace: apidefaults.Namespace})
	require.NoError(t, err)
	copy, _ = s.srv.GetSession(ctx, apidefaults.Namespace, sess.ID)
	require.Len(t, copy.Parties, 2)

	// remove the 2nd party:
	deleted := copy.RemoveParty(parties[1].ID)
	require.True(t, deleted)
	err = s.srv.UpdateSession(ctx, UpdateRequest{ID: copy.ID, Parties: &copy.Parties, Namespace: apidefaults.Namespace})
	require.NoError(t, err)
	copy, _ = s.srv.GetSession(ctx, apidefaults.Namespace, sess.ID)
	require.Len(t, copy.Parties, 1)

	// we still have the 1st party in:
	require.Equal(t, parties[0].ID, copy.Parties[0].ID)
}
