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
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLineLogger(t *testing.T) {
	t.Parallel()

	out := &bytes.Buffer{}
	ll := lineLogger{
		ctx: context.Background(),
		log: slog.New(slog.NewTextHandler(out,
			&slog.HandlerOptions{ReplaceAttr: msgOnly},
		)),
	}

	for _, e := range []struct {
		v string
		n int
	}{
		{v: "", n: 0},
		{v: "a", n: 1},
		{v: "b\n", n: 2},
		{v: "c\nd", n: 3},
		{v: "e\nf\ng", n: 5},
		{v: "h", n: 1},
		{v: "", n: 0},
		{v: "\n", n: 1},
		{v: "i\n", n: 2},
		{v: "j", n: 1},
	} {
		n, err := ll.Write([]byte(e.v))
		require.NoError(t, err)
		require.Equal(t, e.n, n)
	}
	require.Equal(t, "msg=ab\nmsg=c\nmsg=de\nmsg=f\nmsg=gh\nmsg=i\n", out.String())
	ll.Flush()
	require.Equal(t, "msg=ab\nmsg=c\nmsg=de\nmsg=f\nmsg=gh\nmsg=i\nmsg=j\n", out.String())
}

func msgOnly(_ []string, a slog.Attr) slog.Attr {
	switch a.Key {
	case "time", "level":
		return slog.Attr{}
	}
	return slog.Attr{Key: a.Key, Value: a.Value}
}

func TestMonitor(t *testing.T) {
	t.Parallel()

	svc := &SystemdService{
		Log: slog.Default(),
	}

	for _, tt := range []struct {
		name     string
		ticks    []int64
		maxStops int
		minClean int
		errored  bool
		canceled bool
	}{
		{
			name:     "one restart",
			ticks:    []int64{1, 1, 1, 1},
			maxStops: 2,
			minClean: 3,
			errored:  false,
		},
		{
			name:     "two restarts",
			ticks:    []int64{1, 1, 1, 2, 2, 2, 2},
			maxStops: 2,
			minClean: 3,
			errored:  false,
		},
		{
			name:     "too many restarts long",
			ticks:    []int64{1, 1, 1, 2, 2, 2, 3},
			maxStops: 2,
			minClean: 3,
			errored:  true,
		},
		{
			name:     "too many restarts short",
			ticks:    []int64{1, 2, 3},
			maxStops: 2,
			minClean: 3,
			errored:  true,
		},
		{
			name:     "too many restarts after okay",
			ticks:    []int64{1, 1, 1, 1, 2, 3},
			maxStops: 2,
			minClean: 3,
			errored:  false,
		},
		{
			name:     "too many restarts before okay",
			ticks:    []int64{1, 2, 3, 3, 3, 3},
			maxStops: 2,
			minClean: 3,
			errored:  true,
		},
		{
			name:     "no error if no minClean",
			ticks:    []int64{1, 2, 3},
			maxStops: 2,
			minClean: 0,
			errored:  false,
		},
		{
			name:     "cancel",
			maxStops: 2,
			minClean: 3,
			canceled: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			ch := make(chan int64)
			go func() {
				defer cancel() // always quit after last tick
				for _, tick := range tt.ticks {
					ch <- tick
				}
			}()
			err := svc.monitorRestarts(ctx, ch, tt.maxStops, tt.minClean)
			require.Equal(t, tt.canceled, errors.Is(err, context.Canceled))
			if !tt.canceled {
				require.Equal(t, tt.errored, err != nil)
			}
		})
	}
}

func TestTicks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	restartPath := filepath.Join(dir, "restart")
	svc := &SystemdService{
		Log:             slog.Default(),
		LastRestartPath: restartPath,
	}

	for _, tt := range []struct {
		name    string
		ticks   []int64
		errored bool
	}{
		{
			name:    "consistent",
			ticks:   []int64{1, 1, 1},
			errored: false,
		},
		{
			name:    "divergent",
			ticks:   []int64{1, 2, 3},
			errored: false,
		},
		{
			name:    "start error",
			ticks:   []int64{-1, 1, 1},
			errored: false,
		},
		{
			name:    "ephemeral error",
			ticks:   []int64{1, -1, 1},
			errored: false,
		},
		{
			name:    "end error",
			ticks:   []int64{1, 1, -1},
			errored: true,
		},
		{
			name: "cancel",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			tickC := make(chan time.Time)
			ch := make(chan int64)

			go func() {
				defer cancel() // always quit after last tick or fail
				for _, tick := range tt.ticks {
					if tick >= 0 {
						err := os.WriteFile(restartPath, []byte(fmt.Sprintln(tick)), os.ModePerm)
						require.NoError(t, err)
					} else {
						_ = os.Remove(restartPath)
					}
					tickC <- time.Now()
					res := <-ch
					if tick < 0 {
						tick = 0
					}
					require.Equal(t, tick, res)
				}
			}()
			err := svc.tickRestarts(ctx, ch, tickC)
			require.Equal(t, tt.errored, err != nil)
			if err != nil {
				require.ErrorIs(t, err, os.ErrNotExist)
			}
		})
	}
}
