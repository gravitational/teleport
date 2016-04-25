package events

import (
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
	// create audit log, write a couple of events into it, close it
	alog, err := NewAuditLog(a.dataDir, true)
	c.Assert(err, check.IsNil)
	alog.TimeSource = func() time.Time { return now }

	// emit two session-attached events (same session)
	err = alog.EmitAuditEvent(SessionStartEvent, EventFields{SessionEventID: "100", SessionEventLogin: "vincent"})
	c.Assert(err, check.IsNil)
	c.Assert(alog.loggers, check.HasLen, 1)
	err = alog.EmitAuditEvent(SessionLeaveEvent, EventFields{SessionEventID: "100", SessionEventLogin: "vincent"})
	c.Assert(alog.loggers, check.HasLen, 1)
	err = alog.EmitAuditEvent(SessionJoinEvent, EventFields{SessionEventID: "200", SessionEventLogin: "doggy"})
	c.Assert(err, check.IsNil)
	c.Assert(alog.loggers, check.HasLen, 2)

	// emit "sesion-end" event. one of the loggers must disappear
	err = alog.EmitAuditEvent(SessionEndEvent, EventFields{SessionEventID: "200", SessionEventLogin: "doggy"})
	c.Assert(err, check.IsNil)
	c.Assert(alog.loggers, check.HasLen, 1)

	// add a few more loggers and close:
	alog.EmitAuditEvent(SessionJoinEvent, EventFields{SessionEventID: "300", SessionEventLogin: "frankie"})
	alog.EmitAuditEvent(SessionJoinEvent, EventFields{SessionEventID: "400", SessionEventLogin: "rosie"})
	alog.Close()
	c.Assert(alog.loggers, check.HasLen, 0)
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
	c.Assert(string(bytes), check.Equals, now.String()+",user.farted,{\"apples?\":\"yes\"}\n")
}
