package events

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
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
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	retval, ok := alog.(*AuditLog)
	if !ok {
		c.FailNow()
	}
	return retval, nil
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

func (a *AuditTestSuite) TestComplexLogging(c *check.C) {
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

func (a *AuditTestSuite) TestSessionRecordingOff(c *check.C) {
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

	// get all events from the audit log, we should have two
	found, err := alog.SearchEvents(now.Add(-time.Hour), now.Add(time.Hour), "")
	c.Assert(err, check.IsNil)
	c.Assert(found, check.HasLen, 2)
	c.Assert(found[0].GetString(EventLogin), check.Equals, "doggy")
	c.Assert(found[1].GetString(EventLogin), check.Equals, "doggy")

	// inspect the session log for "200". it should be empty.
	history, err := alog.GetSessionEvents(defaults.Namespace, "200", 0)
	c.Assert(err, check.IsNil)
	c.Assert(history, check.HasLen, 0)
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
	alogI, err := NewAuditLog(AuditLogConfig{
		DataDir:        a.dataDir,
		RecordSessions: true,
		Clock:          fakeClock,
	})
	c.Assert(err, check.IsNil)
	alog := alogI.(*AuditLog)

	// start the session and emit data stream to it
	err = alog.EmitAuditEvent(SessionStartEvent, EventFields{SessionEventID: "100", EventLogin: "bob", EventNamespace: defaults.Namespace})
	c.Assert(err, check.IsNil)
	c.Assert(alog.loggers.Len(), check.Equals, 1)

	sessionID := "100"
	// type "hello" into session "100":
	err = alog.PostSessionChunk(defaults.Namespace, session.ID(sessionID), bytes.NewBufferString("hello"))
	c.Assert(err, check.IsNil)

	// now fake sleep past expiration
	fakeClock.Advance(defaults.SessionIdlePeriod * 2)

	// logger for idle session should be closed
	alog.closeInactiveLoggers()
	c.Assert(alog.loggers.Len(), check.Equals, 0)

	// send another event to the session
	err = alog.PostSessionChunk(defaults.Namespace, session.ID(sessionID), bytes.NewBufferString("howdy"))
	c.Assert(err, check.IsNil)
	// the logger has been reopened
	c.Assert(alog.loggers.Len(), check.Equals, 1)

	// emitting session end event should close the session
	err = alog.EmitAuditEvent(SessionEndEvent, EventFields{SessionEventID: sessionID, EventLogin: "bob", EventNamespace: defaults.Namespace})
	c.Assert(alog.loggers.Len(), check.Equals, 0)

	// read the session bytes
	history, err := alog.GetSessionEvents(defaults.Namespace, "100", 2)
	c.Assert(err, check.IsNil)
	c.Assert(history, check.HasLen, 2)

	// make sure offsets were properly set (0 for the first event and 5 bytes for hello):
	c.Assert(history[0][SessionByteOffset], check.Equals, float64(0))
	c.Assert(history[1][SessionByteOffset], check.Equals, float64(len("hello")))
}

// TestCloseOutstanding makes sure the logger closed outstanding sessions
func (a *AuditTestSuite) TestCloseOutstanding(c *check.C) {
	// create audit log, write a couple of events into it, close it
	fakeClock := clockwork.NewFakeClock()
	alogI, err := NewAuditLog(AuditLogConfig{
		DataDir:        a.dataDir,
		RecordSessions: true,
		Clock:          fakeClock,
	})
	c.Assert(err, check.IsNil)
	alog := alogI.(*AuditLog)

	// start the session and emit data stream to it
	err = alog.EmitAuditEvent(SessionStartEvent, EventFields{SessionEventID: "100", EventLogin: "bob", EventNamespace: defaults.Namespace})
	c.Assert(err, check.IsNil)
	c.Assert(alog.loggers.Len(), check.Equals, 1)

	alog.Close()
	c.Assert(alog.loggers.Len(), check.Equals, 0)
}
