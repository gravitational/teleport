/*
Copyright 2019 Gravitational, Inc.

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

package reversetunnel

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/events"
)

func TestEmitConnTeleport(t *testing.T) {
	server, client := net.Pipe()
	const msg = "Teleport-Proxy stuff"

	go server.Write([]byte(msg))

	conn := newEmitConn(context.Background(), client, events.NewDiscardEmitterReal(), "serverid")
	buffer := make([]byte, 64)
	n, err := conn.Read(buffer)
	require.NoError(t, err)
	require.Equal(t, len(msg), n)
	require.False(t, conn.emitted)
}

func TestEmitConnNotTeleport(t *testing.T) {
	server, client := net.Pipe()
	const msg = "something other than Teleport-Proxy"

	go server.Write([]byte(msg))

	conn := newEmitConn(context.Background(), client, events.NewDiscardEmitterReal(), "serverid")
	buffer := make([]byte, 64)
	n, err := conn.Read(buffer)
	require.NoError(t, err)
	require.Equal(t, len(msg), n)
	require.True(t, conn.emitted)
}

func TestEmitConnTeleportSmallReads(t *testing.T) {
	chunks := []string{"Te", "lepo", "rt-Pro", "xy stuff", " and things"}
	server, client := net.Pipe()

	go func() {
		for _, chunk := range chunks {
			server.Write([]byte(chunk))
		}
	}()

	conn := newEmitConn(context.Background(), client, events.NewDiscardEmitterReal(), "serverid")
	buffer := make([]byte, 64)

	for _, chunk := range chunks {
		n, err := conn.Read(buffer)
		require.NoError(t, err)
		require.Equal(t, len(chunk), n)
	}

	require.False(t, conn.emitted)
}

func TestEmitConnNotTeleportSmallReads(t *testing.T) {
	chunks := []string{"no", "t Tele", "port", "-Proxy"}
	server, client := net.Pipe()

	go func() {
		for _, chunk := range chunks {
			server.Write([]byte(chunk))
		}
	}()

	conn := newEmitConn(context.Background(), client, events.NewDiscardEmitterReal(), "serverid")
	buffer := make([]byte, 64)

	for _, chunk := range chunks {
		n, err := conn.Read(buffer)
		require.NoError(t, err)
		require.Equal(t, len(chunk), n)
	}

	require.True(t, conn.emitted)
}
