// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		for i := 0; i < 3; i++ {
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
