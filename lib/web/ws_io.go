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
	"context"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
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

// startPingLoop starts a loop that will continuously send a ping frame through the websocket
// to prevent the connection between web client and teleport proxy from becoming idle.
// Interval is determined by the keep_alive_interval config set by user (or default).
// Loop will terminate when there is an error sending ping frame or when the context is canceled.
func startPingLoop(ctx context.Context, ws WSConn, keepAliveInterval time.Duration, log logrus.FieldLogger, onClose func() error) {
	log.Debugf("Starting websocket ping loop with interval %v.", keepAliveInterval)
	tickerCh := time.NewTicker(keepAliveInterval)
	defer tickerCh.Stop()

	for {
		select {
		case <-tickerCh.C:
			// A short deadline is used here to detect a broken connection quickly.
			// If this is just a temporary issue, we will retry shortly anyway.
			deadline := time.Now().Add(time.Second)
			if err := ws.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
				log.WithError(err).Error("Unable to send ping frame to web client")
				if onClose != nil {
					if err := onClose(); err != nil {
						log.WithError(err).Error("OnClose handler failed")
					}
				}
				return
			}
		case <-ctx.Done():
			log.Debug("Terminating websocket ping loop.")
			return
		}
	}
}
