/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
package genmap

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

// TestCaching verifies the basic expected behavior of foreground operations.
func TestCaching(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

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
	for range 100 {
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

	ctx := t.Context()

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

	var increments atomic.Uint64
	var eg errgroup.Group
	eg.Go(func() error {
		for increments.Load() < 5 {
			gm.Terminate("some-key")
		}

		return nil
	})

	eg.Go(func() error {
		var lastSeen int
		for increments.Load() < 5 {
			n, err := gm.Get(ctx, "some-key")
			if err != nil {
				return trace.Wrap(err)
			}

			switch {
			case n > lastSeen:
				lastSeen = n
				increments.Add(1)
			case n < lastSeen:
				return trace.Errorf("unexpected decrement %d -> %d", lastSeen, n)
			case n == 0:
				return trace.Errorf("n should not be zero")
			}
		}

		return nil
	})

	require.NoError(t, eg.Wait())
}

// TestBackground tests basic expected behaviors of background regen.
func TestBackground(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

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
	for range 4 {
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
