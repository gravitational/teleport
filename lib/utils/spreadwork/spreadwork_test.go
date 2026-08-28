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

package spreadwork

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyOverTime(t *testing.T) {
	t.Run("all items at once", func(t *testing.T) {
		ctx := context.Background()
		fakeClock := clockwork.NewFakeClock()
		items := []string{"1", "2", "3", "4", "5", "6"}
		conf := ApplyOverTimeConfig{
			MaxDuration: time.Second,
			clock:       fakeClock,
		}

		var ops atomic.Uint64

		go func() {
			err := ApplyOverTime(ctx, conf, items, func(s string) {
				ops.Add(1)
			})
			assert.NoError(t, err)
		}()

		require.Eventually(t, func() bool {
			return ops.Load() == uint64(len(items))
		}, 1*time.Second, 10*time.Millisecond)
	})

	t.Run("items processed in one chunks with some time left", func(t *testing.T) {
		ctx := context.Background()
		fakeClock := clockwork.NewFakeClock()
		items := []string{"1", "2", "3", "4", "5", "6"}
		conf := ApplyOverTimeConfig{
			MaxDuration:   3 * time.Second,
			BatchInterval: 2 * time.Second,
			clock:         fakeClock,
		}

		var ops atomic.Uint64

		go func() {
			err := ApplyOverTime(ctx, conf, items, func(s string) {
				ops.Add(1)
			})
			assert.NoError(t, err)
		}()

		require.Eventually(t, func() bool {
			fakeClock.Advance(100 * time.Millisecond)
			return ops.Load() == uint64(len(items))
		}, 1*time.Second, time.Millisecond)
	})

	t.Run("items processed in two chunks", func(t *testing.T) {
		ctx := context.Background()
		fakeClock := clockwork.NewFakeClock()
		items := []string{"1", "2", "3", "4", "5", "6"}
		conf := ApplyOverTimeConfig{
			MaxDuration:   2 * time.Second,
			BatchInterval: time.Second,
			MinBatchSize:  1,
			clock:         fakeClock,
		}

		var ops atomic.Uint64

		go func() {
			err := ApplyOverTime(ctx, conf, items, func(s string) {
				ops.Add(1)
			})
			assert.NoError(t, err)
		}()

		require.Eventually(t, func() bool {
			fakeClock.Advance(100 * time.Millisecond)
			return ops.Load() == uint64(len(items))
		}, 1*time.Second, time.Millisecond)
	})

	t.Run("items processed in 3 chunks with uneven items", func(t *testing.T) {
		ctx := context.Background()
		fakeClock := clockwork.NewFakeClock()
		items := []string{"1", "2", "3", "4", "5", "6", "7", "8"}
		conf := ApplyOverTimeConfig{
			MaxDuration:   3 * time.Second,
			BatchInterval: time.Second,
			MinBatchSize:  1,
			clock:         fakeClock,
		}

		var ops atomic.Uint64

		go func() {
			err := ApplyOverTime(ctx, conf, items, func(s string) {
				ops.Add(1)
			})
			assert.NoError(t, err)
		}()

		require.Eventually(t, func() bool {
			fakeClock.Advance(100 * time.Millisecond)
			return ops.Load() == uint64(len(items))
		}, 1*time.Second, time.Millisecond)
	})

	t.Run("cancel processing after three items", func(t *testing.T) {
		ctx, cancelFn := context.WithCancel(context.Background())
		fakeClock := clockwork.NewFakeClock()
		items := []string{"1", "2", "3", "4", "5", "6", "7", "8"}
		conf := ApplyOverTimeConfig{
			MaxDuration:   3 * time.Second,
			BatchInterval: time.Second,
			MinBatchSize:  1,
			clock:         fakeClock,
		}

		var ops atomic.Uint64
		go func() {
			err := ApplyOverTime(ctx, conf, items, func(s string) {
				ops.Add(1)
			})

			assert.ErrorIs(t, err, context.Canceled)
		}()

		require.Eventually(t, func() bool {
			fakeClock.Advance(100 * time.Millisecond)
			if ops.Load() == 3 {
				cancelFn()
				return true
			}

			return false
		}, 1*time.Second, time.Millisecond)
		require.Equal(t, uint64(3), ops.Load())
	})

	t.Run("check and set defaults", func(t *testing.T) {
		for _, tt := range []struct {
			name     string
			input    ApplyOverTimeConfig
			errCheck require.ErrorAssertionFunc
		}{
			{
				name: "minimal config",
				input: ApplyOverTimeConfig{
					MaxDuration: 3 * time.Second,
				},
				errCheck: require.NoError,
			},
			{
				name: "all values set",
				input: ApplyOverTimeConfig{
					MaxDuration:   3 * time.Second,
					BatchInterval: 2 * time.Second,
				},
				errCheck: require.NoError,
			},
			{
				name: "invalid max duration",
				input: ApplyOverTimeConfig{
					MaxDuration: 0,
				},
				errCheck: require.Error,
			},
			{
				name: "batch interval is greater than max duration",
				input: ApplyOverTimeConfig{
					MaxDuration:   time.Second,
					BatchInterval: time.Minute,
				},
				errCheck: require.Error,
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				tt.errCheck(t, tt.input.CheckAndSetDefaults())
			})
		}
	})
}
