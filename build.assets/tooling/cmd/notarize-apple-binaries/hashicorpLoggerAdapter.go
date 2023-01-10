/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/sirupsen/logrus"
)

// All of our other tools use logrus for logging, but the gon package
// uses https://pkg.go.dev/github.com/hashicorp/go-hclog. This is an
// adapter that will send hclog output to logrus.
type HCLogLogrusAdapter struct {
	name        string
	logger      *logrus.Logger
	impliedArgs []interface{}
}

func NewHCLogLogrusAdapter() HCLogLogrusAdapter {
	return HCLogLogrusAdapter{
		logger: logrus.StandardLogger(),
	}
}

// Args are alternating key, val pairs
// keys must be strings
// vals can be any type, but display is implementation specific
// Emit a message and key/value pairs at a provided log level
func (hclla HCLogLogrusAdapter) Log(level hclog.Level, msg string, args ...interface{}) {
	logrusLevel := hclogLevelToLogrusLevel(level)
	msgWithImpliedArgs := fmt.Sprintf("%s %%s", msg)
	argsWithImpliedArgsString := append(args, hclla.createImpliedArgsString())
	hclla.logger.Logf(logrusLevel, msgWithImpliedArgs, argsWithImpliedArgsString)
}

// Emit a message and key/value pairs at the TRACE level
func (hclla HCLogLogrusAdapter) Trace(msg string, args ...interface{}) {
	hclla.Log(hclog.Trace, msg, args)
}

// Emit a message and key/value pairs at the DEBUG level
func (hclla HCLogLogrusAdapter) Debug(msg string, args ...interface{}) {
	hclla.Log(hclog.Debug, msg, args)
}

// Emit a message and key/value pairs at the INFO level
func (hclla HCLogLogrusAdapter) Info(msg string, args ...interface{}) {
	hclla.Log(hclog.Info, msg, args)
}

// Emit a message and key/value pairs at the WARN level
func (hclla HCLogLogrusAdapter) Warn(msg string, args ...interface{}) {
	hclla.Log(hclog.Warn, msg, args)
}

// Emit a message and key/value pairs at the ERROR level
func (hclla HCLogLogrusAdapter) Error(msg string, args ...interface{}) {
	hclla.Log(hclog.Error, msg, args)
}

// Indicate if TRACE logs would be emitted. This and the other Is* guards
// are used to elide expensive logging code based on the current level.
func (hclla HCLogLogrusAdapter) IsTrace() bool {
	return hclla.logger.IsLevelEnabled(logrus.TraceLevel)
}

// Indicate if DEBUG logs would be emitted. This and the other Is* guards
func (hclla HCLogLogrusAdapter) IsDebug() bool {
	return hclla.logger.IsLevelEnabled(logrus.DebugLevel)
}

// Indicate if INFO logs would be emitted. This and the other Is* guards
func (hclla HCLogLogrusAdapter) IsInfo() bool {
	return hclla.logger.IsLevelEnabled(logrus.InfoLevel)
}

// Indicate if WARN logs would be emitted. This and the other Is* guards
func (hclla HCLogLogrusAdapter) IsWarn() bool {
	return hclla.logger.IsLevelEnabled(logrus.WarnLevel)
}

// Indicate if ERROR logs would be emitted. This and the other Is* guards
func (hclla HCLogLogrusAdapter) IsError() bool {
	return hclla.logger.IsLevelEnabled(logrus.ErrorLevel)
}

// ImpliedArgs returns With key/value pairs
func (hclla HCLogLogrusAdapter) ImpliedArgs() []interface{} {
	return hclla.impliedArgs
}

// Creates a sublogger that will always have the given key/value pairs
func (HCLogLogrusAdapter) With(args ...interface{}) hclog.Logger {
	// Ensure that there is a key for every value
	if len(args)%2 != 0 {
		extraValue := args[len(args)-1]
		// Pulled from https://github.com/hashicorp/go-hclog/blob/main/intlogger.go#L731
		args[len(args)-1] = "EXTRA_VALUE_AT_END"
		args = append(args, extraValue)
	}

	newLogger := NewHCLogLogrusAdapter()
	newLogger.impliedArgs = append(newLogger.impliedArgs, args)
	return newLogger
}

// Returns the Name of the logger
func (hclla HCLogLogrusAdapter) Name() string {
	return hclla.name
}

// Create a logger that will prepend the name string on the front of all messages.
// If the logger already has a name, the new value will be appended to the current
// name. That way, a major subsystem can use this to decorate all it's own logs
// without losing context.
func (hclla HCLogLogrusAdapter) Named(name string) hclog.Logger {
	if hclla.name != "" {
		name = fmt.Sprintf("%s.%s", name, hclla.name)
	}

	return hclla.ResetNamed(name)
}

// Create a logger that will prepend the name string on the front of all messages.
// This sets the name of the logger to the value directly, unlike Named which honor
// the current name as well.
func (HCLogLogrusAdapter) ResetNamed(name string) hclog.Logger {
	newLogger := NewHCLogLogrusAdapter()
	newLogger.name = name

	return newLogger
}

// Updates the level. This should affect all related loggers as well,
// unless they were created with IndependentLevels. If an
// implementation cannot update the level on the fly, it should no-op.
func (hclla HCLogLogrusAdapter) SetLevel(level hclog.Level) {
	hclla.logger.SetLevel(hclogLevelToLogrusLevel(level))
}

// Returns the current level
func (hclla HCLogLogrusAdapter) GetLevel() hclog.Level {
	switch hclla.logger.GetLevel() {
	case logrus.FatalLevel:
	case logrus.PanicLevel:
	case logrus.ErrorLevel:
		return hclog.Error
	case logrus.WarnLevel:
		return hclog.Warn
	case logrus.InfoLevel:
		return hclog.Info
	case logrus.DebugLevel:
		return hclog.Debug
	case logrus.TraceLevel:
		return hclog.Trace
	}

	return hclog.NoLevel
}

// Return a value that conforms to the stdlib log.Logger interface
// Options are ignored as it's not currently worth the time to implement them
func (hclla HCLogLogrusAdapter) StandardLogger(options *hclog.StandardLoggerOptions) *log.Logger {
	return log.New(hclla.StandardWriter(options), "", 0)
}

// Return a value that conforms to io.Writer, which can be passed into log.SetOutput()
// Options are ignored as it's not currently worth the time to implement them
func (hclla HCLogLogrusAdapter) StandardWriter(_ *hclog.StandardLoggerOptions) io.Writer {
	logrusOut := hclla.logger.Out
	if logrusOut != nil {
		return logrusOut
	}

	return os.Stderr
}

func hclogLevelToLogrusLevel(level hclog.Level) logrus.Level {
	// The docs for hclog also list a "Off" level, but it's not defined in the version of hclog used by gon.
	switch level {
	case hclog.Error:
		return logrus.ErrorLevel
	case hclog.Warn:
		return logrus.WarnLevel
	case hclog.Info:
		return logrus.InfoLevel
	case hclog.Debug:
		return logrus.DebugLevel
	case hclog.Trace:
		return logrus.TraceLevel
	case hclog.NoLevel:
		return logrus.InfoLevel
	}

	return logrus.InfoLevel
}

func (hclla HCLogLogrusAdapter) createImpliedArgsString() string {
	keyPairCount := len(hclla.impliedArgs) / 2
	keyPairString := ""
	for keyPairNumber := 0; keyPairNumber < keyPairCount; keyPairNumber++ {
		if keyPairString != "" {
			keyPairString = fmt.Sprintf("%s, ", keyPairString)
		}
		keyPairString = fmt.Sprintf("%s%+v=%+v", keyPairString, hclla.impliedArgs[2*keyPairNumber], hclla.impliedArgs[(2*keyPairNumber)+1])
	}

	return keyPairString
}
