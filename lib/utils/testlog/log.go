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
	"testing"

	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

// FailureOnly returns a logger that only prints the logs to STDERR when the
// test fails.
func FailureOnly(t TestingInterface) utils.Logger {
	// Collect all output into buf.
	buf := utils.NewSyncBuffer()
	logger := utils.NewLoggerForTests()
	logger.SetOutput(buf)

	// Register a cleanup callback which prints buf iff t has failed.
	t.Cleanup(func() {
		if t.Failed() || testing.Verbose() {
			fmt.Fprintln(os.Stderr, buf.String())
		}
	})

	return utils.WrapLogger(logger.WithField("test", t.Name()))
}

// NewCheckTestWrapper creates a new logging wrapper for the specified
// *gocheck.C value.
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
// after the test has completed
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

// TestWrapper wraps an existing *gocheck.C value for a specific test.
// Implements TestingInterface
type TestWrapper struct {
	// Log specifies the logger that can be used to emit
	// test-specific messages
	Log utils.Logger

	c        *check.C
	cleanups []func()
}

// TestingInterface abstracts a testing implementation.
type TestingInterface interface {
	// Cleanup registers the specified handler f to be run
	// after the test has completed
	Cleanup(func())
	// Failed returns true of the underlying test has failed
	Failed() bool
	// Name returns the name of the underlying test
	Name() string
}
