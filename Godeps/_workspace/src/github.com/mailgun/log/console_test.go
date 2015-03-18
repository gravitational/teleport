package log

import (
	"bytes"
	"strings"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

type ConsoleLogSuite struct {
	out *bytes.Buffer
}

var _ = Suite(&ConsoleLogSuite{})

func (s *ConsoleLogSuite) SetUpTest(c *C) {
	SetSeverity(SeverityInfo)
	s.out = &bytes.Buffer{}
	logger.loggers = []Logger{&writerLogger{w: s.out}}
	runtimeCaller = func(skip int) (pc uintptr, file string, line int, ok bool) {
		return 0, "", 0, false
	}
	exit = func() {}
}

func (s *ConsoleLogSuite) TearDownTest(c *C) {
	logger.loggers = []Logger{}
	SetSeverity(SeverityError)
}

func (s *ConsoleLogSuite) output() string {
	return s.out.String()
}

func (s *ConsoleLogSuite) TestNewConsoleLogger(c *C) {
	config := &LogConfig{Name: "testNew"}
	logger, err := NewConsoleLogger(config)
	c.Assert(logger, NotNil)
	c.Assert(err, IsNil)
}

func (s *ConsoleLogSuite) TestInfo(c *C) {
	Infof("test message")
	c.Assert(s.output(), Matches, ".*INFO.*test message.*\n")
}

func (s *ConsoleLogSuite) TestWarning(c *C) {
	Warningf("test message")
	c.Assert(s.output(), Matches, ".*WARN.*test message.*\n")
}

func (s *ConsoleLogSuite) TestError(c *C) {
	Errorf("test message")
	c.Assert(s.output(), Matches, ".*ERROR.*test message.*\n")
}

func (s *ConsoleLogSuite) TestFatal(c *C) {
	Fatalf("test message")
	c.Assert(strings.Split(s.output(), "\n")[0], Matches, ".*FATAL.*test message")
}

func (s *ConsoleLogSuite) TestUpperLevel(c *C) {
	SetSeverity(SeverityError)
	Infof("info message")
	Errorf("error message")
	c.Assert(s.output(), Matches, ".*ERROR.*error message.*\n")
}

func (s *ConsoleLogSuite) TestUpdateLevel(c *C) {
	SetSeverity(SeverityError)
	Infof("info message")
	c.Assert(s.output(), Equals, "")

	SetSeverity(SeverityInfo)
	Infof("info message")
	c.Assert(s.output(), Matches, ".*INFO.*info message.*\n")
}
