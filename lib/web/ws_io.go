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

package web

import (
	"context"
	"log/slog"
	"time"

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

type wsPinger interface {
	WriteControl(messageType int, data []byte, deadline time.Time) error
}

// startWSPingLoop starts a loop that will continuously send a ping frame through the websocket
// to prevent the connection between web client and teleport proxy from becoming idle.
// Interval is determined by the keep_alive_interval config set by user (or default).
// Loop will terminate when there is an error sending ping frame or when the context is canceled.
func startWSPingLoop(ctx context.Context, pinger wsPinger, keepAliveInterval time.Duration, log *slog.Logger, onClose func() error) {
	log.DebugContext(ctx, "Starting websocket ping loop with interval", "interval", keepAliveInterval)
	tickerCh := time.NewTicker(keepAliveInterval)
	defer tickerCh.Stop()

	for {
		select {
		case <-tickerCh.C:
			// A short deadline is used here to detect a broken connection quickly.
			// If this is just a temporary issue, we will retry shortly anyway.
			deadline := time.Now().Add(time.Second)
			if err := pinger.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
				log.ErrorContext(ctx, "Unable to send ping frame to web client", "error", err)
				if onClose != nil {
					if err := onClose(); err != nil {
						log.ErrorContext(ctx, "OnClose handler failed", "error", err)
					}
				}
				return
			}
		case <-ctx.Done():
			log.DebugContext(ctx, "Terminating websocket ping loop.")
			return
		}
	}
}
