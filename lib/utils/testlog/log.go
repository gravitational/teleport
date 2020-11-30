/*
Copyright 2020 Gravitational, Inc.

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

// Package testlog provides custom loggers for use in tests.
package testlog

import (
	"fmt"
	"os"

	"github.com/gravitational/teleport/lib/utils"

	"github.com/sirupsen/logrus"
	"gopkg.in/check.v1"
)

// FailureOnly returns a logger that only prints the logs to STDERR when the
// test fails.
func FailureOnly(t testingInterface) logrus.FieldLogger {
	// Collect all output into buf.
	buf := utils.NewSyncBuffer()
	log := utils.NewLoggerForTests()
	log.SetOutput(buf)

	// Register a cleanup callback which prints buf iff t has failed.
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		fmt.Fprintln(os.Stderr, buf.String())
	})

	return log.WithField("test", t.Name())
}

// NewCheckTestWrapper creates a new logging wrapper for the specified
// instance of the gocheck.C.
// Returned value has an exported Log attribute that represents
// the logger for the underlying test.
// It is caller's responsibility to release the wrapper by invoking
// Close after the test has completed.
func NewCheckTestWrapper(c *check.C) *TestWrapper {
	w := &TestWrapper{
		c: c,
	}
	w.Log = FailureOnly(w)
	return w
}

// Cleanup registers the specified handler f to be run
// after the hest gas completed
func (r *TestWrapper) Cleanup(f func()) {
	r.cleanups = append(r.cleanups, f)
}

// Failed returns true if the underlying test has failed
func (r *TestWrapper) Failed() bool {
	return r.c.Failed()
}

// Name returns the name of the underlying test
func (r *TestWrapper) Name() string {
	return r.c.TestName()
}

// Close invokes all registered cleanup handlers
func (r *TestWrapper) Close() {
	for _, f := range r.cleanups {
		f()
	}
}

// TestWrapper wraps an existing instance of gocheck.C
// for a specific test.
// Implements testingInterface
type TestWrapper struct {
	// Log specifies the logger that can be used to emit
	// test-specific messages
	Log logrus.FieldLogger

	c        *check.C
	cleanups []func()
}

type testingInterface interface {
	// Cleanup registers the specified handler f to be run
	// after the hest gas completed
	Cleanup(func())
	// Failed returns true of the underlying test has failed
	Failed() bool
	// Name returns the name of the underlying test
	Name() string
}
