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

package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWaitForStablePID(t *testing.T) {
	t.Parallel()

	svc := &SystemdService{
		Log: slog.Default(),
	}

	for _, tt := range []struct {
		name       string
		ticks      []int
		baseline   int
		minStable  int
		maxCrashes int
		findErrs   map[int]error

		errored  bool
		canceled bool
	}{
		{
			name:       "immediate restart",
			ticks:      []int{2, 2},
			baseline:   1,
			minStable:  1,
			maxCrashes: 1,
		},
		{
			name: "zero stable",
		},
		{
			name:       "immediate crash",
			ticks:      []int{2, 3},
			baseline:   1,
			minStable:  1,
			maxCrashes: 0,
			errored:    true,
		},
		{
			name:       "no changes times out",
			ticks:      []int{1, 1, 1, 1},
			baseline:   1,
			minStable:  3,
			maxCrashes: 2,
			canceled:   true,
		},
		{
			name:       "baseline restart",
			ticks:      []int{2, 2, 2, 2},
			baseline:   1,
			minStable:  3,
			maxCrashes: 2,
		},
		{
			name:       "one restart then stable",
			ticks:      []int{1, 2, 2, 2, 2},
			baseline:   1,
			minStable:  3,
			maxCrashes: 2,
		},
		{
			name:       "two restarts then stable",
			ticks:      []int{1, 2, 3, 3, 3, 3},
			baseline:   1,
			minStable:  3,
			maxCrashes: 2,
		},
		{
			name:       "three restarts then stable",
			ticks:      []int{1, 2, 3, 4, 4, 4, 4},
			baseline:   1,
			minStable:  3,
			maxCrashes: 2,
		},
		{
			name:       "too many restarts excluding baseline",
			ticks:      []int{1, 2, 3, 4, 5},
			baseline:   1,
			minStable:  3,
			maxCrashes: 2,
			errored:    true,
		},
		{
			name:       "too many restarts including baseline",
			ticks:      []int{1, 2, 3, 4},
			baseline:   0,
			minStable:  3,
			maxCrashes: 2,
			errored:    true,
		},
		{
			name:       "too many restarts slow",
			ticks:      []int{1, 1, 1, 2, 2, 2, 3, 3, 3, 4},
			baseline:   0,
			minStable:  3,
			maxCrashes: 2,
			errored:    true,
		},
		{
			name:       "too many restarts after stable",
			ticks:      []int{1, 1, 1, 2, 2, 2, 3, 3, 3, 3, 4},
			baseline:   0,
			minStable:  3,
			maxCrashes: 2,
		},
		{
			name:       "stable after too many restarts",
			ticks:      []int{1, 1, 1, 2, 2, 2, 3, 3, 3, 4, 4, 4, 4},
			baseline:   0,
			minStable:  3,
			maxCrashes: 2,
			errored:    true,
		},
		{
			name:       "cancel",
			ticks:      []int{1, 1, 1},
			baseline:   0,
			minStable:  3,
			maxCrashes: 2,
			canceled:   true,
		},
		{
			name:       "stale PID crash",
			ticks:      []int{2, 2, 2, 2, 2},
			baseline:   1,
			minStable:  3,
			maxCrashes: 2,
			findErrs: map[int]error{
				2: os.ErrProcessDone,
			},
			errored: true,
		},
		{
			name:       "stale PID but fixed",
			ticks:      []int{2, 2, 3, 3, 3, 3},
			baseline:   1,
			minStable:  3,
			maxCrashes: 2,
			findErrs: map[int]error{
				2: os.ErrProcessDone,
			},
		},
		{
			name:       "error PID",
			ticks:      []int{2, 2, 3, 3, 3, 3},
			baseline:   1,
			minStable:  3,
			maxCrashes: 2,
			findErrs: map[int]error{
				2: errors.New("bad"),
			},
			errored: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			ch := make(chan int)
			go func() {
				defer cancel() // always quit after last tick
				for _, tick := range tt.ticks {
					ch <- tick
				}
			}()
			err := svc.waitForStablePID(ctx, tt.minStable, tt.maxCrashes,
				tt.baseline, ch, func(pid int) error {
					return tt.findErrs[pid]
				})
			require.Equal(t, tt.canceled, errors.Is(err, context.Canceled))
			if !tt.canceled {
				require.Equal(t, tt.errored, err != nil)
			}
		})
	}
}

func TestTickFile(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		ticks   []int
		errored bool
	}{
		{
			name:    "consistent",
			ticks:   []int{1, 1, 1},
			errored: false,
		},
		{
			name:    "divergent",
			ticks:   []int{1, 2, 3},
			errored: false,
		},
		{
			name:    "start error",
			ticks:   []int{-1, 1, 1},
			errored: false,
		},
		{
			name:    "ephemeral error",
			ticks:   []int{1, -1, 1},
			errored: false,
		},
		{
			name:    "end error",
			ticks:   []int{1, 1, -1},
			errored: true,
		},
		{
			name:    "start missing",
			ticks:   []int{0, 1, 1},
			errored: false,
		},
		{
			name:    "ephemeral missing",
			ticks:   []int{1, 0, 1},
			errored: false,
		},
		{
			name:    "end missing",
			ticks:   []int{1, 1, 0},
			errored: false,
		},
		{
			name:    "cancel-only",
			errored: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(t.TempDir(), "file")

			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			tickC := make(chan time.Time)
			ch := make(chan int)

			go func() {
				defer cancel() // always quit after last tick or fail
				for _, tick := range tt.ticks {
					_ = os.RemoveAll(filePath)
					switch {
					case tick > 0:
						err := os.WriteFile(filePath, []byte(fmt.Sprintln(tick)), os.ModePerm)
						require.NoError(t, err)
					case tick < 0:
						err := os.Mkdir(filePath, os.ModePerm)
						require.NoError(t, err)
					}
					tickC <- time.Now()
					res := <-ch
					if tick < 0 {
						tick = 0
					}
					require.Equal(t, tick, res)
				}
			}()
			err := tickFile(ctx, filePath, ch, tickC)
			require.Equal(t, tt.errored, err != nil)
		})
	}
}

func TestWaitForSocketPresent(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name      string
		timeout   time.Duration
		canceled  bool
		notSocket bool
		exists    bool

		matchErr string
	}{
		{
			name:      "missing",
			timeout:   0,
			canceled:  false,
			notSocket: false,
			exists:    false,
			matchErr:  os.ErrNotExist.Error(),
		},
		{
			name:      "context timeout",
			timeout:   10 * time.Second,
			canceled:  true,
			notSocket: false,
			exists:    false,
			matchErr:  context.Canceled.Error(),
		},
		{
			name:      "not socket",
			timeout:   0,
			canceled:  false,
			notSocket: true,
			exists:    false,
			matchErr:  "other file",
		},
		{
			name:      "present",
			timeout:   0,
			canceled:  false,
			notSocket: false,
			exists:    true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			socketFile := filepath.Join(tmp, "socket")
			svc := &SystemdService{
				Log:        slog.Default(),
				SocketPath: socketFile,
			}
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			if tt.canceled {
				cancel()
			}
			if tt.exists {
				l, err := net.Listen("unix", socketFile)
				t.Cleanup(func() { l.Close() })
				require.NoError(t, err)
			}
			if tt.notSocket {
				err := os.WriteFile(socketFile, []byte("test"), os.ModePerm)
				require.NoError(t, err)
			}
			ch := make(chan time.Time, 1)
			ch <- time.Time{}
			err := svc.waitForSocketPresent(ctx, ch, tt.timeout)
			if tt.matchErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.matchErr)
			}
		})
	}
}
