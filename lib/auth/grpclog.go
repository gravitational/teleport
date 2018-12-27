/*
Copyright 2018 Gravitational, Inc.

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

package auth

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

// GLogger implements GRPC logger interface LoggerV2
type GLogger struct {
	Entry *logrus.Entry
	// Verbosity is verbosity as it's understood by GRPC
	Verbosity int
}

// Info logs to INFO log. Arguments are handled in the manner of fmt.Print.
func (g *GLogger) Info(args ...interface{}) {
	// GRPC is very verbose, so this is intentionally
	// pushes info level statements as Teleport's debug level ones
	g.Entry.Debug(args...)
}

// Infoln logs to INFO log. Arguments are handled in the manner of fmt.Println.
func (g *GLogger) Infoln(args ...interface{}) {
	// GRPC is very verbose, so this is intentionally
	// pushes info level statements as Teleport's debug level ones
	g.Entry.Debug(fmt.Sprintln(args...))
}

// Infof logs to INFO log. Arguments are handled in the manner of fmt.Printf.
func (g *GLogger) Infof(format string, args ...interface{}) {
	// GRPC is very verbose, so this is intentionally
	// pushes info level statements as Teleport's debug level ones
	g.Entry.Debugf(format, args...)
}

// Warning logs to WARNING log. Arguments are handled in the manner of fmt.Print.
func (g *GLogger) Warning(args ...interface{}) {
	g.Entry.Warning(args...)
}

// Warningln logs to WARNING log. Arguments are handled in the manner of fmt.Println.
func (g *GLogger) Warningln(args ...interface{}) {
	g.Entry.Warning(fmt.Sprintln(args...))
}

// Warningf logs to WARNING log. Arguments are handled in the manner of fmt.Printf.
func (g *GLogger) Warningf(format string, args ...interface{}) {
	g.Entry.Warningf(format, args...)
}

// Error logs to ERROR log. Arguments are handled in the manner of fmt.Print.
func (g *GLogger) Error(args ...interface{}) {
	g.Entry.Error(args...)
}

// Errorln logs to ERROR log. Arguments are handled in the manner of fmt.Println.
func (g *GLogger) Errorln(args ...interface{}) {
	g.Entry.Error(fmt.Sprintln(args...))
}

// Errorf logs to ERROR log. Arguments are handled in the manner of fmt.Printf.
func (g *GLogger) Errorf(format string, args ...interface{}) {
	g.Entry.Errorf(format, args...)
}

// Fatal logs to ERROR log. Arguments are handled in the manner of fmt.Print.
// gRPC ensures that all Fatal logs will exit with os.Exit(1).
// Implementations may also call os.Exit() with a non-zero exit code.
func (g *GLogger) Fatal(args ...interface{}) {
	// Teleport can be used as a library, prevent GRPC
	// from crashing the process
	g.Entry.Error(args...)
}

// Fatalln logs to ERROR log. Arguments are handled in the manner of fmt.Println.
// gRPC ensures that all Fatal logs will exit with os.Exit(1).
// Implementations may also call os.Exit() with a non-zero exit code.
func (g *GLogger) Fatalln(args ...interface{}) {
	// Teleport can be used as a library, prevent GRPC
	// from crashing the process
	g.Entry.Error(fmt.Sprintln(args...))
}

// Fatalf logs to ERROR log. Arguments are handled in the manner of fmt.Printf.
// gRPC ensures that all Fatal logs will exit with os.Exit(1).
// Implementations may also call os.Exit() with a non-zero exit code.
func (g *GLogger) Fatalf(format string, args ...interface{}) {
	// Teleport can be used as a library, prevent GRPC
	// from crashing the process
	g.Entry.Errorf(format, args...)
}

// V reports whether verbosity level l is at least the requested verbose level.
func (g *GLogger) V(l int) bool {
	return l <= g.Verbosity
}
