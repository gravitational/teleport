// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package utils

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/user"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// TestBackgroundTask is a task that should be run in the background for the remaining duration of a test and
// reliably exit before the test completes.
type TestBackgroundTask struct {
	// Name is an identifier for the task that will be included in logs and error messages.
	Name string

	// Task is the function that will be called in a background goroutine to run the task.
	//
	// It must not terminate before the context is canceled, and it must reliably terminate after the context
	// is canceled and Terminate is called.
	//
	// Any error other than [context.Canceled] will fail the test.
	Task func(ctx context.Context) error

	// Terminate is an optional function that will be called to terminate the task during test cleanup.
	// It does not need to be set if the task will reliably terminate after context cancellation.
	Terminate func() error
}

// RunTestBackgroundTask runs task.Task in the background for the remaining duration of the test.
// During test cleanup it will cancel the context passed to the task, call task.Terminate if set, and wait for
// the task to terminate before allowing the test to complete.
func RunTestBackgroundTask(ctx context.Context, t *testing.T, task *TestBackgroundTask) {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	go func() {
		defer close(done)
		err := task.Task(ctx)
		if ctx.Err() == nil {
			// The context hasn't been canceled yet, meaning the task has exited prematurely. This should
			// fail the test even if the error is nil.
			t.Errorf("Test background task %q exited prematurely with error: %s", task.Name, trace.DebugReport(err))
			return
		}
		// The context has been canceled and the task has successfully exited, but any error other than
		// context.Canceled should still fail the test.
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("Test background task %q exited with error: %+v", task.Name, trace.DebugReport(err))
		}
	}()

	t.Cleanup(func() {
		t.Logf("Cleanup: terminating test background task %q.", task.Name)
		cancel()
		if task.Terminate != nil {
			if err := task.Terminate(); err != nil {
				t.Errorf("Terminating test background task %q failed with error: %s", task.Name, trace.DebugReport(err))
			}
		}
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				t.Logf("Waiting for test background task %q to terminate.", task.Name)
			case <-done:
				return
			}
		}
	})
}

// RequireRoot skips the current test if it is not running as root.
func RequireRoot(tb testing.TB) {
	tb.Helper()
	if os.Geteuid() != 0 {
		tb.Skip("This test will be skipped because tests are not being run as root.")
	}
}

func generateUsername(tb testing.TB) string {
	suffix := make([]byte, 8)
	_, err := rand.Read(suffix)
	require.NoError(tb, err)
	return fmt.Sprintf("teleport-%x", suffix)
}

// GenerateLocalUsername generates the username for a local user that does not
// already exists (but it does not create the user).
func GenerateLocalUsername(tb testing.TB) string {
	const maxAttempts = 10
	for i := 0; i < maxAttempts; i++ {
		login := generateUsername(tb)
		_, err := user.Lookup(login)
		if errors.Is(err, user.UnknownUserError(login)) {
			return login
		}
		require.NoError(tb, err)
	}
	tb.Fatalf("Unable to generate unused username after %v attempts", maxAttempts)
	return ""
}
