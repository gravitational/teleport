package log

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/siddontang/go-log/loggers"
)

const (
	timeFormat     = "2006/01/02 15:04:05"
	maxBufPoolSize = 16
)

// Logger flag
const (
	Ltime  = 1 << iota // time format "2006/01/02 15:04:05"
	Lfile              // file.go:123
	Llevel             // [Trace|Debug|Info...]
)

// Level type
type Level int

// Log level, from low to high, more high means more serious
const (
	LevelTrace Level = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

// String returns level String
func (l Level) String() string {
	switch l {
	case LevelTrace:
		return "trace"
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	case LevelFatal:
		return "fatal"
	}
	// return default info
	return "info"
}

// Logger is the logger to record log
type Logger struct {
	// TODO: support logger.Contextual
	loggers.Advanced

	level Level
	flag  int

	hLock   sync.Mutex
	handler Handler

	bufs sync.Pool
}

// New creates a logger with specified handler and flag
func New(handler Handler, flag int) *Logger {
	var l = new(Logger)

	l.level = LevelInfo
	l.handler = handler

	l.flag = flag

	l.bufs = sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, 1024)
		},
	}

	return l
}

// NewDefault creates default logger with specified handler and flag: Ltime|Lfile|Llevel
func NewDefault(handler Handler) *Logger {
	return New(handler, Ltime|Lfile|Llevel)
}

func newStdHandler() *StreamHandler {
	h, _ := NewStreamHandler(os.Stdout)
	return h
}

// Close closes the logger
func (l *Logger) Close() {
	l.hLock.Lock()
	defer l.hLock.Unlock()
	l.handler.Close()
}

// SetLevel sets log level, any log level less than it will not log
func (l *Logger) SetLevel(level Level) {
	l.level = level
}

// SetLevelByName sets log level by name
func (l *Logger) SetLevelByName(name string) {
	level := LevelInfo
	switch strings.ToLower(name) {
	case "trace":
		level = LevelTrace
	case "debug":
		level = LevelDebug
	case "warn", "warning":
		level = LevelWarn
	case "error":
		level = LevelError
	case "fatal":
		level = LevelFatal
	default:
		level = LevelInfo
	}

	l.SetLevel(level)
}

// Output records the log with special callstack depth and log level.
func (l *Logger) Output(callDepth int, level Level, msg string) {
	if l.level > level {
		return
	}

	buf := l.bufs.Get().([]byte)
	buf = buf[0:0]
	defer l.bufs.Put(buf)

	if l.flag&Ltime > 0 {
		now := time.Now().Format(timeFormat)
		buf = append(buf, '[')
		buf = append(buf, now...)
		buf = append(buf, "] "...)
	}

	if l.flag&Llevel > 0 {
		buf = append(buf, '[')
		buf = append(buf, level.String()...)
		buf = append(buf, "] "...)
	}

	if l.flag&Lfile > 0 {
		_, file, line, ok := runtime.Caller(callDepth)
		if !ok {
			file = "???"
			line = 0
		} else {
			for i := len(file) - 1; i > 0; i-- {
				if file[i] == '/' {
					file = file[i+1:]
					break
				}
			}
		}

		buf = append(buf, file...)
		buf = append(buf, ':')

		buf = strconv.AppendInt(buf, int64(line), 10)
		buf = append(buf, ' ')
	}

	buf = append(buf, msg...)
	if len(msg) == 0 || msg[len(msg)-1] != '\n' {
		buf = append(buf, '\n')
	}

	l.hLock.Lock()
	l.handler.Write(buf)
	l.hLock.Unlock()
}

// Fatal records the log with fatal level and exits
func (l *Logger) Fatal(args ...interface{}) {
	l.Output(2, LevelFatal, fmt.Sprint(args...))
	os.Exit(1)
}

// Fatalf records the log with fatal level and exits
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.Output(2, LevelFatal, fmt.Sprintf(format, args...))
	os.Exit(1)
}

// Fatalln records the log with fatal level and exits
func (l *Logger) Fatalln(args ...interface{}) {
	l.Output(2, LevelFatal, fmt.Sprintln(args...))
	os.Exit(1)
}

// Panic records the log with fatal level and panics
func (l *Logger) Panic(args ...interface{}) {
	msg := fmt.Sprint(args...)
	l.Output(2, LevelError, msg)
	panic(msg)
}

// Panicf records the log with fatal level and panics
func (l *Logger) Panicf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Output(2, LevelError, msg)
	panic(msg)
}

// Panicln records the log with fatal level and panics
func (l *Logger) Panicln(args ...interface{}) {
	msg := fmt.Sprintln(args...)
	l.Output(2, LevelError, msg)
	panic(msg)
}

// Print records the log with trace level
func (l *Logger) Print(args ...interface{}) {
	l.Output(2, LevelTrace, fmt.Sprint(args...))
}

// Printf records the log with trace level
func (l *Logger) Printf(format string, args ...interface{}) {
	l.Output(2, LevelTrace, fmt.Sprintf(format, args...))
}

// Println records the log with trace level
func (l *Logger) Println(args ...interface{}) {
	l.Output(2, LevelTrace, fmt.Sprintln(args...))
}

// Debug records the log with debug level
func (l *Logger) Debug(args ...interface{}) {
	l.Output(2, LevelDebug, fmt.Sprint(args...))
}

// Debugf records the log with debug level
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.Output(2, LevelDebug, fmt.Sprintf(format, args...))
}

// Debugln records the log with debug level
func (l *Logger) Debugln(args ...interface{}) {
	l.Output(2, LevelDebug, fmt.Sprintln(args...))
}

// Error records the log with error level
func (l *Logger) Error(args ...interface{}) {
	l.Output(2, LevelError, fmt.Sprint(args...))
}

// Errorf records the log with error level
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Output(2, LevelError, fmt.Sprintf(format, args...))
}

// Errorln records the log with error level
func (l *Logger) Errorln(args ...interface{}) {
	l.Output(2, LevelError, fmt.Sprintln(args...))
}

// Info records the log with info level
func (l *Logger) Info(args ...interface{}) {
	l.Output(2, LevelInfo, fmt.Sprint(args...))
}

// Infof records the log with info level
func (l *Logger) Infof(format string, args ...interface{}) {
	l.Output(2, LevelInfo, fmt.Sprintf(format, args...))
}

// Infoln records the log with info level
func (l *Logger) Infoln(args ...interface{}) {
	l.Output(2, LevelInfo, fmt.Sprintln(args...))
}

// Warn records the log with warn level
func (l *Logger) Warn(args ...interface{}) {
	l.Output(2, LevelWarn, fmt.Sprint(args...))
}

// Warnf records the log with warn level
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.Output(2, LevelWarn, fmt.Sprintf(format, args...))
}

// Warnln records the log with warn level
func (l *Logger) Warnln(args ...interface{}) {
	l.Output(2, LevelWarn, fmt.Sprintln(args...))
}
