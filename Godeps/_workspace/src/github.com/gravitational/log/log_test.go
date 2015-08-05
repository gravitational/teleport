package log

import (
	"io/ioutil"
	"testing"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestModel(t *testing.T) { TestingT(t) }

type LogSuite struct{}

var _ = Suite(&LogSuite{})

func (s *LogSuite) SetUpTest(c *C) {
	// mock exit function
	runtimeCaller = func(skip int) (pc uintptr, file string, line int, ok bool) {
		return 0, "", 0, false
	}
	exit = func() {}
	SetSeverity(SeverityInfo)
}

func (s *LogSuite) TearDownTest(c *C) {
	SetSeverity(SeverityInfo)
}

func (s *LogSuite) SetUpSuite(c *C) {
	consoleConfig := &LogConfig{Name: "console"}
	syslogConfig := &LogConfig{Name: "syslog"}
	err := Init([]*LogConfig{consoleConfig, syslogConfig})
	c.Assert(err, IsNil)
	for _, l := range logger.loggers {
		if cl, ok := l.(*writerLogger); ok {
			cl.w = ioutil.Discard
		}
	}
}

func (s *LogSuite) TestInitError(c *C) {
	unknownConfig := &LogConfig{Name: "unknown"}
	err := Init([]*LogConfig{unknownConfig})
	c.Assert(err, NotNil)
	c.Assert(logger.loggers, HasLen, 2)
}

func (s *LogSuite) TestInfof(c *C) {
	Infof("test message, %v", "info")
}

func (s *LogSuite) TestWarningf(c *C) {
	Warningf("test message, %v", "warning")
}

func (s *LogSuite) TestErrorf(c *C) {
	Errorf("test message, %v", "error")
}

func (s *LogSuite) TestFatalf(c *C) {
	Fatalf("test message, %v", "fatal")
}

func (s *LogSuite) TestCallerInfoError(c *C) {
	file, line := callerInfo(3)
	c.Assert(file, Equals, "unknown")
	c.Assert(line, Equals, 0)
}

func (s *LogSuite) TestGetSetSeverity(c *C) {
	for sev := range severityName {
		SetSeverity(sev)
		c.Assert(GetSeverity(), Equals, sev)
	}
}

func (s *LogSuite) TestSeverityFromString(c *C) {
	for sev, name := range severityName {
		out, err := SeverityFromString(name)
		c.Assert(err, IsNil)
		c.Assert(out, Equals, sev)
	}
}

func (s *LogSuite) TestSeverityToString(c *C) {
	for sev, name := range severityName {
		c.Assert(sev.String(), Equals, name)
	}
}
