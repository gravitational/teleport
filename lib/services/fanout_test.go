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
	ctx := context.Background()
	eventsCh := make(chan FanoutEvent, 1)
	f := NewFanout(eventsCh)
	w, err := f.NewWatcher(ctx,
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
		Kinds: []types.WatchKind{{Kind: "spam"}, {Kind: "eggs"}},
	})
	require.NoError(t, err)

	f.SetInit([]types.WatchKind{{Kind: "spam"}, {Kind: "eggs"}})

	select {
	case e := <-w.Events():
		require.Equal(t, types.OpInit, e.Type)
		require.Equal(t, types.NewWatchStatus([]types.WatchKind{{Kind: "spam"}, {Kind: "eggs"}}), e.Resource)
	default:
		t.Fatalf("Expected init event")
	}

	select {
	case e := <-w.Events():
		t.Fatalf("Unexpected second event: %+v", e)
	default:
	}
}

// TestUnsupportedKindInitialized verifies that an initialized Fanout fails immediately when requested a watched
// for a resource kind that wasn't confirmed by the event source in regular mode, but works in partial success mode.
func TestUnsupportedKindInitialized(t *testing.T) {
	ctx := context.Background()

	f := NewFanout()
	f.SetInit([]types.WatchKind{{Kind: "spam"}})

	// fails immediately in regular mode
	testWatch := types.Watch{
		Name:  "test",
		Kinds: []types.WatchKind{{Kind: "spam"}, {Kind: "eggs"}},
	}
	_, err := f.NewWatcher(ctx, testWatch)
	require.Error(t, err)

	// works in partial success mode
	testWatch.AllowPartialSuccess = true
	w, err := f.NewWatcher(ctx, testWatch)
	require.NoError(t, err)

	select {
	case e := <-w.Events():
		require.Equal(t, types.OpInit, e.Type)
		watchStatus, ok := e.Resource.(types.WatchStatus)
		require.True(t, ok)
		require.Equal(t, []types.WatchKind{{Kind: "spam"}}, watchStatus.GetKinds())
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for event.")
	}
}

// TestUnsupportedKindDelayed verifies that, upon initialization, Fanout closes pending watchers that requested
// resource kinds that weren't confirmed by the event source and didn't enable partial success mode.
func TestUnsupportedKindDelayed(t *testing.T) {
	ctx := context.Background()
	f := NewFanout()

	regularWatcher, err := f.NewWatcher(ctx, types.Watch{
		Name:  "test",
		Kinds: []types.WatchKind{{Kind: "spam"}, {Kind: "eggs"}},
	})
	require.NoError(t, err)

	partialSuccessWatcher, err := f.NewWatcher(ctx, types.Watch{
		Name:                "test",
		Kinds:               []types.WatchKind{{Kind: "spam"}, {Kind: "eggs"}},
		AllowPartialSuccess: true,
	})
	require.NoError(t, err)

	f.SetInit([]types.WatchKind{{Kind: "spam"}})

	// regular watcher fails upon Fanout initialization
	select {
	case <-regularWatcher.Events():
		t.Fatal("unexpected event from watcher that's supposed to fail")
	case <-regularWatcher.Done():
		require.Error(t, regularWatcher.Error())
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for close event.")
	}

	// watcher in partial success mode receives OpInit with partial confirmation
	select {
	case e := <-partialSuccessWatcher.Events():
		require.Equal(t, types.OpInit, e.Type)
		watchStatus, ok := e.Resource.(types.WatchStatus)
		require.True(t, ok)
		require.Equal(t, []types.WatchKind{{Kind: "spam"}}, watchStatus.GetKinds())
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for event.")
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
		f.SetInit([]types.WatchKind{{Kind: "spam"}, {Kind: "eggs"}})

		var wg sync.WaitGroup

		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				w, err := f.NewWatcher(ctx, types.Watch{
					Name:  "test",
					Kinds: []types.WatchKind{{Kind: "spam"}, {Kind: "eggs"}},
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
		f.SetInit([]types.WatchKind{{Kind: "spam"}, {Kind: "eggs"}})

		var wg sync.WaitGroup

		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				w, err := f.NewWatcher(ctx, types.Watch{
					Name:  "test",
					Kinds: []types.WatchKind{{Kind: "spam"}, {Kind: "eggs"}},
				})
				require.NoError(b, err)
				w.Close()
			}()
		}

		wg.Wait()
	}
}
