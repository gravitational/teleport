// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package oslog

// #cgo CFLAGS: -Wall -xobjective-c -fblocks -fobjc-arc -mmacosx-version-min=11.0
// #cgo LDFLAGS: -framework Foundation
// #include <stdlib.h>
// #include <os/log.h>
// #include "oslog_darwin.h"
import "C"

import (
	"unsafe"
)

// Logger encapsulates os_log_t object.
type Logger struct {
	osLog unsafe.Pointer
}

// NewLogger creates a new logger that writes to os_log. The caller is expected to call the Close
// method when Logger is no longer needed.
//
// Calling this function twice with the same arguments returns a pointer to a different Go struct,
// but the underlying osLog pointer is the same. This is handled by the logging runtime. The
// underlying os_log_t object is never deallocated. See os_log_create(3) for more details.
//
// https://developer.apple.com/documentation/os/1643744-os_log_create/
func NewLogger(subsystem string, category string) *Logger {
	cSubsystem := C.CString(subsystem)
	defer C.free(unsafe.Pointer(cSubsystem))
	cCategory := C.CString(category)
	defer C.free(unsafe.Pointer(cCategory))

	osLog := C.TELCreateLog(cSubsystem, cCategory)
	logger := Logger{osLog: osLog}

	return &logger
}

// Log logs the given message as the given logType.
//
// All messages are logged as public messages [1] since the Teleport codebase doesn't have the
// notion of public and private log messages.
//
// Log messages have a 1024-byte encoded size limit. Messages over this size will be truncated.
// The limit can be increased to 32 kilobytes on a per-subsystem or per-category level through
// Enable-Oversize-Messages, see os_log(5). Apple does not recommended enabling this for
// performance-sensitive code paths. Since at Teleport any component can log long stack traces at
// any code path, it's generally recommend to leave it turned off.
//
// [1]: https://developer.apple.com/documentation/os/generating-log-messages-from-your-code?language=objc#Redact-Sensitive-User-Data-from-a-Log-Message
func (l *Logger) Log(logType OsLogType, message string) {
	cMessage := C.CString(message)
	defer C.free(unsafe.Pointer(cMessage))

	C.TELLog(l.osLog, C.uint(logType), cMessage)
}

// OsLogType describes available log types that can be passed to Logger.Log.
// By default OsLogTypeDebug and OsLogTypeInfo are stored only in memory and other log types are
// persisted to disk.
type OsLogType int

const (
	// OsLogTypeDebug is the equivalent of slog.LevelDebug. Messages of this type are stored only in
	// memory unless the subsystem or the category is configured to store them on disk.
	// See os_log(5) and https://developer.apple.com/documentation/os/customizing-logging-behavior-while-debugging?language=objc
	OsLogTypeDebug OsLogType = C.OS_LOG_TYPE_DEBUG
	// OsLogTypeInfo is the equivalent of slog.LevelInfo. Messages of this type are stored only in
	// memory unless the subsystem or the category is configured to store them on disk.
	// See man 5 os_log and https://developer.apple.com/documentation/os/customizing-logging-behavior-while-debugging?language=objc
	OsLogTypeInfo OsLogType = C.OS_LOG_TYPE_INFO
	// OsLogTypeDefault is functionally the equivalent of slog.LevelWarn. Messages of this type are
	// always persisted in the data store.
	OsLogTypeDefault OsLogType = C.OS_LOG_TYPE_DEFAULT
	// OsLogTypeError is the equivalent of slog.LevelError. Messages of this type are always persisted
	// in the data store.
	OsLogTypeError OsLogType = C.OS_LOG_TYPE_ERROR
)
