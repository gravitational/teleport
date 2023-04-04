/*
Copyright 2023 Gravitational, Inc.

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

package utils

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestRunTasksPerSecond(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	clock := clockwork.NewFakeClock()
	var tasksExecuted atomic.Uint32

	var wg sync.WaitGroup
	fn := func(_ context.Context) error {
		wg.Done()
		tasksExecuted.Add(1)
		return nil
	}

	// 2 functions should be queued and run.
	var runWg sync.WaitGroup

	runWg.Add(1)
	wg.Add(2)
	var runErr error
	go func() {
		runErr = RunTasksPerSecond(ctx, clock, 2, []TaskFn{fn, fn, fn})
		runWg.Done()
	}()

	// Both functions should complete
	wg.Wait()
	require.Equal(t, uint32(2), tasksExecuted.Load())

	// Wait for the third function to run.
	wg.Add(1)
	clock.Advance(2 * time.Second)

	// The third function should complete.
	wg.Wait()
	require.Equal(t, uint32(3), tasksExecuted.Load())

	runWg.Wait()
	require.NoError(t, runErr)

	fnErr := func(_ context.Context) error {
		return trace.BadParameter("fn err")
	}

	// Only 1 function should run and an error should be reported.
	tasksExecuted.Store(0)
	runWg.Add(1)
	wg.Add(1)
	go func() {
		runErr = RunTasksPerSecond(ctx, clock, 2, []TaskFn{fn, fnErr, fn})
		runWg.Done()
	}()

	wg.Wait()
	require.Equal(t, uint32(1), tasksExecuted.Load())

	runWg.Wait()
	require.ErrorIs(t, runErr, fnErr(ctx))

	// 2 functions should run, the third should be skipped as the context is canceled.
	tasksExecuted.Store(0)
	runWg.Add(1)
	wg.Add(2)
	go func() {
		runErr = RunTasksPerSecond(ctx, clock, 2, []TaskFn{fn, fn, fn})
		runWg.Done()
	}()

	wg.Wait()
	require.Equal(t, uint32(2), tasksExecuted.Load())

	cancel()
	runWg.Wait()

	require.ErrorIs(t, runErr, context.Canceled)
	require.Equal(t, uint32(2), tasksExecuted.Load())
}
