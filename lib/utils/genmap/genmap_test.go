/*
Copyright 2024 Gravitational, Inc.

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
package genmap

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

// TestCaching verifies the basic expected behavior of foreground operations.
func TestCaching(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// set up a genmap with a long regen interval
	var counter atomic.Uint64
	gm, err := New(Config[string, int]{
		RegenInterval: time.Hour,
		Generator: func(_ context.Context, key string) (int, error) {
			return int(counter.Add(1)), nil
		},
	})
	require.NoError(t, err)
	defer gm.Close()

	// verify that many concurrent calls result in only a single call to
	// the underlying generator.
	var eg errgroup.Group
	for i := 0; i < 100; i++ {
		eg.Go(func() error {
			n, err := gm.Get(ctx, "some-key")
			if err != nil {
				return err
			}

			if n != 1 {
				return fmt.Errorf("expected 1, got %d", n)
			}

			return nil
		})
	}

	require.NoError(t, eg.Wait())

	// force an early regen
	gm.Generate("some-key")

	// verify that regen occurs
	require.Eventually(t, func() bool {
		n, _ := gm.Get(ctx, "some-key")
		return n == 2
	}, time.Second*30, time.Millisecond*100)
}

// TestConcurrentTermination verifies that concurrently terminating background ops does not
// interfere with the ability of each individual Get to yield a sensible/expected value.
func TestConcurrentTermination(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// set up a genmap with a short regen interval
	var counter atomic.Uint64
	gm, err := New(Config[string, int]{
		RegenInterval: time.Hour,
		Generator: func(_ context.Context, key string) (int, error) {
			return int(counter.Add(1)), nil
		},
	})
	require.NoError(t, err)
	defer gm.Close()

	var eg errgroup.Group
	for i := 0; i < 200; i++ {
		if i%2 == 0 {
			eg.Go(func() error {
				for j := 0; j < 100; j++ {
					n, err := gm.Get(ctx, "some-key")
					if err != nil {
						return err
					}
					if n == 0 {
						return fmt.Errorf("expected nonzero n")
					}
				}
				return nil
			})
		} else {
			eg.Go(func() error {
				gm.Terminate("some-key")
				return nil
			})
		}
	}

	require.NoError(t, eg.Wait())

	// sanity check to assert that concurrent termination really happened (in practice we expect
	// this value to always be *much* larger than 2).
	n, _ := gm.Get(ctx, "some-key")
	require.Greater(t, n, 2)
}

// TestBackground tests basic expected behaviors of background regen.
func TestBackground(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// set up a genmap with a short regen interval
	var counter atomic.Uint64
	gench := make(chan struct{})
	gm, err := New(Config[string, int]{
		RegenInterval: time.Millisecond,
		Generator: func(_ context.Context, key string) (int, error) {
			select {
			case gench <- struct{}{}:
			default:
			}
			return int(counter.Add(1)), nil
		},
	})
	require.NoError(t, err)
	defer gm.Close()

	// trigger generation for a key
	gm.Generate("some-key")

	// verify that background regeneration occurs multiple times
	timeout := time.After(time.Second * 30)
	for i := 0; i < 3; i++ {
		select {
		case <-gench:
		case <-timeout:
			require.FailNow(t, "timeout waiting for regen")
		}
	}

	n, _ := gm.Get(ctx, "some-key")
	require.Greater(t, n, 2)

	// kill the background refreshes of our target key
	gm.Terminate("some-key")

	// termiante blocks until the background generation routine exits, so once
	// terminate returns we should not observe any additional gen calls.
	select {
	case <-gench:
		require.FailNow(t, "unexpected call to generator after termination greater than 1 regen interval ago")
	case <-time.After(time.Millisecond * 200):
	}
}
