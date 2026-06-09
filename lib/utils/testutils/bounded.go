/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package testutils

import (
	"runtime/debug"
	"time"
)

// FailReporter is the minimal subset of testing.TB and rapid.T needed by
// RunWithTimeout.
type FailReporter interface {
	Fatalf(format string, args ...any)
	Helper()
}

// RunWithTimeout calls fn in a goroutine and reports a failure via t if fn panics or runs past timeout.
// The goroutine is leaked on timeout (it can't be canceled from the outside), which is acceptable in tests.
func RunWithTimeout(t FailReporter, timeout time.Duration, fn func()) {
	t.Helper()

	done := make(chan struct{})
	var panicVal any
	var stack []byte
	go func() {
		defer func() {
			if r := recover(); r != nil {
				panicVal = r
				stack = debug.Stack()
			}
			close(done)
		}()
		fn()
	}()

	select {
	case <-done:
		if panicVal != nil {
			t.Fatalf("panic: %v\n%s", panicVal, stack)
		}
	case <-time.After(timeout):
		t.Fatalf("fn did not return within %s", timeout)
	}
}
