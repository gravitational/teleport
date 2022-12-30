/*
Copyright 2021 Gravitational, Inc.

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

package web

import (
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
)

type WebsocketIO struct {
	Conn      *websocket.Conn
	remaining []byte
}

func (ws *WebsocketIO) Write(p []byte) (int, error) {
	err := ws.Conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return len(p), nil
}

func (ws *WebsocketIO) Read(p []byte) (int, error) {
	if len(ws.remaining) == 0 {
		ty, data, err := ws.Conn.ReadMessage()
		if err != nil {
			return 0, trace.Wrap(err)
		}
		if ty != websocket.BinaryMessage {
			return 0, trace.BadParameter("expected websocket message of type BinaryMessage, got %T", ty)
		}

		ws.remaining = data
	}

	copied := copy(p, ws.remaining)
	ws.remaining = ws.remaining[copied:]
	return copied, nil
}

func (ws *WebsocketIO) Close() error {
	return trace.Wrap(ws.Conn.Close())
}
