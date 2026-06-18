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

package sshagent

import (
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/test/bufconn"
)

func TestConcurrentServerAccept(t *testing.T) {
	synctest.Test(t, synctestConcurrentServerAccept)
}
func synctestConcurrentServerAccept(t *testing.T) {
	l := bufconn.Listen(16384)
	defer l.Close()

	const concurrentRequests = 5

	var waiting atomic.Int32
	barrier := make(chan struct{})

	s := &Server{
		getAgent: func() (Client, error) {
			waiting.Add(1)
			<-barrier
			return nil, errors.New("nope")
		},
		listener: l,
	}
	go s.Serve()
	defer s.Close()

	conns := make([]net.Conn, 0, concurrentRequests)
	defer func() {
		for _, c := range conns {
			_ = c.Close()
		}
	}()

	for range concurrentRequests {
		c, err := l.Dial()
		require.NoError(t, err)
		conns = append(conns, c)
	}

	synctest.Wait()
	require.EqualValues(t, concurrentRequests, waiting.Load())
	close(barrier)
	for _, c := range conns {
		n, err := c.Read(make([]byte, 1))
		require.Zero(t, n)
		require.ErrorIs(t, err, io.EOF)
	}
}
