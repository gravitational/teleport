// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package watcher

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func newGate(t *testing.T, window time.Duration) *WindowedGate {
	t.Helper()

	g, err := NewWindowedGate(WindowedGateConfig{
		Window: window,
	})
	require.NoError(t, err)
	return g
}

func TestMaybeDo_WindowRespected(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		g := newGate(t, time.Second)

		var calls atomic.Int32

		fn := func(context.Context) error {
			calls.Add(1)
			return nil
		}

		const numberOfCalls = 16
		for range numberOfCalls {
			go func() {
				g.MaybeDo(ctx, fn)
			}()
		}

		synctest.Wait()

		require.Equal(t, int32(1), calls.Load(), " expected single execution within the window")
	})
}

func TestMaybeDo_WindowExpires(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		g := newGate(t, time.Second)

		var calls atomic.Int32

		fn := func(context.Context) error {
			calls.Add(1)
			return nil
		}

		const numberOfCalls = 16
		for i := range numberOfCalls {
			go func() {
				g.MaybeDo(ctx, fn)
			}()

			if (i+1)%4 == 0 { // Every 4th call advance window
				time.Sleep(time.Second + time.Millisecond)
			}
		}

		require.Equal(t, int32(4), calls.Load(), "Expected 4 calls total across 4 windows")
	})
}
