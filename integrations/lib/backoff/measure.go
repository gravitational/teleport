/*
Copyright 2021 Gravitational, Inc.

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

package backoff

import (
	"context"
	"runtime"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

func measure(ctx context.Context, clock clockwork.FakeClock, fn func() error) (time.Duration, error) {
	done := make(chan struct{})
	var dur time.Duration
	var err error
	go func() {
		before := clock.Now()
		err = fn()
		after := clock.Now()
		dur = after.Sub(before)
		close(done)
	}()
	clock.BlockUntil(1)
	for {
		/*
			What does runtime.Gosched() do?
			> Gosched yields the processor, allowing other goroutines to run. It does not
			> suspend the current goroutine, so execution resumes automatically.

			Why do we need it?
			There are two concurrent goroutines at this point:
			- this one
			- the one that executes `fn()`
			When this one is scheduled to run it advances the clock a bit more.
			It might happen that this one keeps running over and over, while the other one is not scheduled.
			When that happens, the other 'select' (the one in decorr.Do) gets called and returns nil,
			the goroutine sets the `dur` value.
			However, it's too late because the observed time (`dur`) is already larger than expected.

			If both goroutines ran sequentially, this would work.
			Calling runtime.Gosched here, tries to give priority to the other goroutine.
			So, when the other goroutine's select is ready (the clock.After returns), it immediately returns and
			`dur` has the expected value.
		*/
		runtime.Gosched()
		select {
		case <-done:
			return dur, trace.Wrap(err)
		case <-ctx.Done():
			return time.Duration(0), trace.Wrap(ctx.Err())
		default:
			clock.Advance(5 * time.Millisecond)
			runtime.Gosched()
		}
	}
}
