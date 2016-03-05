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

// Package test contains a log backend acceptance test suite that is
// implementation independant each backend will use the suite to test itself
package test

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"

	"github.com/codahale/lunk"
	. "gopkg.in/check.v1"
)

func TestEventLog(t *testing.T) { TestingT(t) }

type EventSuite struct {
	L events.Log
}

func (s *EventSuite) EventsCRUD(c *C) {
	start := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)

	e1 := lunk.NewEntry(lunk.NewRootEventID(),
		&events.AuthAttempt{SessionID: "sid1", User: "bob"})
	e1.Time = start
	c.Assert(s.L.LogEntry(e1), IsNil)

	e2 := lunk.NewEntry(
		lunk.NewRootEventID(),
		&events.AuthAttempt{SessionID: "sid1", User: "alice"})
	e2.Time = start.Add(time.Second)

	c.Assert(s.L.LogEntry(e2), IsNil)

	e3 := lunk.NewEntry(
		lunk.NewRootEventID(),
		&events.AuthAttempt{SessionID: "sid2", User: "bob"})
	e3.Time = start.Add(2 * time.Second)

	c.Assert(s.L.LogEntry(e3), IsNil)

	// get last 2 events
	es, err := s.L.GetEvents(
		events.Filter{
			Start: start.Add(2 * time.Second),
			Order: events.Desc,
			Limit: 2,
		})
	c.Assert(err, IsNil)
	c.Assert(e2p(es...), DeepEquals, e2p(e3, e2))

	// get last 2 events for session sid1
	es, err = s.L.GetEvents(
		events.Filter{
			Start:     start.Add(2 * time.Second),
			Order:     events.Desc,
			Limit:     2,
			SessionID: "sid1",
		})
	c.Assert(err, IsNil)
	c.Assert(e2p(es...), DeepEquals, e2p(e2, e1))

	// get events in range from start to end
	es, err = s.L.GetEvents(
		events.Filter{Start: start, End: start.Add(time.Second)})
	c.Assert(err, IsNil)
	c.Assert(e2p(es...), DeepEquals, e2p(e1, e2))
}

func (s *EventSuite) SessionsCRUD(c *C) {
	start := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)

	sess1 := session.Session{
		ID:         "sid1",
		LastActive: start,
	}
	c.Assert(s.L.LogSession(sess1), IsNil)

	sess2 := session.Session{
		ID:         "sid2",
		LastActive: start.Add(time.Second),
	}
	c.Assert(s.L.LogSession(sess2), IsNil)

	sess3 := session.Session{
		ID:         "sid3",
		LastActive: start.Add(2 * time.Second),
	}
	c.Assert(s.L.LogSession(sess3), IsNil)

	// get last 2 sessions
	sessions, err := s.L.GetSessionEvents(
		events.Filter{
			Start: start.Add(2 * time.Second),
			Order: events.Desc,
			Limit: 2,
		})
	c.Assert(err, IsNil)
	c.Assert(s2p(sessions...), DeepEquals, s2p(sess3, sess2))

	// get events in range from start to end
	sessions, err = s.L.GetSessionEvents(
		events.Filter{Start: start, End: start.Add(time.Second)})
	c.Assert(err, IsNil)
	c.Assert(s2p(sessions...), DeepEquals, s2p(sess1, sess2))
}

func e2p(es ...lunk.Entry) []map[string]string {
	out := make([]map[string]string, len(es))
	for i, a := range es {
		out[i] = a.Properties
	}
	return out
}

func s2p(sessions ...session.Session) []string {
	out := make([]string, len(sessions))
	for i, sess := range sessions {
		out[i] = sess.ID
	}
	return out
}
