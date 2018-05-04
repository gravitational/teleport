/*
Copyright 2015-2018 Gravitational, Inc.

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

package events

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/check.v1"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events/filesessions"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

type AuditTestSuite struct {
	dataDir string
}

// bootstrap check
func TestAuditLog(t *testing.T) { check.TestingT(t) }

var _ = check.Suite(&AuditTestSuite{})

func (a *AuditTestSuite) TearDownSuite(c *check.C) {
	os.RemoveAll(a.dataDir)
}

// creates a file-based audit log and returns a proper *AuditLog pointer
// instead of the usual IAuditLog interface
func (a *AuditTestSuite) makeLog(c *check.C, dataDir string, recordSessions bool) (*AuditLog, error) {
	alog, err := NewAuditLog(AuditLogConfig{
		DataDir:        dataDir,
		RecordSessions: recordSessions,
		ServerID:       "server1",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return alog, nil
}

func (a *AuditTestSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}

func (a *AuditTestSuite) SetUpTest(c *check.C) {
	a.dataDir = c.MkDir()
}

func (a *AuditTestSuite) TestNew(c *check.C) {
	alog, err := a.makeLog(c, a.dataDir, true)
	c.Assert(err, check.IsNil)
	// close twice:
	c.Assert(alog.Close(), check.IsNil)
	c.Assert(alog.Close(), check.IsNil)
}

// TestSessionsOnOneAuthServer tests scenario when there are two auth servers
// but session is recorded only on the one
func (a *AuditTestSuite) TestSessionsOnOneAuthServer(c *check.C) {
	fakeClock := clockwork.NewFakeClock()

	alog, err := NewAuditLog(AuditLogConfig{
		Clock:          fakeClock,
		DataDir:        a.dataDir,
		RecordSessions: true,
		ServerID:       "server1",
	})
	c.Assert(err, check.IsNil)

	alog2, err := NewAuditLog(AuditLogConfig{
		Clock:          fakeClock,
		DataDir:        a.dataDir,
		RecordSessions: true,
		ServerID:       "server2",
	})
	c.Assert(err, check.IsNil)

	sessionID := "100"
	// start the session and emit data stream to it
	firstMessage := []byte("hello")
	err = alog.PostSessionSlice(SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: sessionID,
		Chunks: []*SessionChunk{
			// start the seession
			&SessionChunk{
				Time:       fakeClock.Now().UTC().UnixNano(),
				EventIndex: 0,
				EventType:  SessionStartEvent,
				Data:       marshal(EventFields{EventLogin: "bob"}),
			},
			// type "hello" into session "100"
			&SessionChunk{
				Time:       fakeClock.Now().UTC().UnixNano(),
				EventIndex: 1,
				ChunkIndex: 0,
				Offset:     0,
				EventType:  SessionPrintEvent,
				Data:       firstMessage,
			},
			// emitting session end event should close the session
			&SessionChunk{
				Time:       fakeClock.Now().UTC().UnixNano(),
				EventIndex: 4,
				EventType:  SessionEndEvent,
				Data:       marshal(EventFields{EventLogin: "bob"}),
			},
		},
		Version: V2,
	})
	c.Assert(err, check.IsNil)

	// does not matter which audit server is accessed the results should be the same
	for _, a := range []*AuditLog{alog, alog2} {
		// read the session bytes
		history, err := a.GetSessionEvents(defaults.Namespace, session.ID(sessionID), 0, true)
		c.Assert(err, check.IsNil)
		c.Assert(history, check.HasLen, 3)

		// make sure offsets were properly set (0 for the first event and 5 bytes for hello):
		c.Assert(history[1][SessionByteOffset], check.Equals, float64(0))
		c.Assert(history[1][SessionEventTimestamp], check.Equals, float64(0))

		// fetch all bytes
		buff, err := a.GetSessionChunk(defaults.Namespace, session.ID(sessionID), 0, 5000)
		c.Assert(err, check.IsNil)
		c.Assert(string(buff), check.Equals, string(firstMessage))

		// with offset
		buff, err = a.GetSessionChunk(defaults.Namespace, session.ID(sessionID), 2, 5000)
		c.Assert(err, check.IsNil)
		c.Assert(string(buff), check.Equals, string(firstMessage[2:]))
	}
}

// TestSessionsTwoAuthServers tests two auth servers behind the load balancer handling the event stream
// for the same session
func (a *AuditTestSuite) TestSessionsTwoAuthServers(c *check.C) {
	fakeClock := clockwork.NewFakeClock()

	alog, err := NewAuditLog(AuditLogConfig{
		Clock:          fakeClock,
		DataDir:        a.dataDir,
		RecordSessions: true,
		ServerID:       "server1",
	})
	c.Assert(err, check.IsNil)

	alog2, err := NewAuditLog(AuditLogConfig{
		Clock:          fakeClock,
		DataDir:        a.dataDir,
		RecordSessions: true,
		ServerID:       "server2",
	})
	c.Assert(err, check.IsNil)

	sessionID := "100"
	// start the session and emit data stream to it
	firstMessage := []byte("hello")
	err = alog.PostSessionSlice(SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: sessionID,
		Chunks: []*SessionChunk{
			// start the seession
			&SessionChunk{
				Time:       fakeClock.Now().UTC().UnixNano(),
				EventIndex: 0,
				EventType:  SessionStartEvent,
				Data:       marshal(EventFields{EventLogin: "bob"}),
			},
			// type "hello" into session "100"
			&SessionChunk{
				Time:       fakeClock.Now().UTC().UnixNano(),
				EventIndex: 1,
				ChunkIndex: 0,
				Offset:     0,
				EventType:  SessionPrintEvent,
				Data:       firstMessage,
			},
		},
		Version: V2,
	})
	c.Assert(err, check.IsNil)
	c.Assert(alog.loggers.Len(), check.Equals, 1)

	// now fake sleep past expiration
	firstDelay := defaults.SessionIdlePeriod * 2
	fakeClock.Advance(firstDelay)

	// logger for idle session should be closed
	alog.closeInactiveLoggers()
	c.Assert(alog.loggers.Len(), check.Equals, 0)

	// send another event to the session via second auth server
	secondMessage := []byte("good day")
	err = alog2.PostSessionSlice(SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: sessionID,
		Chunks: []*SessionChunk{
			// notice how offsets are sent by the client
			&SessionChunk{
				Time:       fakeClock.Now().UTC().UnixNano(),
				Delay:      int64(firstDelay / time.Millisecond),
				EventIndex: 2,
				ChunkIndex: 1,
				Offset:     int64(len(firstMessage)),
				EventType:  SessionPrintEvent,
				Data:       secondMessage,
			},
		},
		Version: V2,
	})
	c.Assert(err, check.IsNil)
	c.Assert(alog2.loggers.Len(), check.Equals, 1)

	// emit next event 17 milliseconds later to the first auth server
	thirdMessage := []byte("test")
	secondDelay := 17 * time.Millisecond
	fakeClock.Advance(secondDelay)
	err = alog.PostSessionSlice(SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: sessionID,
		Chunks: []*SessionChunk{
			// notice how offsets are sent by the client
			&SessionChunk{
				Time:       fakeClock.Now().UTC().UnixNano(),
				EventIndex: 3,
				ChunkIndex: 2,
				Delay:      int64((firstDelay + secondDelay) / time.Millisecond),
				Offset:     int64(len(firstMessage) + len(secondMessage)),
				EventType:  SessionPrintEvent,
				Data:       thirdMessage,
			},
			// emitting session end event should close the session
			&SessionChunk{
				Time:       fakeClock.Now().UTC().UnixNano(),
				EventIndex: 4,
				EventType:  SessionEndEvent,
				Data:       marshal(EventFields{EventLogin: "bob"}),
			},
		},
		Version: V2,
	})
	c.Assert(err, check.IsNil)

	// emitting session end event should close the session
	c.Assert(alog.loggers.Len(), check.Equals, 0)

	// does not matter which audit server is accessed the results should be the same
	for _, a := range []*AuditLog{alog, alog2} {
		// read the session bytes
		history, err := a.GetSessionEvents(defaults.Namespace, session.ID(sessionID), 1, true)
		c.Assert(err, check.IsNil)
		c.Assert(history, check.HasLen, 4)

		// make sure offsets were properly set (0 for the first event and 5 bytes for hello):
		c.Assert(history[0][SessionByteOffset], check.Equals, float64(0))
		c.Assert(history[1][SessionByteOffset], check.Equals, float64(len(firstMessage)))
		c.Assert(history[2][SessionByteOffset], check.Equals, float64(len(firstMessage)+len(secondMessage)))

		// make sure delays are right
		c.Assert(history[0][SessionEventTimestamp], check.Equals, float64(0))
		c.Assert(history[1][SessionEventTimestamp], check.Equals, float64(firstDelay/time.Millisecond))
		c.Assert(history[2][SessionEventTimestamp], check.Equals, float64((firstDelay+secondDelay)/time.Millisecond))

		// fetch all bytes
		buff, err := a.GetSessionChunk(defaults.Namespace, session.ID(sessionID), 0, 5000)
		c.Assert(err, check.IsNil)
		c.Assert(string(buff), check.Equals, "hellogood daytest")

		// with offset
		buff, err = a.GetSessionChunk(defaults.Namespace, session.ID(sessionID), 2, 5000)
		c.Assert(err, check.IsNil)
		c.Assert(string(buff), check.Equals, "llogood daytest")

		// with another offset at the boundary of the first message
		buff, err = a.GetSessionChunk(defaults.Namespace, session.ID(sessionID), len(firstMessage), 5000)
		c.Assert(err, check.IsNil)
		c.Assert(string(buff), check.Equals, "good daytest")

		// with another offset after the boundary of the first message
		buff, err = a.GetSessionChunk(defaults.Namespace, session.ID(sessionID), len(firstMessage)+1, 5000)
		c.Assert(err, check.IsNil)
		c.Assert(string(buff), check.Equals, "ood daytest")

		// with another offset after the boundary of the third message
		buff, err = a.GetSessionChunk(defaults.Namespace, session.ID(sessionID), len(firstMessage)+len(secondMessage), 5000)
		c.Assert(err, check.IsNil)
		c.Assert(string(buff), check.Equals, "test")

		// with another offset outside the boundaries
		buff, err = a.GetSessionChunk(defaults.Namespace, session.ID(sessionID), len(firstMessage)+len(secondMessage)+len(thirdMessage), 5000)
		c.Assert(err, check.IsNil)
		c.Assert(string(buff), check.Equals, "")
	}
}

// TestSearchTwoAuthServers tests search on two auth servers behind the load balancer handling the event stream
// for the same session
func (a *AuditTestSuite) TestSearchTwoAuthServers(c *check.C) {
	startTime := time.Now().UTC()

	alog, err := NewAuditLog(AuditLogConfig{
		DataDir:        a.dataDir,
		RecordSessions: true,
		ServerID:       "server1",
	})
	c.Assert(err, check.IsNil)

	alog2, err := NewAuditLog(AuditLogConfig{
		DataDir:        a.dataDir,
		RecordSessions: true,
		ServerID:       "server2",
	})
	c.Assert(err, check.IsNil)

	sessionID := "100"
	// start the session and emit data stream to it
	firstMessage := []byte("hello")
	err = alog.PostSessionSlice(SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: sessionID,
		Chunks: []*SessionChunk{
			// start the seession
			&SessionChunk{
				Time:       time.Now().UTC().UnixNano(),
				EventIndex: 0,
				EventType:  SessionStartEvent,
				Data:       marshal(EventFields{EventLogin: "bob"}),
			},
			// type "hello" into session "100"
			&SessionChunk{
				Time:       time.Now().UTC().UnixNano(),
				EventIndex: 1,
				ChunkIndex: 0,
				Offset:     0,
				EventType:  SessionPrintEvent,
				Data:       firstMessage,
			},
		},
		Version: V2,
	})
	c.Assert(err, check.IsNil)
	c.Assert(alog.loggers.Len(), check.Equals, 1)

	// send another event to the session via second auth server
	secondMessage := []byte("good day")
	err = alog2.PostSessionSlice(SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: sessionID,
		Chunks: []*SessionChunk{
			// notice how offsets are sent by the client
			&SessionChunk{
				Time:       time.Now().UTC().UnixNano(),
				EventIndex: 2,
				ChunkIndex: 1,
				Offset:     int64(len(firstMessage)),
				EventType:  SessionPrintEvent,
				Data:       secondMessage,
			},
		},
		Version: V2,
	})
	c.Assert(err, check.IsNil)
	c.Assert(alog2.loggers.Len(), check.Equals, 1)

	// emit next event 17 milliseconds later to the second auth server
	thirdMessage := []byte("test")
	err = alog2.PostSessionSlice(SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: sessionID,
		Chunks: []*SessionChunk{
			// notice how offsets are sent by the client
			&SessionChunk{
				Time:       time.Now().Add(time.Hour).UTC().UnixNano(),
				EventIndex: 3,
				ChunkIndex: 2,
				Offset:     int64(len(firstMessage) + len(secondMessage)),
				EventType:  SessionPrintEvent,
				Data:       thirdMessage,
			},
			// emitting session end event should close the session
			&SessionChunk{
				Time:       time.Now().Add(time.Hour).UTC().UnixNano(),
				EventIndex: 4,
				EventType:  SessionEndEvent,
				Data:       marshal(EventFields{EventLogin: "bob"}),
			},
		},
		Version: V2,
	})
	c.Assert(err, check.IsNil)

	// emitting session end event should close the session on the second logger
	c.Assert(alog2.loggers.Len(), check.Equals, 0)

	// does not matter which audit server is accessed the results should be the same
	for _, a := range []*AuditLog{alog, alog2} {
		comment := check.Commentf("auth server %v", a.ServerID)

		// search events, start time is in the future
		query := fmt.Sprintf("%s=%s", EventType, SessionStartEvent)
		found, err := a.SearchEvents(startTime.Add(48*time.Hour), startTime.Add(72*time.Hour), query, 0)
		c.Assert(err, check.IsNil)
		c.Assert(len(found), check.Equals, 0, comment)

		// try searching (wrong query)
		found, err = a.SearchEvents(startTime, startTime.Add(time.Hour), "foo=bar", 0)
		c.Assert(err, check.IsNil)
		c.Assert(len(found), check.Equals, 0, comment)

		// try searching (good query: for "session start")
		found, err = a.SearchEvents(startTime.Add(-time.Hour), startTime.Add(time.Hour), query, 0)
		c.Assert(err, check.IsNil)
		c.Assert(len(found), check.Equals, 1, comment)
		c.Assert(found[0].GetString(EventLogin), check.Equals, "bob", comment)

		// try searching (empty query means "anything")
		found, err = alog.SearchEvents(startTime.Add(-time.Hour), startTime.Add(time.Hour), "", 0)
		c.Assert(err, check.IsNil)
		c.Assert(len(found), check.Equals, 2) // total number of events logged in this test
		c.Assert(found[0].GetString(EventType), check.Equals, SessionStartEvent, comment)
		c.Assert(found[1].GetString(EventType), check.Equals, SessionEndEvent, comment)

		// limit to 1
		found, err = alog.SearchEvents(startTime.Add(-time.Hour), startTime.Add(time.Hour), "", 1)
		c.Assert(err, check.IsNil)
		c.Assert(len(found), check.Equals, 1) // total number of events logged in this test
	}
}

// TestSearchTwoAuthServersSameTime tests search on two auth servers behind the load balancer handling the event stream
// for the same session emitted around the same time
func (a *AuditTestSuite) TestSearchTwoAuthServersSameTime(c *check.C) {
	startTime := time.Now().UTC()

	alog, err := NewAuditLog(AuditLogConfig{
		DataDir:        a.dataDir,
		RecordSessions: true,
		ServerID:       "server1",
	})
	c.Assert(err, check.IsNil)

	alog2, err := NewAuditLog(AuditLogConfig{
		DataDir:        a.dataDir,
		RecordSessions: true,
		ServerID:       "server2",
	})
	c.Assert(err, check.IsNil)

	sessionID := "100"
	// start the session and emit data stream to it
	firstMessage := []byte("hello")
	now := time.Now().UTC().UnixNano()
	err = alog.PostSessionSlice(SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: sessionID,
		Chunks: []*SessionChunk{
			// start the seession
			&SessionChunk{
				Time:       now,
				EventIndex: 0,
				EventType:  SessionStartEvent,
				Data:       marshal(EventFields{EventLogin: "bob"}),
			},
			// type "hello" into session "100"
			&SessionChunk{
				Time:       now,
				EventIndex: 1,
				ChunkIndex: 0,
				Offset:     0,
				EventType:  SessionPrintEvent,
				Data:       firstMessage,
			},
		},
		Version: V2,
	})
	c.Assert(err, check.IsNil)
	c.Assert(alog.loggers.Len(), check.Equals, 1)

	// send another event to the session via second auth server
	secondMessage := []byte("good day")
	err = alog2.PostSessionSlice(SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: sessionID,
		Chunks: []*SessionChunk{
			// notice how offsets are sent by the client
			&SessionChunk{
				Time:       now,
				EventIndex: 2,
				ChunkIndex: 1,
				Offset:     int64(len(firstMessage)),
				EventType:  SessionPrintEvent,
				Data:       secondMessage,
			},
		},
		Version: V2,
	})
	c.Assert(err, check.IsNil)
	c.Assert(alog2.loggers.Len(), check.Equals, 1)

	// emit next event 17 milliseconds later to the second auth server
	thirdMessage := []byte("test")
	err = alog2.PostSessionSlice(SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: sessionID,
		Chunks: []*SessionChunk{
			// notice how offsets are sent by the client
			&SessionChunk{
				Time:       now,
				EventIndex: 3,
				ChunkIndex: 2,
				Offset:     int64(len(firstMessage) + len(secondMessage)),
				EventType:  SessionPrintEvent,
				Data:       thirdMessage,
			},
			// emitting session end event should close the session
			&SessionChunk{
				Time:       now,
				EventIndex: 4,
				EventType:  SessionEndEvent,
				Data:       marshal(EventFields{EventLogin: "bob"}),
			},
		},
		Version: V2,
	})
	c.Assert(err, check.IsNil)

	// emitting session end event should close the session on the second logger
	c.Assert(alog2.loggers.Len(), check.Equals, 0)

	// does not matter which audit server is accessed the results should be the same
	for _, a := range []*AuditLog{alog, alog2} {
		comment := check.Commentf("auth server %v", a.ServerID)

		// try searching (empty query means "anything")
		found, err := alog.SearchEvents(startTime.Add(-time.Hour), startTime.Add(time.Hour), "", 0)
		c.Assert(err, check.IsNil)
		c.Assert(len(found), check.Equals, 2) // total number of events logged in this test
		c.Assert(found[0].GetString(EventType), check.Equals, SessionStartEvent, comment)
		c.Assert(found[1].GetString(EventType), check.Equals, SessionEndEvent, comment)
	}
}

type file struct {
	path     string
	contents []byte
}

var v1Files = []file{
	{
		path: "2017-12-14.00:00:00.log",
		contents: []byte(`{"event":"user.login","method":"oidc","time":"2017-12-13T17:38:34Z","user":"alice@example.com"}
{"addr.local":"172.10.1.20:3022","addr.remote":"172.10.1.254:52406","event":"session.start","login":"root","namespace":"default","server_id":"3e79a2d7-c9e3-4d3f-96ce-1e1346b4900c","sid":"74a5fc73-e02c-11e7-aee2-0242ac0a0101","size":"80:25","time":"2017-12-13T17:38:40Z","user":"alice@example.com"}
{"event":"session.leave","namespace":"default","server_id":"020130c8-b41f-4da5-a061-74c2b0e2b40b","sid":"75aef036-e02c-11e7-aee2-0242ac0a0101","time":"2017-12-13T17:38:42Z","user":"alice@example.com"}
`),
	},
	{
		path: "sessions/default/74a5fc73-e02c-11e7-aee2-0242ac0a0101.session.log",
		contents: []byte(`{"addr.local":"172.10.1.20:3022","addr.remote":"172.10.1.254:52406","event":"session.start","login":"root","namespace":"default","server_id":"3e79a2d7-c9e3-4d3f-96ce-1e1346b4900c","sid":"74a5fc73-e02c-11e7-aee2-0242ac0a0101","size":"80:25","time":"2017-12-13T17:38:40Z","user":"alice@example.com"}
{"event":"session.leave","namespace":"default","server_id":"3e79a2d7-c9e3-4d3f-96ce-1e1346b4900c","sid":"74a5fc73-e02c-11e7-aee2-0242ac0a0101","time":"2017-12-13T17:38:41Z","user":"alice@example.com"}
{"time":"2017-12-13T17:38:40.038Z","event":"print","bytes":31,"ms":0,"offset":0}
`),
	},
	{
		path:     "sessions/default/74a5fc73-e02c-11e7-aee2-0242ac0a0101.session.bytes",
		contents: []byte(`"this string is exactly 31 bytes"`),
	},
}

func (a *AuditTestSuite) TestSessionRecordingOff(c *check.C) {
	now := time.Now().In(time.UTC).Round(time.Second)

	// create audit log with session recording disabled
	alog, err := a.makeLog(c, a.dataDir, false)
	c.Assert(err, check.IsNil)
	alog.Clock = clockwork.NewFakeClockAt(now)

	username := "alice"
	sessionID := "200"

	// start the session and emit data stream to it
	firstMessage := []byte("hello")
	err = alog.PostSessionSlice(SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: sessionID,
		Chunks: []*SessionChunk{
			// start the session
			&SessionChunk{
				Time:       alog.Clock.Now().UTC().UnixNano(),
				EventIndex: 0,
				EventType:  SessionStartEvent,
				Data:       marshal(EventFields{EventLogin: username}),
			},
			// type "hello" into session "100"
			&SessionChunk{
				Time:       alog.Clock.Now().UTC().UnixNano(),
				EventIndex: 1,
				ChunkIndex: 0,
				Offset:     0,
				EventType:  SessionPrintEvent,
				Data:       firstMessage,
			},
			// end the session
			&SessionChunk{
				Time:       alog.Clock.Now().UTC().UnixNano(),
				EventIndex: 0,
				EventType:  SessionEndEvent,
				Data:       marshal(EventFields{EventLogin: username}),
			},
		},
		Version: V2,
	})
	c.Assert(err, check.IsNil)
	c.Assert(alog.loggers.Len(), check.Equals, 0)

	// get all events from the audit log, should have two events
	found, err := alog.SearchEvents(now.Add(-time.Hour), now.Add(time.Hour), "", 0)
	c.Assert(err, check.IsNil)
	c.Assert(found, check.HasLen, 2)
	c.Assert(found[0].GetString(EventLogin), check.Equals, username)
	c.Assert(found[1].GetString(EventLogin), check.Equals, username)

	// inspect the session log for "200", should have two events
	history, err := alog.GetSessionEvents(defaults.Namespace, session.ID(sessionID), 0, true)
	c.Assert(err, check.IsNil)
	c.Assert(history, check.HasLen, 2)

	// try getting the session stream, should get an error
	_, err = alog.GetSessionChunk(defaults.Namespace, session.ID(sessionID), 0, 5000)
	c.Assert(err, check.NotNil)
}

func (a *AuditTestSuite) TestBasicLogging(c *check.C) {
	now := time.Now().In(time.UTC).Round(time.Second)
	// create audit log, write a couple of events into it, close it
	alog, err := a.makeLog(c, a.dataDir, true)
	c.Assert(err, check.IsNil)
	alog.Clock = clockwork.NewFakeClockAt(now)

	// emit regular event:
	err = alog.EmitAuditEvent("user.joined", EventFields{"apples?": "yes"})
	c.Assert(err, check.IsNil)
	logfile := alog.file.Name()
	c.Assert(alog.Close(), check.IsNil)

	// read back what's been written:
	bytes, err := ioutil.ReadFile(logfile)
	c.Assert(err, check.IsNil)
	c.Assert(string(bytes), check.Equals,
		fmt.Sprintf("{\"apples?\":\"yes\",\"event\":\"user.joined\",\"time\":\"%s\"}\n", now.Format(time.RFC3339)))
}

// TestAutoClose tests scenario with auto closing of inactive sessions
func (a *AuditTestSuite) TestAutoClose(c *check.C) {
	// create audit log, write a couple of events into it, close it
	fakeClock := clockwork.NewFakeClock()
	alog, err := NewAuditLog(AuditLogConfig{
		DataDir:        a.dataDir,
		RecordSessions: true,
		Clock:          fakeClock,
		ServerID:       "autoclose1",
	})
	c.Assert(err, check.IsNil)

	sessionID := "100"
	// start the session and emit data stream to it
	firstMessage := []byte("hello")
	err = alog.PostSessionSlice(SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: sessionID,
		Chunks: []*SessionChunk{
			// start the seession
			&SessionChunk{
				Time:       fakeClock.Now().UTC().UnixNano(),
				EventIndex: 0,
				EventType:  SessionStartEvent,
				Data:       marshal(EventFields{EventLogin: "bob"}),
			},
			// type "hello" into session "100"
			&SessionChunk{
				Time:       fakeClock.Now().UTC().UnixNano(),
				EventIndex: 1,
				ChunkIndex: 0,
				Offset:     0,
				EventType:  SessionPrintEvent,
				Data:       firstMessage,
			},
		},
		Version: V2,
	})
	c.Assert(err, check.IsNil)
	c.Assert(alog.loggers.Len(), check.Equals, 1)

	// now fake sleep past expiration
	firstDelay := defaults.SessionIdlePeriod * 2
	fakeClock.Advance(firstDelay)

	// logger for idle session should be closed
	alog.closeInactiveLoggers()
	c.Assert(alog.loggers.Len(), check.Equals, 0)

	// send another event to the session
	secondMessage := []byte("good day")
	err = alog.PostSessionSlice(SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: sessionID,
		Chunks: []*SessionChunk{
			// notice how offsets are sent by the client
			&SessionChunk{
				Time:       fakeClock.Now().UTC().UnixNano(),
				Delay:      int64(firstDelay / time.Millisecond),
				EventIndex: 2,
				ChunkIndex: 1,
				Offset:     int64(len(firstMessage)),
				EventType:  SessionPrintEvent,
				Data:       secondMessage,
			},
		},
		Version: V2,
	})
	c.Assert(err, check.IsNil)
	c.Assert(alog.loggers.Len(), check.Equals, 1)

	// emit next event 17 milliseconds later
	thirdMessage := []byte("test")
	secondDelay := 17 * time.Millisecond
	fakeClock.Advance(secondDelay)
	err = alog.PostSessionSlice(SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: sessionID,
		Chunks: []*SessionChunk{
			// notice how offsets are sent by the client
			&SessionChunk{
				Time:       fakeClock.Now().UTC().UnixNano(),
				EventIndex: 3,
				ChunkIndex: 2,
				Delay:      int64((firstDelay + secondDelay) / time.Millisecond),
				Offset:     int64(len(firstMessage) + len(secondMessage)),
				EventType:  SessionPrintEvent,
				Data:       thirdMessage,
			},
			// emitting session end event should close the session
			&SessionChunk{
				Time:       fakeClock.Now().UTC().UnixNano(),
				EventIndex: 4,
				EventType:  SessionEndEvent,
				Data:       marshal(EventFields{EventLogin: "bob"}),
			},
		},
		Version: V2,
	})
	c.Assert(err, check.IsNil)

	// emitting session end event should close the session
	c.Assert(alog.loggers.Len(), check.Equals, 0)

	// read the session bytes
	history, err := alog.GetSessionEvents(defaults.Namespace, session.ID(sessionID), 1, true)
	c.Assert(err, check.IsNil)
	c.Assert(history, check.HasLen, 4)

	// make sure offsets were properly set (0 for the first event and 5 bytes for hello):
	c.Assert(history[0][SessionByteOffset], check.Equals, float64(0))
	c.Assert(history[1][SessionByteOffset], check.Equals, float64(len(firstMessage)))
	c.Assert(history[2][SessionByteOffset], check.Equals, float64(len(firstMessage)+len(secondMessage)))

	// make sure delays are right
	c.Assert(history[0][SessionEventTimestamp], check.Equals, float64(0))
	c.Assert(history[1][SessionEventTimestamp], check.Equals, float64(firstDelay/time.Millisecond))
	c.Assert(history[2][SessionEventTimestamp], check.Equals, float64((firstDelay+secondDelay)/time.Millisecond))

	// Cleanup old playbacks
	// Fetch chunks so playback directory will be populated
	data, err := alog.GetSessionChunk(defaults.Namespace, session.ID(sessionID), 0, 5000)
	c.Assert(err, check.IsNil)
	c.Assert(string(data), check.Equals, "hellogood daytest")

	// two files were unpacked in playback dir
	c.Assert(listFiles(alog.playbackDir), check.HasLen, 2)

	// first cleanup did not delete files
	alog.Clock = clockwork.NewFakeClockAt(time.Now().UTC())
	c.Assert(alog.cleanupOldPlaybacks(), check.IsNil)
	c.Assert(listFiles(alog.playbackDir), check.HasLen, 2)

	// Advance clock past cleanup TTL, files were cleaned up
	alog.Clock = clockwork.NewFakeClockAt(time.Now().UTC().Add(alog.PlaybackRecycleTTL + time.Minute))
	c.Assert(alog.cleanupOldPlaybacks(), check.IsNil)
	c.Assert(listFiles(alog.playbackDir), check.HasLen, 0)
}

func listFiles(name string) []string {
	df, err := os.Open(name)
	if err != nil {
		panic(err)
	}
	defer df.Close()
	entries, err := df.Readdir(-1)
	if err != nil {
		panic(err)
	}
	var out []string
	for i := range entries {
		fi := entries[i]
		if !fi.IsDir() {
			out = append(out, fi.Name())
		}
	}
	return out
}

// TestCloseOutstanding makes sure the logger closed outstanding sessions
func (a *AuditTestSuite) TestCloseOutstanding(c *check.C) {
	// create audit log, write a couple of events into it, close it
	fakeClock := clockwork.NewFakeClock()
	alog, err := NewAuditLog(AuditLogConfig{
		DataDir:        a.dataDir,
		RecordSessions: true,
		Clock:          fakeClock,
		ServerID:       "outstanding",
	})
	c.Assert(err, check.IsNil)
	// start the session and emit data stream to it
	err = alog.PostSessionSlice(SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: "100",
		Chunks: []*SessionChunk{
			&SessionChunk{
				EventIndex: 0,
				EventType:  SessionStartEvent,
				Data:       marshal(EventFields{EventLogin: "bob"}),
			},
		},
		Version: V2,
	})
	c.Assert(err, check.IsNil)
	c.Assert(alog.loggers.Len(), check.Equals, 1)

	alog.Close()
	c.Assert(alog.loggers.Len(), check.Equals, 0)
}

// TestForwardAndUpload tests forwarding server and upload
// server case
func (a *AuditTestSuite) TestForwardAndUpload(c *check.C) {
	storageDir := c.MkDir()
	fileHandler, err := filesessions.NewHandler(filesessions.Config{
		Directory: storageDir,
	})

	// start uploader and make sure it uploads event to the event
	// storage

	uploadDir := c.MkDir()
	err = os.MkdirAll(filepath.Join(uploadDir, "upload", "sessions", defaults.Namespace), 0755)

	fakeClock := clockwork.NewFakeClock()

	alog, err := NewAuditLog(AuditLogConfig{
		DataDir:        a.dataDir,
		RecordSessions: true,
		Clock:          fakeClock,
		ServerID:       "remote",
		UploadHandler:  fileHandler,
	})
	c.Assert(err, check.IsNil)

	sessionID := session.ID("100")
	forwarder, err := NewForwarder(ForwarderConfig{
		Namespace:      defaults.Namespace,
		SessionID:      sessionID,
		ServerID:       "upload",
		DataDir:        uploadDir,
		RecordSessions: true,
		ForwardTo:      alog,
	})
	c.Assert(err, check.IsNil)

	// start the session and emit data stream to it and wrap it up
	firstMessage := []byte("hello")
	err = forwarder.PostSessionSlice(SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: string(sessionID),
		Chunks: []*SessionChunk{
			// start the seession
			&SessionChunk{
				Time:       fakeClock.Now().UTC().UnixNano(),
				EventIndex: 0,
				EventType:  SessionStartEvent,
				Data:       marshal(EventFields{EventLogin: "bob"}),
			},
			// type "hello" into session "100"
			&SessionChunk{
				Time:       fakeClock.Now().UTC().UnixNano(),
				EventIndex: 1,
				ChunkIndex: 0,
				Offset:     0,
				EventType:  SessionPrintEvent,
				Data:       firstMessage,
			},
			// emitting session end event should close the session
			&SessionChunk{
				Time:       fakeClock.Now().UTC().UnixNano(),
				EventIndex: 4,
				EventType:  SessionEndEvent,
				Data:       marshal(EventFields{EventLogin: "bob"}),
			},
		},
		Version: V2,
	})
	c.Assert(err, check.IsNil)
	c.Assert(forwarder.Close(), check.IsNil)

	// start uploader process
	eventsC := make(chan *UploadEvent, 100)
	uploader, err := NewUploader(UploaderConfig{
		ServerID:   "upload",
		DataDir:    uploadDir,
		Clock:      fakeClock,
		Namespace:  defaults.Namespace,
		Context:    context.TODO(),
		ScanPeriod: 100 * time.Millisecond,
		AuditLog:   alog,
		EventsC:    eventsC,
	})
	c.Assert(err, check.IsNil)

	// scanner should upload the events
	err = uploader.scan()
	c.Assert(err, check.IsNil)

	select {
	case event := <-eventsC:
		c.Assert(event, check.NotNil)
		c.Assert(event.Error, check.IsNil)
	case <-time.After(time.Second):
		c.Fatalf("Timeout wating for the upload event")
	}

	compare := func() error {
		history, err := alog.GetSessionEvents(defaults.Namespace, session.ID(sessionID), 0, true)
		if err != nil {
			return trace.Wrap(err)
		}
		if len(history) != 3 {
			return trace.BadParameter("expected history of 3, got %v", len(history))
		}

		// make sure offsets were properly set (0 for the first event and 5 bytes for hello):
		if history[1][SessionByteOffset].(float64) != float64(0) {
			return trace.BadParameter("expected offset of 0, got %v", history[1][SessionByteOffset])
		}
		if history[1][SessionEventTimestamp].(float64) != float64(0) {
			return trace.BadParameter("expected timestamp of 0, got %v", history[1][SessionEventTimestamp])
		}

		// fetch all bytes
		buff, err := alog.GetSessionChunk(defaults.Namespace, session.ID(sessionID), 0, 5000)
		if err != nil {
			return trace.Wrap(err)
		}
		if string(buff) != string(firstMessage) {
			return trace.CompareFailed("%q != %q", string(buff), string(firstMessage))
		}

		// with offset
		buff, err = alog.GetSessionChunk(defaults.Namespace, session.ID(sessionID), 2, 5000)
		if err != nil {
			return trace.Wrap(err)
		}
		if string(buff) != string(firstMessage[2:]) {
			return trace.CompareFailed("%q != %q", string(buff), string(firstMessage[2:]))
		}
		return nil
	}

	// trigger several parallel downloads, they should not fail
	iterations := 50
	resultsC := make(chan error, iterations)
	for i := 0; i < iterations; i++ {
		go func() {
			resultsC <- compare()
		}()
	}

	timeout := time.After(time.Second)
	for i := 0; i < iterations; i++ {
		select {
		case err := <-resultsC:
			c.Assert(err, check.IsNil)
		case <-timeout:
			c.Fatalf("timeout waiting for goroutines to finish")
		}
	}
}

func marshal(f EventFields) []byte {
	data, err := json.Marshal(f)
	if err != nil {
		panic(err)
	}
	return data
}
