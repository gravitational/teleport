package gofakes3

import "log"

type LogLevel string

const (
	LogErr  LogLevel = "ERR"
	LogWarn LogLevel = "WARN"
	LogInfo LogLevel = "INFO"
)

// Logger provides a very minimal target for logging implementations to hit to
// allow arbitrary logging dependencies to be used with GoFakeS3.
//
// Only an interface to the standard library's log package is provided with
// GoFakeS3, other libraries will require an adapter. Adapters are trivial to
// write.
//
// For zap:
//
//	type LogrusLog struct {
//		log *zap.Logger
//	}
//
//	func (l LogrusLog) Print(level LogLevel, v ...interface{}) {
//		switch level {
//		case gofakes3.LogErr:
//			l.log.Error(fmt.Sprint(v...))
//		case gofakes3.LogWarn:
//			l.log.Warn(fmt.Sprint(v...))
//		case gofakes3.LogInfo:
//			l.log.Info(fmt.Sprint(v...))
//		default:
//			panic("unknown level")
//		}
//	}
//
//
// For logrus:
//
//	type LogrusLog struct {
//		log *logrus.Logger
//	}
//
//	func (l LogrusLog) Print(level LogLevel, v ...interface{}) {
//		switch level {
//		case gofakes3.LogErr:
//			l.log.Errorln(v...)
//		case gofakes3.LogWarn:
//			l.log.Warnln(v...)
//		case gofakes3.LogInfo:
//			l.log.Infoln(v...)
//		default:
//			panic("unknown level")
//		}
//	}
//
type Logger interface {
	Print(level LogLevel, v ...interface{})
}

// GlobalLog creates a Logger that uses the global log.Println() function.
//
// All levels are reported by default. If you pass levels to this function,
// it will act as a level whitelist.
func GlobalLog(levels ...LogLevel) Logger {
	return newStdLog(log.Println, levels...)
}

// StdLog creates a Logger that uses the stdlib's log.Logger type.
//
// All levels are reported by default. If you pass levels to this function,
// it will act as a level whitelist.
func StdLog(log *log.Logger, levels ...LogLevel) Logger {
	return newStdLog(log.Println, levels...)
}

// DiscardLog creates a Logger that discards all messages.
func DiscardLog() Logger {
	return &discardLog{}
}

type stdLog struct {
	log    func(v ...interface{})
	levels map[LogLevel]bool
}

func newStdLog(log func(v ...interface{}), levels ...LogLevel) Logger {
	sl := &stdLog{log: log}
	if len(levels) > 0 {
		sl.levels = map[LogLevel]bool{}
		for _, lv := range levels {
			sl.levels[lv] = true
		}
	}
	return sl
}

func (s *stdLog) Print(level LogLevel, v ...interface{}) {
	if s.levels == nil || s.levels[level] {
		v = append(v, nil)
		copy(v[1:], v)
		v[0] = level
		s.log(v...)
	}
}

type discardLog struct{}

func (d discardLog) Print(level LogLevel, v ...interface{}) {}
