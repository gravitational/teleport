package log

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
)

var pid = os.Getpid()
var currentSeverity Severity

// Severity implementation is borrowed from glog, uses sync/atomic int32
type Severity int32

const (
	SeverityInfo Severity = iota
	SeverityWarn
	SeverityError
	SeverityFatal
)

var severityName = map[Severity]string{
	SeverityInfo:  "INFO",
	SeverityWarn:  "WARN",
	SeverityError: "ERROR",
	SeverityFatal: "FATAL",
}

// get returns the value of the severity.
func (s *Severity) Get() Severity {
	return Severity(atomic.LoadInt32((*int32)(s)))
}

// set sets the value of the severity.
func (s *Severity) Set(val Severity) {
	atomic.StoreInt32((*int32)(s), int32(val))
}

// less returns if this severity is greater than passed severity
func (s *Severity) Gt(val Severity) bool {
	return s.Get() > val
}

func (s Severity) String() string {
	n, ok := severityName[s]
	if !ok {
		return "UNKNOWN SEVERITY"
	}
	return n
}

func SeverityFromString(s string) (Severity, error) {
	s = strings.ToUpper(s)
	for k, val := range severityName {
		if val == s {
			return k, nil
		}
	}
	return -1, fmt.Errorf("unsupported severity: %s", s)
}

// Logger is a unified interface for all loggers.
type Logger interface {
	Infof(format string, args ...interface{})
	Warningf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})

	Writer(Severity) io.Writer
}

// Logging configuration to be passed to all loggers during initialization.
type LogConfig struct {
	Name string
}

func (c LogConfig) String() string {
	return fmt.Sprintf("LogConfig(Name=%v)", c.Name)
}

// SetSeverity sets current logging severity. Acceptable values are SeverityInfo, SeverityWarn, SeverityError, SeverityFatal
func SetSeverity(s Severity) {
	currentSeverity.Set(s)
}

// GetSeverity returns currently set severity.
func GetSeverity() Severity {
	return currentSeverity
}

// Logging initialization, must be called at the beginning of your cool app.
func Init(logConfigs []*LogConfig) error {
	for _, config := range logConfigs {
		l, err := NewLogger(config)
		if err != nil {
			return err
		}
		logger.add(l)
	}
	return nil
}

// Make a proper logger from a given configuration.
func NewLogger(config *LogConfig) (Logger, error) {
	switch config.Name {
	case "console":
		return NewConsoleLogger(config)
	case "syslog":
		return NewSysLogger(config)
	}
	return nil, errors.New(fmt.Sprintf("Unknown logger: %v", config))
}

// GetLogger returns global logger
func GetLogger() Logger {
	return logger
}

// Infof logs to the INFO log.
func Infof(format string, args ...interface{}) {
	infof(1, logger.info, format, args...)
}

// Warningf logs to the WARNING and INFO logs.
func Warningf(format string, args ...interface{}) {
	warningf(1, logger.warn, format, args...)
}

// Errorf logs to the ERROR, WARNING, and INFO logs.
func Errorf(format string, args ...interface{}) {
	errorf(1, logger.warn, format, args...)
}

// Fatalf logs to the FATAL, ERROR, WARNING, and INFO logs,
// including a stack trace of all running goroutines, then calls os.Exit(255).
func Fatalf(format string, args ...interface{}) {
	fatalf(1, logger.fatal, format, args...)
}

func infof(depth int, w io.Writer, format string, args ...interface{}) {
	if currentSeverity.Gt(SeverityInfo) {
		return
	}
	writeMessage(depth+1, w, SeverityInfo, format, args...)
}

func warningf(depth int, w io.Writer, format string, args ...interface{}) {
	if currentSeverity.Gt(SeverityWarn) {
		return
	}
	writeMessage(depth+1, w, SeverityWarn, format, args...)
}

func errorf(depth int, w io.Writer, format string, args ...interface{}) {
	if currentSeverity.Gt(SeverityError) {
		return
	}
	writeMessage(depth+1, w, SeverityError, format, args...)
}

func fatalf(depth int, w io.Writer, format string, args ...interface{}) {
	if currentSeverity.Gt(SeverityFatal) {
		return
	}
	writeMessage(depth+1, w, SeverityFatal, format, args...)
	stacks := stackTraces()
	io.WriteString(w, stacks)
}

func writeMessage(depth int, w io.Writer, sev Severity, format string, args ...interface{}) {
	file, line := callerInfo(depth + 1)
	io.WriteString(w, fmt.Sprintf("%s PID:%d [%s:%d] %s", sev, pid, file, line, fmt.Sprintf(format, args...)))
}

// Return stack traces of all the running goroutines.
func stackTraces() string {
	trace := make([]byte, 100000)
	nbytes := runtime.Stack(trace, true)
	return string(trace[:nbytes])
}

// Return a file name and a line number.
func callerInfo(depth int) (string, int) {
	_, file, line, ok := runtimeCaller(depth + 1) // number of frames to the user's call.

	if !ok {
		file = "unknown"
		line = 0
	} else {
		slashPosition := strings.LastIndex(file, "/")
		if slashPosition >= 0 {
			file = file[slashPosition+1:]
		}
	}

	return file, line
}

// runtime functions for mocking
var runtimeCaller = runtime.Caller

var exit = func() {
	os.Exit(255)
}
