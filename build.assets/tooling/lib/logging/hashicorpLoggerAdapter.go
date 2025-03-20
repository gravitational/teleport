/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

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

func NewHCLogLogrusAdapter(logrusLogger *logrus.Logger) *HCLogLogrusAdapter {
	return &HCLogLogrusAdapter{
		logger: logrusLogger,
	}
}

// Args are alternating key, val pairs
// keys must be strings
// vals can be any type, but display is implementation specific
// Emit a message and key/value pairs at a provided log level
func (h *HCLogLogrusAdapter) Log(level hclog.Level, msg string, args ...interface{}) {
	logrusLevel := hclogLevelToLogrusLevel(level)
	msgWithImpliedArgs := fmt.Sprintf("%s %%s", msg)
	argsWithImpliedArgsString := append(args, h.createImpliedArgsString())
	h.logger.Logf(logrusLevel, msgWithImpliedArgs, argsWithImpliedArgsString)
}

// Emit a message and key/value pairs at the TRACE level
func (h *HCLogLogrusAdapter) Trace(msg string, args ...interface{}) {
	h.Log(hclog.Trace, msg, args)
}

// Emit a message and key/value pairs at the DEBUG level
func (h *HCLogLogrusAdapter) Debug(msg string, args ...interface{}) {
	h.Log(hclog.Debug, msg, args)
}

// Emit a message and key/value pairs at the INFO level
func (h *HCLogLogrusAdapter) Info(msg string, args ...interface{}) {
	h.Log(hclog.Info, msg, args)
}

// Emit a message and key/value pairs at the WARN level
func (h *HCLogLogrusAdapter) Warn(msg string, args ...interface{}) {
	h.Log(hclog.Warn, msg, args)
}

// Emit a message and key/value pairs at the ERROR level
func (h *HCLogLogrusAdapter) Error(msg string, args ...interface{}) {
	h.Log(hclog.Error, msg, args)
}

// Indicate if TRACE logs would be emitted. This and the other Is* guards
// are used to elide expensive logging code based on the current level.
func (h *HCLogLogrusAdapter) IsTrace() bool {
	return h.logger.IsLevelEnabled(logrus.TraceLevel)
}

// Indicate if DEBUG logs would be emitted. This and the other Is* guards
func (h *HCLogLogrusAdapter) IsDebug() bool {
	return h.logger.IsLevelEnabled(logrus.DebugLevel)
}

// Indicate if INFO logs would be emitted. This and the other Is* guards
func (h *HCLogLogrusAdapter) IsInfo() bool {
	return h.logger.IsLevelEnabled(logrus.InfoLevel)
}

// Indicate if WARN logs would be emitted. This and the other Is* guards
func (h *HCLogLogrusAdapter) IsWarn() bool {
	return h.logger.IsLevelEnabled(logrus.WarnLevel)
}

// Indicate if ERROR logs would be emitted. This and the other Is* guards
func (h *HCLogLogrusAdapter) IsError() bool {
	return h.logger.IsLevelEnabled(logrus.ErrorLevel)
}

// ImpliedArgs returns With key/value pairs
func (h *HCLogLogrusAdapter) ImpliedArgs() []interface{} {
	return h.impliedArgs
}

// Creates a sublogger that will always have the given key/value pairs
func (h *HCLogLogrusAdapter) With(args ...interface{}) hclog.Logger {
	// Ensure that there is a key for every value
	if len(args)%2 != 0 {
		extraValue := args[len(args)-1]
		// Pulled from https://github.com/hashicorp/go-hclog/blob/main/intlogger.go#L731
		args[len(args)-1] = "EXTRA_VALUE_AT_END"
		args = append(args, extraValue)
	}

	newLogger := NewHCLogLogrusAdapter(h.logger)
	newLogger.impliedArgs = append(newLogger.impliedArgs, args)
	return newLogger
}

// Returns the Name of the logger
func (h *HCLogLogrusAdapter) Name() string {
	return h.name
}

// Create a logger that will prepend the name string on the front of all messages.
// If the logger already has a name, the new value will be appended to the current
// name. That way, a major subsystem can use this to decorate all it's own logs
// without losing context.
func (h *HCLogLogrusAdapter) Named(name string) hclog.Logger {
	if h.name != "" {
		name = fmt.Sprintf("%s.%s", name, h.name)
	}

	return h.ResetNamed(name)
}

// Create a logger that will prepend the name string on the front of all messages.
// This sets the name of the logger to the value directly, unlike Named which honor
// the current name as well.
func (h *HCLogLogrusAdapter) ResetNamed(name string) hclog.Logger {
	newLogger := NewHCLogLogrusAdapter(h.logger)
	newLogger.name = name

	return newLogger
}

// Updates the level. This should affect all related loggers as well,
// unless they were created with IndependentLevels. If an
// implementation cannot update the level on the fly, it should no-op.
func (h *HCLogLogrusAdapter) SetLevel(level hclog.Level) {
	h.logger.SetLevel(hclogLevelToLogrusLevel(level))
}

// Returns the current level
func (h *HCLogLogrusAdapter) GetLevel() hclog.Level {
	switch h.logger.GetLevel() {
	case logrus.FatalLevel, logrus.PanicLevel, logrus.ErrorLevel:
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
func (h *HCLogLogrusAdapter) StandardLogger(options *hclog.StandardLoggerOptions) *log.Logger {
	return log.New(h.StandardWriter(options), "", 0)
}

// Return a value that conforms to io.Writer, which can be passed into log.SetOutput()
// Options are ignored as it's not currently worth the time to implement them
func (h *HCLogLogrusAdapter) StandardWriter(_ *hclog.StandardLoggerOptions) io.Writer {
	logrusOut := h.logger.Out
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

func (h *HCLogLogrusAdapter) createImpliedArgsString() string {
	keyPairCount := len(h.impliedArgs) / 2
	var argsStringBuilder strings.Builder
	for keyPairNumber := 0; keyPairNumber < keyPairCount; keyPairNumber++ {
		if argsStringBuilder.Len() != 0 {
			fmt.Fprint(&argsStringBuilder, ", ")
		}
		keyPairPosition := 2 * keyPairNumber
		key := h.impliedArgs[keyPairPosition]
		value := h.impliedArgs[keyPairPosition+1]
		fmt.Fprintf(&argsStringBuilder, "%+v=%+v", key, value)
	}

	return argsStringBuilder.String()
}
