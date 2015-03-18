package log

import (
	"io"
	"log/syslog"
	"os"
	"path/filepath"
)

// Syslogger sends all your logs to syslog
// Note: the logs are going to MAIL_LOG facility
type sysLogger struct {
	info *syslog.Writer
	warn *syslog.Writer
	err  *syslog.Writer
}

var newSyslogWriter = syslog.New // for mocking in tests

func NewSysLogger(config *LogConfig) (Logger, error) {
	info, err := newSyslogWriter(syslog.LOG_MAIL|syslog.LOG_INFO, getAppName())
	if err != nil {
		return nil, err
	}

	warn, err := newSyslogWriter(syslog.LOG_MAIL|syslog.LOG_WARNING, getAppName())
	if err != nil {
		return nil, err
	}

	error, err := newSyslogWriter(syslog.LOG_MAIL|syslog.LOG_ERR, getAppName())
	if err != nil {
		return nil, err
	}

	return &sysLogger{
		info: info,
		warn: warn,
		err:  error,
	}, nil
}

// Get process name
func getAppName() string {
	return filepath.Base(os.Args[0])
}

func (l *sysLogger) Writer(sev Severity) io.Writer {
	switch sev {
	case SeverityInfo:
		return l.info
	case SeverityWarn:
		return l.warn
	default:
		return l.err
	}
}

func (l *sysLogger) Infof(format string, args ...interface{}) {
	infof(1, l.Writer(SeverityInfo), format, args...)
}

func (l *sysLogger) Warningf(format string, args ...interface{}) {
	warningf(1, l.Writer(SeverityWarn), format, args...)
}

func (l *sysLogger) Errorf(format string, args ...interface{}) {
	errorf(1, l.Writer(SeverityError), format, args...)
}

func (l *sysLogger) Fatalf(format string, args ...interface{}) {
	fatalf(1, l.Writer(SeverityFatal), format, args...)
}
