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

package services

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// TestFanoutWatcherClose tests fanout watcher close
// removes it from the buffer
func TestFanoutWatcherClose(t *testing.T) {
	eventsCh := make(chan FanoutEvent, 1)
	f := NewFanout(eventsCh)
	w, err := f.NewWatcher(context.TODO(),
		types.Watch{Name: "test", Kinds: []types.WatchKind{{Name: "test"}}})
	require.NoError(t, err)
	require.Equal(t, f.Len(), 1)

	err = w.Close()
	select {
	case <-eventsCh:
	case <-time.After(time.Second):
		t.Fatalf("Timeout waiting for event")
	}
	require.NoError(t, err)
	require.Equal(t, f.Len(), 0)
}

// TestFanoutInit verifies that Init event is sent exactly once.
func TestFanoutInit(t *testing.T) {
	f := NewFanout()

	w, err := f.NewWatcher(context.TODO(), types.Watch{
		Name:  "test",
		Kinds: []types.WatchKind{{Name: "spam"}, {Name: "eggs"}},
	})
	require.NoError(t, err)

	f.SetInit()

	select {
	case e := <-w.Events():
		require.Equal(t, types.OpInit, e.Type)
	default:
		t.Fatalf("Expected init event")
	}

	select {
	case e := <-w.Events():
		t.Fatalf("Unexpected second event: %+v", e)
	default:
	}
}

/*
goos: linux
goarch: amd64
pkg: github.com/gravitational/teleport/lib/services
cpu: Intel(R) Core(TM) i9-10885H CPU @ 2.40GHz
BenchmarkFanoutRegistration-16       	       1	118856478045 ns/op
*/
// NOTE: this benchmark exists primarily to "contrast" with the set registration
// benchmark below, and demonstrate why the set-based strategy is necessary.
func BenchmarkFanoutRegistration(b *testing.B) {
	const iterations = 100_000
	ctx := context.Background()

	for n := 0; n < b.N; n++ {
		f := NewFanout()
		f.SetInit()

		var wg sync.WaitGroup

		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				w, err := f.NewWatcher(ctx, types.Watch{
					Name:  "test",
					Kinds: []types.WatchKind{{Name: "spam"}, {Name: "eggs"}},
				})
				require.NoError(b, err)
				w.Close()
			}()
		}

		wg.Wait()
	}
}

/*
goos: linux
goarch: amd64
pkg: github.com/gravitational/teleport/lib/services
cpu: Intel(R) Core(TM) i9-10885H CPU @ 2.40GHz
BenchmarkFanoutSetRegistration-16    	       3	 394211563 ns/op
*/
func BenchmarkFanoutSetRegistration(b *testing.B) {
	const iterations = 100_000
	ctx := context.Background()

	for n := 0; n < b.N; n++ {
		f := NewFanoutSet()
		f.SetInit()

		var wg sync.WaitGroup

		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				w, err := f.NewWatcher(ctx, types.Watch{
					Name:  "test",
					Kinds: []types.WatchKind{{Name: "spam"}, {Name: "eggs"}},
				})
				require.NoError(b, err)
				w.Close()
			}()
		}

		wg.Wait()
	}
}
