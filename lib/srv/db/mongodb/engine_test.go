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

package mongodb

import (
	"log/slog"
	"net"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/mongodb/protocol"
)

func TestEngineReplyError(t *testing.T) {
	connectError := trace.NotFound("user not found")

	clientMsgDoc, err := bson.Marshal(bson.M{
		"isMaster": 1,
	})
	require.NoError(t, err)
	clientMsg := protocol.MakeOpMsg(clientMsgDoc)
	clientMsg.Header.RequestID = 123456

	t.Run("wait for client message", func(t *testing.T) {
		t.Parallel()

		e := NewEngine(common.EngineConfig{
			Clock: clockwork.NewRealClock(),
			Log:   slog.Default(),
		}).(*Engine)

		enginePipeEndConn, clientPipeEndConn := net.Pipe()
		defer enginePipeEndConn.Close()
		defer clientPipeEndConn.Close()

		go e.replyError(enginePipeEndConn, nil, connectError)

		_, err = clientPipeEndConn.Write(clientMsg.ToWire(0))
		require.NoError(t, err)
		msg, err := protocol.ReadMessage(clientPipeEndConn, protocol.DefaultMaxMessageSizeBytes)
		require.NoError(t, err)
		require.Equal(t, clientMsg.Header.RequestID, msg.GetHeader().ResponseTo)
		require.Contains(t, msg.String(), connectError.Error())
	})

	t.Run("no wait", func(t *testing.T) {
		t.Parallel()

		e := NewEngine(common.EngineConfig{
			Clock: clockwork.NewRealClock(),
			Log:   slog.Default(),
		}).(*Engine)
		e.serverConnected = true

		enginePipeEndConn, clientPipeEndConn := net.Pipe()
		defer enginePipeEndConn.Close()
		defer clientPipeEndConn.Close()

		go e.replyError(enginePipeEndConn, nil, connectError)

		// There is no need to write a message and reply does not respond to a
		// message.
		msg, err := protocol.ReadMessage(clientPipeEndConn, protocol.DefaultMaxMessageSizeBytes)
		require.NoError(t, err)
		require.Equal(t, int32(0), msg.GetHeader().ResponseTo)
		require.Contains(t, msg.String(), connectError.Error())
	})
}
