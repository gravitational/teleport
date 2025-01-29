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

package vnet

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
)

func newProcessManager() (*ProcessManager, context.Context) {
	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)
	pm := &ProcessManager{
		g:      g,
		cancel: cancel,
		closed: make(chan struct{}),
	}
	pm.closeOnce = sync.OnceFunc(func() {
		close(pm.closed)
	})
	return pm, ctx
}

// ProcessManager handles background tasks needed to run VNet.
// Its semantics are similar to an error group with a context, but it cancels the context whenever
// any task returns prematurely, that is, a task exits while the context was not canceled.
type ProcessManager struct {
	g         *errgroup.Group
	cancel    context.CancelFunc
	closed    chan struct{}
	closeOnce func()
	networkStackInfo NetworkStackInfo
}

// AddCriticalBackgroundTask adds a function to the error group. [task] is expected to block until
// the context returned by [newProcessManager] gets canceled. The context gets canceled either by
// calling Close on [ProcessManager] or if any task returns.
func (pm *ProcessManager) AddCriticalBackgroundTask(name string, task func() error) {
	pm.g.Go(func() error {
		err := task()
		if err == nil {
			// Make sure to always return an error so that the errgroup context is canceled.
			err = fmt.Errorf("critical task %q exited prematurely", name)
		}
		return trace.Wrap(err)
	})
}

// Wait blocks and waits for the background tasks to finish, which typically happens when another
// goroutine calls Close on the process manager.
func (pm *ProcessManager) Wait() error {
	err := pm.g.Wait()
	select {
	case <-pm.closed:
		// Errors are expected after the process manager has been closed,
		// usually due to context cancellation, but other error types may be
		// returned. Log unexpected errors at debug level but return nil.
		if err != nil && !errors.Is(err, context.Canceled) {
			log.DebugContext(context.Background(), "ProcessManager exited with error after being closed", "error", err)
		}
		return nil
	default:
		return trace.Wrap(err)
	}
}

// Close stops any active background tasks by canceling the underlying context,
// and waits for all tasks to terminate.
func (pm *ProcessManager) Close() {
	pm.closeOnce()
	pm.cancel()
}
