package log

import (
	"fmt"
	"io"
	"os"
	"time"
)

// writerLogger outputs the logs to the underlying writer
type writerLogger struct {
	w io.Writer
}

func NewConsoleLogger(config *LogConfig) (Logger, error) {
	return &writerLogger{w: os.Stdout}, nil
}

func (l *writerLogger) Writer(sev Severity) io.Writer {
	return l
}

func (l *writerLogger) Write(val []byte) (int, error) {
	return io.WriteString(
		l.w,
		fmt.Sprintf("%v: %v\n", time.Now().UTC().Format(time.StampMilli),
			string(val)))
}

func (l *writerLogger) Infof(format string, args ...interface{}) {
	infof(1, l, format, args...)
}

func (l *writerLogger) Warningf(format string, args ...interface{}) {
	warningf(1, l, format, args...)
}

func (l *writerLogger) Errorf(format string, args ...interface{}) {
	errorf(1, l, format, args...)
}

func (l *writerLogger) Fatalf(format string, args ...interface{}) {
	fatalf(1, l, format, args...)
}
