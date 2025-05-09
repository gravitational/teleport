//go:build go1.24 && enablesynctest

/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package player_test

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/player"
)

/*
This file uses the experimental testing/synctest package introduced with Go 1.24:

    https://go.dev/blog/synctest

When editing this file, you should set GOEXPERIMENT=synctest for your editor/LSP
to ensure that the language server doesn't fail to recognize the package.

This file is also protected by a build tag to ensure that `go test` doesn't fail
for users who haven't set the environment variable.
*/

// TestInterruptsDelay tests that the player responds to playback
// controls even when it is waiting to emit an event.
func TestInterruptsDelay(t *testing.T) {
	synctest.Run(func() {
		p, err := player.New(&player.Config{
			SessionID: "test-session",
			Streamer:  &simpleStreamer{count: 3, delay: 5000},
		})
		require.NoError(t, err)

		synctest.Wait()

		require.NoError(t, p.Play())
		t.Cleanup(func() { p.Close() })

		synctest.Wait() // player is now waiting to emit event 0

		// emulate the user seeking forward while the player is waiting..
		start := time.Now()
		p.SetPos(10_001 * time.Millisecond)

		// expect event 0 and event 1 to be emitted right away
		evt0 := <-p.C()
		evt1 := <-p.C()
		require.Equal(t, int64(0), evt0.GetIndex())
		require.Equal(t, int64(1), evt1.GetIndex())

		// Time will advance automatically in the synctest bubble.
		// This assertion checks that the player was unblocked by
		// the SetPos call and not because enough time elapsed.
		require.Zero(t, time.Since(start))

		<-p.C()

		// the user seeked to 10.001 seconds, it should take 4.999
		// seconds to get the third event that arrives at second 15.
		require.Equal(t, time.Since(start), 4999*time.Millisecond)
	})
}

func TestSeekForward(t *testing.T) {
	synctest.Run(func() {
		p, err := player.New(&player.Config{
			SessionID: "test-session",
			Streamer:  &simpleStreamer{count: 1, delay: 6000},
		})
		require.NoError(t, err)
		t.Cleanup(func() { p.Close() })
		require.NoError(t, p.Play())

		start := time.Now()

		time.Sleep(100 * time.Millisecond)
		p.SetPos(500 * time.Millisecond)
		time.Sleep(100 * time.Millisecond)
		p.SetPos(5900 * time.Millisecond)

		<-p.C()

		// It should take 300ms for the event to be emitted.
		// Two 100ms sleeps (200ms), then a seek to 5.900 seconds which
		// requires another 100ms to wait for the event at 6s.
		require.Equal(t, time.Since(start), 300*time.Millisecond)
	})
}
