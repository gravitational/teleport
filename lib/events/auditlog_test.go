package events

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/check.v1"

	"github.com/gravitational/teleport/lib/defaults"
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

// DELETE IN: 2.6.0
func (a *AuditTestSuite) TestCompatComplexLogging(c *check.C) {
	now := time.Now().In(time.UTC).Round(time.Second)

	// create audit log, write a couple of events into it, close it
	alog, err := a.makeLog(c, a.dataDir, true)
	c.Assert(err, check.IsNil)
	alog.Clock = clockwork.NewFakeClockAt(now)

	// emit two session-attached events (same session)
	err = alog.EmitAuditEvent(SessionStartEvent, EventFields{SessionEventID: "100", EventLogin: "vincent", EventNamespace: defaults.Namespace})
	c.Assert(err, check.IsNil)
	c.Assert(alog.loggers.Len(), check.Equals, 1)
	err = alog.EmitAuditEvent(SessionLeaveEvent, EventFields{SessionEventID: "100", EventLogin: "vincent", EventNamespace: defaults.Namespace})
	c.Assert(alog.loggers.Len(), check.Equals, 1)
	err = alog.EmitAuditEvent(SessionJoinEvent, EventFields{SessionEventID: "200", EventLogin: "doggy", EventNamespace: defaults.Namespace})
	c.Assert(err, check.IsNil)
	c.Assert(alog.loggers.Len(), check.Equals, 2)

	// type "hello" into session "200":
	err = alog.PostSessionChunk(defaults.Namespace, "200", bytes.NewBufferString("hello"))
	c.Assert(err, check.IsNil)

	// emit "sesion-end" event. one of the loggers must disappear
	err = alog.EmitAuditEvent(SessionEndEvent, EventFields{SessionEventID: "200", EventLogin: "doggy", EventNamespace: defaults.Namespace})
	c.Assert(err, check.IsNil)
	c.Assert(alog.loggers.Len(), check.Equals, 1)

	// add a few more loggers and close:
	alog.EmitAuditEvent(SessionJoinEvent, EventFields{SessionEventID: "300", EventLogin: "frankie", EventNamespace: defaults.Namespace})
	alog.EmitAuditEvent(SessionJoinEvent, EventFields{SessionEventID: "400", EventLogin: "rosie", EventNamespace: defaults.Namespace})
	alog.Close()
	c.Assert(alog.loggers.Len(), check.Equals, 0)

	// inspect session "200". it could have three events: join, print and leave:
	history, err := alog.GetSessionEvents(defaults.Namespace, "200", 0)
	c.Assert(err, check.IsNil)
	c.Assert(history, check.HasLen, 3)
	c.Assert(history[0][EventLogin], check.Equals, "doggy")
	c.Assert(history[0][EventType], check.Equals, SessionJoinEvent)
	c.Assert(history[1][EventType], check.Equals, SessionPrintEvent)
	c.Assert(history[1][SessionByteOffset].(float64), check.Equals, float64(0))
	c.Assert(history[1][SessionPrintEventBytes].(float64), check.Equals, float64(5))
	c.Assert(history[2][EventType], check.Equals, SessionEndEvent)

	// try the same, but with 'afterN', we should only get the 3rd event:
	history2, err := alog.GetSessionEvents(defaults.Namespace, "200", 2)
	c.Assert(err, check.IsNil)
	c.Assert(history2, check.HasLen, 1)
	c.Assert(history2[0], check.DeepEquals, history[2])

	// lets try session session stream (with offset 2 of bytes, i.e. instead of "hello" we should get "llo")
	buff, err := alog.GetSessionChunk(defaults.Namespace, "200", 2, 5000)
	c.Assert(err, check.IsNil)
	c.Assert(string(buff[:3]), check.Equals, "llo")

	// try searching (in the future)
	query := fmt.Sprintf("%s=%s", EventType, SessionStartEvent)
	found, err := alog.SearchEvents(now.Add(time.Hour), now.Add(time.Hour), query)
	c.Assert(err, check.IsNil)
	c.Assert(len(found), check.Equals, 0)

	// try searching (wrong query)
	found, err = alog.SearchEvents(now.Add(time.Hour), now.Add(time.Hour), "foo=bar")
	c.Assert(err, check.IsNil)
	c.Assert(len(found), check.Equals, 0)

	// try searching (good query: for "session start")
	found, err = alog.SearchEvents(now.Add(-time.Hour), now.Add(time.Hour), query)
	c.Assert(err, check.IsNil)
	c.Assert(len(found), check.Equals, 1)
	c.Assert(found[0].GetString(EventLogin), check.Equals, "vincent")

	// try searching (empty query means "anything")
	found, err = alog.SearchEvents(now.Add(-time.Hour), now.Add(time.Hour), "")
	c.Assert(err, check.IsNil)
	c.Assert(len(found), check.Equals, 6) // total number of events logged in this test
	c.Assert(found[0].GetString(EventLogin), check.Equals, "vincent")
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
		history, err := a.GetSessionEvents(defaults.Namespace, session.ID(sessionID), 0)
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
		history, err := a.GetSessionEvents(defaults.Namespace, session.ID(sessionID), 1)
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

	// emit next event 17 milliseconds later to the first auth server
	thirdMessage := []byte("test")
	err = alog.PostSessionSlice(SessionSlice{
		Namespace: defaults.Namespace,
		SessionID: sessionID,
		Chunks: []*SessionChunk{
			// notice how offsets are sent by the client
			&SessionChunk{
				Time:       time.Now().UTC().UnixNano(),
				EventIndex: 3,
				ChunkIndex: 2,
				Offset:     int64(len(firstMessage) + len(secondMessage)),
				EventType:  SessionPrintEvent,
				Data:       thirdMessage,
			},
			// emitting session end event should close the session
			&SessionChunk{
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
		comment := check.Commentf("auth server %v", a.ServerID)

		// search events, start time is in the future
		query := fmt.Sprintf("%s=%s", EventType, SessionStartEvent)
		found, err := a.SearchEvents(startTime.Add(time.Hour), startTime.Add(time.Hour), query)
		c.Assert(err, check.IsNil)
		c.Assert(len(found), check.Equals, 0, comment)

		// try searching (wrong query)
		found, err = a.SearchEvents(startTime, startTime.Add(time.Hour), "foo=bar")
		c.Assert(err, check.IsNil)
		c.Assert(len(found), check.Equals, 0, comment)

		// try searching (good query: for "session start")
		found, err = a.SearchEvents(startTime.Add(-time.Hour), startTime.Add(time.Hour), query)
		c.Assert(err, check.IsNil)
		c.Assert(len(found), check.Equals, 1, comment)
		c.Assert(found[0].GetString(EventLogin), check.Equals, "bob", comment)

		// try searching (empty query means "anything")
		found, err = alog.SearchEvents(startTime.Add(-time.Hour), startTime.Add(time.Hour), "")
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

// DELETE IN: 2.6.0
// TestMigrationsToV2 tests migrations to V2 loging format
func (a *AuditTestSuite) TestMigrationsToV2(c *check.C) {
	for _, file := range v1Files {
		fileName := filepath.Join(a.dataDir, file.path)
		err := os.MkdirAll(filepath.Dir(fileName), 0755)
		c.Assert(err, check.IsNil)
		err = ioutil.WriteFile(fileName, file.contents, 06440)
		c.Assert(err, check.IsNil)
	}

	// create audit log with session recording disabled
	alog, err := a.makeLog(c, a.dataDir, false)
	c.Assert(err, check.IsNil)

	// sessions have been migrated
	sid := "74a5fc73-e02c-11e7-aee2-0242ac0a0101"
	events, err := alog.GetSessionEvents(defaults.Namespace, session.ID(sid), 0)
	c.Assert(err, check.IsNil)
	c.Assert(events, check.HasLen, 3)

	// global events were migrated
	events, err = alog.SearchEvents(time.Time{}, time.Now().Add(time.Hour), "")
	c.Assert(err, check.IsNil)
	c.Assert(events, check.HasLen, 3)

	// second time migration is idempotent
	alog, err = a.makeLog(c, a.dataDir, false)
	c.Assert(err, check.IsNil)

	events, err = alog.GetSessionEvents(defaults.Namespace, session.ID(sid), 0)
	c.Assert(err, check.IsNil)
	c.Assert(events, check.HasLen, 3)

	// global events were migrated
	events, err = alog.SearchEvents(time.Time{}, time.Now().Add(time.Hour), "")
	c.Assert(err, check.IsNil)
	c.Assert(events, check.HasLen, 3)
}

// DELETE IN: 2.6.0
func (a *AuditTestSuite) TestCompatSessionRecordingOff(c *check.C) {
	now := time.Now().In(time.UTC).Round(time.Second)

	// create audit log with session recording disabled
	alog, err := a.makeLog(c, a.dataDir, false)
	c.Assert(err, check.IsNil)
	alog.Clock = clockwork.NewFakeClockAt(now)

	// emit "session.start" event into the audit log for session "200"
	err = alog.EmitAuditEvent(SessionStartEvent, EventFields{SessionEventID: "200", EventLogin: "doggy", EventNamespace: defaults.Namespace})
	c.Assert(err, check.IsNil)

	// type "hello" into session "200"
	err = alog.PostSessionChunk(defaults.Namespace, "200", bytes.NewBufferString("hello"))
	c.Assert(err, check.IsNil)

	// emit "sesion-end" event into the audit log for session "200"
	err = alog.EmitAuditEvent(SessionEndEvent, EventFields{SessionEventID: "200", EventLogin: "doggy", EventNamespace: defaults.Namespace})
	c.Assert(err, check.IsNil)

	// get all events from the audit log, should have two events
	found, err := alog.SearchEvents(now.Add(-time.Hour), now.Add(time.Hour), "")
	c.Assert(err, check.IsNil)
	c.Assert(found, check.HasLen, 2)
	c.Assert(found[0].GetString(EventLogin), check.Equals, "doggy")
	c.Assert(found[1].GetString(EventLogin), check.Equals, "doggy")

	// inspect the session log for "200", should have two events
	history, err := alog.GetSessionEvents(defaults.Namespace, "200", 0)
	c.Assert(err, check.IsNil)
	c.Assert(history, check.HasLen, 2)

	// try getting the session stream, should get an error
	_, err = alog.GetSessionChunk(defaults.Namespace, "200", 0, 5000)
	c.Assert(err, check.NotNil)
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
	found, err := alog.SearchEvents(now.Add(-time.Hour), now.Add(time.Hour), "")
	c.Assert(err, check.IsNil)
	c.Assert(found, check.HasLen, 2)
	c.Assert(found[0].GetString(EventLogin), check.Equals, username)
	c.Assert(found[1].GetString(EventLogin), check.Equals, username)

	// inspect the session log for "200", should have two events
	history, err := alog.GetSessionEvents(defaults.Namespace, session.ID(sessionID), 0)
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

// DELETE IN: 2.6.0
// TestCompatAutoClose tests scenario with auto closing of inactive sessions for compatibility logger
func (a *AuditTestSuite) TestCompatAutoClose(c *check.C) {
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
	err = alog.EmitAuditEvent(SessionStartEvent, EventFields{SessionEventID: sessionID, EventLogin: "bob", EventNamespace: defaults.Namespace})
	c.Assert(err, check.IsNil)
	c.Assert(alog.loggers.Len(), check.Equals, 1)

	// type "hello" into session "100":
	firstMessage := "hello"
	err = alog.PostSessionChunk(defaults.Namespace, session.ID(sessionID), bytes.NewBufferString(firstMessage))
	c.Assert(err, check.IsNil)

	// now fake sleep past expiration
	firstDelay := defaults.SessionIdlePeriod * 2
	fakeClock.Advance(firstDelay)

	// logger for idle session should be closed
	alog.closeInactiveLoggers()
	c.Assert(alog.loggers.Len(), check.Equals, 0)

	// send another event to the session
	secondMessage := "good day"
	err = alog.PostSessionChunk(defaults.Namespace, session.ID(sessionID), bytes.NewBufferString(secondMessage))
	c.Assert(err, check.IsNil)
	// the logger has been reopened
	c.Assert(alog.loggers.Len(), check.Equals, 1)

	// emit next event 17 milliseconds later
	secondDelay := 17 * time.Millisecond
	fakeClock.Advance(secondDelay)
	err = alog.PostSessionChunk(defaults.Namespace, session.ID(sessionID), bytes.NewBufferString("test"))
	c.Assert(err, check.IsNil)

	// emitting session end event should close the session
	err = alog.EmitAuditEvent(SessionEndEvent, EventFields{SessionEventID: sessionID, EventLogin: "bob", EventNamespace: defaults.Namespace})
	c.Assert(alog.loggers.Len(), check.Equals, 0)

	// read the session bytes
	history, err := alog.GetSessionEvents(defaults.Namespace, "100", 1)
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
	history, err := alog.GetSessionEvents(defaults.Namespace, session.ID(sessionID), 1)
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

}

// DELETE IN: 2.6.0
// TestCompatCloseOutstanding makes sure the logger working in compatibility mode
// closed outstanding sessions
func (a *AuditTestSuite) TestCompatCloseOutstanding(c *check.C) {
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
	err = alog.EmitAuditEvent(SessionStartEvent, EventFields{SessionEventID: "100", EventLogin: "bob", EventNamespace: defaults.Namespace})
	c.Assert(err, check.IsNil)
	c.Assert(alog.loggers.Len(), check.Equals, 1)

	alog.Close()
	c.Assert(alog.loggers.Len(), check.Equals, 0)
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

func marshal(f EventFields) []byte {
	data, err := json.Marshal(f)
	if err != nil {
		panic(err)
	}
	return data
}
