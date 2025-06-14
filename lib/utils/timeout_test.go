/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package utils

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestObeyIdleTimeout(t *testing.T) {
	clock := clockwork.NewFakeClock()
	t.Cleanup(func() { clock.Advance(time.Hour) })

	c1, c2 := net.Pipe()
	c1 = obeyIdleTimeoutClock(c1, time.Minute, clock)
	t.Cleanup(func() { c1.Close() })
	t.Cleanup(func() { c2.Close() })

	go func() {
		c2.Write([]byte{0})
		clock.Sleep(30 * time.Second)
		c2.Write([]byte{0})
	}()

	errC := make(chan error, 3)
	go func() {
		var b [1]byte
		for range 3 {
			_, err := io.ReadFull(c1, b[:])
			errC <- err
		}
	}()

	err1 := <-errC
	// wait for the writing goroutine to be waiting as well (the watchdog counts
	// as a waiter)
	clock.BlockUntil(2)
	clock.Advance(30 * time.Second)
	err2 := <-errC
	clock.Advance(30 * time.Second)
	select {
	case err := <-errC:
		require.FailNow(t, "expected Read to block", "got err %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	clock.Advance(30 * time.Second)
	err3 := <-errC

	require.NoError(t, err1)
	require.NoError(t, err2)
	// this should be net.ErrClosed, but net.Pipe uses io.ErrClosedPipe and it
	// can't be changed
	require.ErrorIs(t, err3, io.ErrClosedPipe)
}
