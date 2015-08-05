package log

import (
	"io"
)

// fanOutLoger outputs the logs to the underlying logger
type fanOutLogger struct {
	loggers []Logger

	info  io.Writer
	warn  io.Writer
	err   io.Writer
	fatal io.Writer
}

func newFanOut() *fanOutLogger {
	fl := &fanOutLogger{
		loggers: []Logger{},
	}
	fl.info = &fanOutWriter{l: fl, sev: SeverityInfo}
	fl.warn = &fanOutWriter{l: fl, sev: SeverityWarn}
	fl.err = &fanOutWriter{l: fl, sev: SeverityError}
	fl.fatal = &fanOutWriter{l: fl, sev: SeverityFatal}
	return fl
}

func (l *fanOutLogger) setLoggers(lg ...Logger) {
	l.loggers = []Logger{}
	l.loggers = append(l.loggers, lg...)
}

func (l *fanOutLogger) add(lg Logger) {
	l.loggers = append(l.loggers, lg)
}

func (l *fanOutLogger) Writer(sev Severity) io.Writer {
	switch sev {
	case SeverityInfo:
		return l.info
	case SeverityWarn:
		return l.warn
	default:
		return l.err
	}
}

func (l *fanOutLogger) Infof(format string, args ...interface{}) {
	if currentSeverity.Gt(SeverityInfo) {
		return
	}
	infof(1, l.info, format, args...)
}

func (l *fanOutLogger) Warningf(format string, args ...interface{}) {
	if currentSeverity.Gt(SeverityWarn) {
		return
	}
	warningf(1, l.warn, format, args...)
}

func (l *fanOutLogger) Errorf(format string, args ...interface{}) {
	if currentSeverity.Gt(SeverityError) {
		return
	}
	errorf(1, l.err, format, args...)
}

func (l *fanOutLogger) Fatalf(format string, args ...interface{}) {
	if currentSeverity.Gt(SeverityFatal) {
		return
	}
	fatalf(1, l.fatal, format, args...)
	exit()
}

var logger = newFanOut()

type fanOutWriter struct {
	sev Severity
	l   *fanOutLogger
}

func (w *fanOutWriter) Write(val []byte) (ln int, err error) {
	for i := range w.l.loggers {
		ln, err = w.l.loggers[i].Writer(w.sev).Write(val)
		if err != nil {
			return ln, err
		}
	}
	return ln, err
}
