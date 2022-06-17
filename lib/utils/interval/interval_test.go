// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package interval

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

// TestIntervalReset verifies the basic behavior of the interval reset functionality.
// Since time based tests tend to be flaky, this test passes if it has a >50% success
// rate (i.e. >50% of resets seemed to have actually extended the timer successfully).
func TestIntervalReset(t *testing.T) {
	const iterations = 1_000
	const duration = time.Millisecond * 666

	success := atomic.NewUint64(0)
	failure := atomic.NewUint64(0)

	var wg sync.WaitGroup

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			resetTimer := time.NewTimer(duration / 3)
			defer resetTimer.Stop()

			interval := New(Config{
				Duration: duration,
			})
			defer interval.Stop()

			start := time.Now()

			for i := 0; i < 6; i++ {
				select {
				case <-interval.Next():
					failure.Inc()
					return
				case <-resetTimer.C:
					interval.Reset()
					resetTimer.Reset(duration / 3)
				}
			}

			<-interval.Next()
			elapsed := time.Since(start)
			// we expect this test to produce elapsed times of
			// 3*duration if it is working properly. we accept a
			// margin or error of +/- 1 duration in order to
			// minimize flakiness.
			if elapsed > duration*2 && elapsed < duration*4 {
				success.Inc()
			} else {
				failure.Inc()
			}
		}()
	}

	wg.Wait()

	t.Logf("success=%d, failure=%d", success.Load(), failure.Load())

	require.True(t, success.Load() > failure.Load())
}

func TestNewNoop(t *testing.T) {
	i := NewNoop()
	ch := i.Next()
	select {
	case <-ch:
		t.Fatalf("noop should not emit anything")
	default:
	}
	i.Stop()
	select {
	case <-ch:
		t.Fatalf("noop should not emit anything")
	default:
	}
}
