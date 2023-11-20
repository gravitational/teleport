/*
Copyright 2023 Gravitational, Inc.

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

package mongodb

import (
	"net"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
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
			Log:   logrus.StandardLogger(),
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
			Log:   logrus.StandardLogger(),
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
