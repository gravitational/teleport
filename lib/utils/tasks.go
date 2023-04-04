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
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// TaskFn is a function that can be supplied to RunTasksPerSecond in order to
// throttle arbitrary tasks.
type TaskFn func(context.Context) error

// RunTasksPerSecond will run tasksPerSecond tasks every second. The intent of this function is provide a
// generic way of throttling tasks within Teleport, particularly with respect to (but not limited to)
// backend operations. This will make it possible to control how many tasks are running
func RunTasksPerSecond(ctx context.Context, clock clockwork.Clock, tasksPerSecond int, tasks []TaskFn) error {
	if tasksPerSecond <= 0 {
		return trace.BadParameter("non-positive number of tasks per second: %d", tasksPerSecond)
	}

	// No tasks to execute.
	if len(tasks) == 0 {
		return nil
	}

	taskCh := make(chan TaskFn, tasksPerSecond)
	var once sync.Once
	closeCh := make(chan struct{})
	closer := func() {
		once.Do(func() { close(closeCh) })
	}

	var wg sync.WaitGroup
	var contextDoneErr error

	// This goroutine enqueues tasks into a task channel in blocks of
	// tasksPerSecond.
	wg.Add(1)
	go func() {
		// The task channel will be closed at the end of this function, which
		// will cause the task executor block below to cease running.
		defer close(taskCh)
		defer wg.Done()

		ticker := clock.NewTicker(1 * time.Second)
		defer ticker.Stop()

		i := 0
		numTasks := len(tasks)
		for {
			upperLimit := i + tasksPerSecond

			// Make sure we don't go off the end of the task array.
			if upperLimit > numTasks {
				upperLimit = numTasks
			}

			for ; i < upperLimit; i++ {
				taskCh <- tasks[i]
			}

			// We've reached the end of the tasks to run.
			if upperLimit == numTasks {
				return
			}

			// Wait for the ticker, the close channel, or context cancelation.
			select {
			case <-ticker.Chan():
			case <-closeCh:
				// If the close channel is closed, we won't consider this an error.
				return
			case <-ctx.Done():
				// If the context is canceled, we'll consider this an error.
				contextDoneErr = ctx.Err()
				return
			}
		}
	}()

	// Run the tasks in the order we receive them from the task channel.
	var errs []error
	var taskWg sync.WaitGroup
	for i := 0; i < tasksPerSecond; i++ {
		taskWg.Add(1)
		go func() {
			defer taskWg.Done()

			fn, ok := <-taskCh
			// If the task channel is closed, exit the loop.
			if !ok {
				return
			}

			// If we encounter an error, record it and exit the loop.
			err := fn(ctx)
			if err != nil {
				closer()
				errs = append(errs, err)
				return
			}
		}()
	}

	taskWg.Wait()

	// Close the close channel. If we encountered an error during the task
	// execution, this will close the task eunqueing function so that no more
	// tasks are queued up and the task channel is closed.
	closer()
	wg.Wait()

	return trace.NewAggregate(append([]error{contextDoneErr}, errs...)...)
}
