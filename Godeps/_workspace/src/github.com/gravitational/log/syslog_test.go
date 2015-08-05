package log

import (
	"errors"
	"log/syslog"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

type SysLogSuite struct {
	logger Logger
}

var _ = Suite(&SysLogSuite{})

func (s *SysLogSuite) SetUpSuite(c *C) {
	config := &LogConfig{Name: "test"}
	s.logger, _ = NewSysLogger(config)
}

func (s *SysLogSuite) TestNewSysLogger(c *C) {
	config := &LogConfig{Name: "syslog"}
	logger, err := NewSysLogger(config)
	c.Assert(logger, NotNil)
	c.Assert(err, IsNil)
}

func (s *SysLogSuite) TestNewSysLoggerError(c *C) {
	config := &LogConfig{Name: "syslog"}
	newSyslogWriter = func(int syslog.Priority, tag string) (*syslog.Writer, error) {
		return nil, errors.New("Error")
	}

	logger, err := NewSysLogger(config)
	c.Assert(logger, IsNil)
	c.Assert(err, NotNil)
}

func (s *SysLogSuite) TestInfo(c *C) {
	s.logger.Infof("test message")
}

func (s *SysLogSuite) TestWarning(c *C) {
	s.logger.Warningf("test message")
}

func (s *SysLogSuite) TestError(c *C) {
	s.logger.Errorf("test message")
}

func (s *SysLogSuite) TestFatal(c *C) {
	s.logger.Fatalf("test message")
}
