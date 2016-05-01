package events

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"gopkg.in/check.v1"

	"github.com/gravitational/teleport/lib/utils"
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

func (a *AuditTestSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
	a.dataDir = c.MkDir()
}

func (a *AuditTestSuite) TestNew(c *check.C) {
	alog, err := NewAuditLog(a.dataDir, true)
	c.Assert(err, check.IsNil)
	// close twice:
	c.Assert(alog.Close(), check.IsNil)
	c.Assert(alog.Close(), check.IsNil)
}

func (a *AuditTestSuite) TestComplexLogging(c *check.C) {
	now := time.Now().In(time.UTC).Round(time.Second)
	os.RemoveAll(a.dataDir)

	// create audit log, write a couple of events into it, close it
	alog, err := NewAuditLog(a.dataDir, true)
	c.Assert(err, check.IsNil)
	alog.TimeSource = func() time.Time { return now }

	// emit two session-attached events (same session)
	err = alog.EmitAuditEvent(SessionStartEvent, EventFields{SessionEventID: "100", EventLogin: "vincent"})
	c.Assert(err, check.IsNil)
	c.Assert(alog.loggers, check.HasLen, 1)
	err = alog.EmitAuditEvent(SessionLeaveEvent, EventFields{SessionEventID: "100", EventLogin: "vincent"})
	c.Assert(alog.loggers, check.HasLen, 1)
	err = alog.EmitAuditEvent(SessionJoinEvent, EventFields{SessionEventID: "200", EventLogin: "doggy"})
	c.Assert(err, check.IsNil)
	c.Assert(alog.loggers, check.HasLen, 2)

	// type "hello" into session "200":
	writer, err := alog.GetSessionWriter("200")
	c.Assert(err, check.IsNil)
	n, err := writer.Write([]byte("hello"))
	c.Assert(err, check.IsNil)
	c.Assert(n, check.Equals, len("hello"))

	// emit "sesion-end" event. one of the loggers must disappear
	err = alog.EmitAuditEvent(SessionEndEvent, EventFields{SessionEventID: "200", EventLogin: "doggy"})
	c.Assert(err, check.IsNil)
	c.Assert(alog.loggers, check.HasLen, 1)

	// add a few more loggers and close:
	alog.EmitAuditEvent(SessionJoinEvent, EventFields{SessionEventID: "300", EventLogin: "frankie"})
	alog.EmitAuditEvent(SessionJoinEvent, EventFields{SessionEventID: "400", EventLogin: "rosie"})
	alog.Close()
	c.Assert(alog.loggers, check.HasLen, 0)

	// inspect session "200". it sould have three events: join, print and leave:
	history, err := alog.GetSessionEvents("200", 0)
	c.Assert(err, check.IsNil)
	c.Assert(history, check.HasLen, 3)
	c.Assert(history[0][EventLogin], check.Equals, "doggy")
	c.Assert(history[0][EventType], check.Equals, SessionJoinEvent)
	c.Assert(history[1][EventType], check.Equals, SessionPrintEvent)
	c.Assert(history[1][SessionByteOffset].(float64), check.Equals, float64(0))
	c.Assert(history[1][SessionPrintEventBytes].(float64), check.Equals, float64(5))
	c.Assert(history[2][EventType], check.Equals, SessionEndEvent)

	// try the same, but with 'afterN', we should only get the 3rd event:
	history2, err := alog.GetSessionEvents("200", 2)
	c.Assert(err, check.IsNil)
	c.Assert(history2, check.HasLen, 1)
	c.Assert(history2[0], check.DeepEquals, history[2])
	writer.Close()

	// lets try session reader (with offset 2 of bytes, i.e. instead of "hello" we should get "llo")
	reader, err := alog.GetSessionReader("200", 2)
	c.Assert(err, check.IsNil)
	defer reader.Close()
	buff := make([]byte, 100)
	n, err = reader.Read(buff)
	c.Assert(err, check.IsNil)
	c.Assert(n, check.Equals, 3)
	c.Assert(string(buff[:3]), check.Equals, "llo")
	n, err = reader.Read(buff)
	c.Assert(err, check.Equals, io.EOF)
	c.Assert(n, check.Equals, 0)

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
	fmt.Println(found[0])
	c.Assert(found[0].GetString(EventLogin), check.Equals, "vincent")
}

func (a *AuditTestSuite) TestBasicLogging(c *check.C) {
	now := time.Now().In(time.UTC).Round(time.Second)
	// create audit log, write a couple of events into it, close it
	alog, err := NewAuditLog(a.dataDir, true)
	c.Assert(err, check.IsNil)
	alog.TimeSource = func() time.Time { return now }

	// emit regular event:
	err = alog.EmitAuditEvent("user.farted", EventFields{"apples?": "yes"})
	c.Assert(err, check.IsNil)
	logfile := alog.file.Name()
	c.Assert(alog.Close(), check.IsNil)

	// read back what's been written:
	bytes, err := ioutil.ReadFile(logfile)
	c.Assert(err, check.IsNil)
	c.Assert(string(bytes), check.Equals,
		fmt.Sprintf("{\"apples?\":\"yes\",\"event\":\"user.farted\",\"time\":\"%s\"}\n", now.Format(time.RFC3339)))
}
