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
	"bytes"
	"context"
	"encoding/binary"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/player"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	messageTypePTY       = byte(1)
	messageTypeError     = byte(2)
	messageTypePlayPause = byte(3)
	messageTypeSeek      = byte(4)
	messageTypeResize    = byte(5)
)

const (
	severityError = byte(1)
)

const (
	actionPlay  = byte(0)
	actionPause = byte(1)
)

func (h *Handler) ttyPlaybackHandle(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	sctx *SessionContext,
	site reversetunnelclient.RemoteSite,
) (interface{}, error) {
	sID := p.ByName("sid")
	if sID == "" {
		return nil, trace.BadParameter("missing session ID in request URL")
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	h.log.Debug("upgrading to websocket")
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Warn("failed upgrade", err)
		// if Upgrade fails, it automatically replies with an HTTP error
		// (this means we don't need to return an error here)
		return nil, nil
	}

	player, err := player.New(&player.Config{
		Clock:     h.clock,
		Log:       h.log,
		SessionID: session.ID(sID),
		Streamer:  clt,
	})
	if err != nil {
		h.log.Warn("player error", err)
		writeError(ws, err)
		return nil, nil
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		defer cancel()
		for {
			typ, b, err := ws.ReadMessage()
			if err != nil {
				if !utils.IsOKNetworkError(err) {
					log.Warnf("websocket read error: %v", err)
				}
				return
			}

			if typ != websocket.BinaryMessage {
				log.Debugf("skipping unknown websocket message type %v", typ)
				continue
			}

			if err := handlePlaybackAction(b, player); err != nil {
				log.Warnf("skipping bad action: %v", err)
				continue
			}
		}
	}()

	go func() {
		defer cancel()
		defer func() {
			h.log.Debug("closing websocket")
			if err := ws.WriteMessage(websocket.CloseMessage, nil); err != nil {
				h.log.Debugf("error sending close message: %v", err)
			}
			if err := ws.Close(); err != nil {
				h.log.Debugf("error closing websocket: %v", err)
			}
		}()

		player.Play()
		defer player.Close()

		headerBuf := make([]byte, 11)
		headerBuf[0] = messageTypePTY

		writePTY := func(b []byte, delay uint64) error {
			writer, err := ws.NextWriter(websocket.BinaryMessage)
			if err != nil {
				return trace.Wrap(err, "getting websocket writer")
			}
			msgLen := uint16(len(b) + 8)
			binary.BigEndian.PutUint16(headerBuf[1:], msgLen)
			binary.BigEndian.PutUint64(headerBuf[3:], delay)
			if _, err := writer.Write(headerBuf); err != nil {
				return trace.Wrap(err, "writing message header")
			}

			// TODO(zmb3): consider optimizing this by bufering for very large sessions
			// (wait up to N ms to batch events into a single websocket write).
			if _, err := writer.Write(b); err != nil {
				return trace.Wrap(err, "writing PTY data")
			}

			if err := writer.Close(); err != nil {
				return trace.Wrap(err, "closing websocket writer")
			}

			return nil
		}

		writeSize := func(size string) error {
			ts, err := session.UnmarshalTerminalParams(size)
			if err != nil {
				h.log.Debugf("Ignoring invalid terminal size %q", size)
				return nil // don't abort playback due to a bad event
			}

			msg := make([]byte, 7)
			msg[0] = messageTypeResize
			binary.BigEndian.PutUint16(msg[1:], 4)
			binary.BigEndian.PutUint16(msg[3:], uint16(ts.W))
			binary.BigEndian.PutUint16(msg[5:], uint16(ts.H))

			return trace.Wrap(ws.WriteMessage(websocket.BinaryMessage, msg))
		}

		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-player.C():
				if !ok {
					// send any playback errors to the browser
					if err := writeError(ws, player.Err()); err != nil {
						h.log.Warnf("failed to send error message to browser: %v", err)
					}
					return
				}

				switch evt := evt.(type) {
				case *events.SessionStart:
					if err := writeSize(evt.TerminalSize); err != nil {
						h.log.Debugf("Failed to write resize: %v", err)
						return
					}

				case *events.SessionPrint:
					if err := writePTY(evt.Data, uint64(evt.DelayMilliseconds)); err != nil {
						h.log.Debugf("Failed to send PTY data: %v", err)
						return
					}

				case *events.SessionEnd:
					// send empty PTY data - this will ensure that any dead time
					// at the end of the recording is processed and the allow
					// the progress bar to go to 100%
					if err := writePTY(nil, uint64(evt.EndTime.Sub(evt.StartTime)/time.Millisecond)); err != nil {
						h.log.Debugf("Failed to send session end data: %v", err)
						return
					}

				case *events.Resize:
					if err := writeSize(evt.TerminalSize); err != nil {
						h.log.Debugf("Failed to write resize: %v", err)
						return
					}

				case *events.SessionLeave: // do nothing

				default:
					h.log.Debugf("unexpected event type %T", evt)
				}
			}
		}
	}()

	<-ctx.Done()
	return nil, nil
}

func writeError(ws *websocket.Conn, err error) error {
	if err == nil {
		return nil
	}

	b := new(bytes.Buffer)
	b.WriteByte(messageTypeError)

	msg := trace.UserMessage(err)
	l := 1 /* severity */ + 2 /* msg length */ + len(msg)
	binary.Write(b, binary.BigEndian, uint16(l))
	b.WriteByte(severityError)
	binary.Write(b, binary.BigEndian, uint16(len(msg)))
	b.WriteString(msg)

	return trace.Wrap(ws.WriteMessage(websocket.BinaryMessage, b.Bytes()))
}

type play interface {
	Play() error
	Pause() error
	SetPos(time.Duration) error
}

// handlePlaybackAction processes a playback message
// received from the browser
func handlePlaybackAction(b []byte, p play) error {
	if len(b) < 3 {
		return trace.BadParameter("invalid playback message")
	}

	msgType := b[0]
	msgLen := binary.BigEndian.Uint16(b[1:])

	if len(b) < int(msgLen)+3 {
		return trace.BadParameter("invalid message length")
	}

	payload := b[3:]
	payload = payload[:msgLen]

	switch msgType {
	case messageTypePlayPause:
		if len(payload) != 1 {
			return trace.BadParameter("invalid play/pause command")
		}
		switch action := payload[0]; action {
		case actionPlay:
			p.Play()
		case actionPause:
			p.Pause()
		default:
			return trace.BadParameter("invalid play/pause action %v", action)
		}
	case messageTypeSeek:
		if len(payload) != 8 {
			return trace.BadParameter("invalid seek message")
		}
		pos := binary.BigEndian.Uint64(payload)
		p.SetPos(time.Duration(pos) * time.Millisecond)
	}

	return nil
}

/*

# Websocket Protocol for TTY Playback:

During playback, the Teleport proxy sends session data to the browser
and the browser sends playback commands (play/pause, seek, etc) to the
proxy.

Each message conforms to the following binary protocol.

## Message Header

The message header starts with a 1-byte identifier followed by a 2-byte
(big endian) integer containing the number of bytes following the header.
This length field does not include the 3-byte header.

## Messages

### 1 - PTY data

This message is used to send recorded PTY data to the browser.

- Message ID: 1
- 8-byte timestamp (milliseconds since session start)
- PTY data

### 2 - Error

This message is used to indicate that an error has occurred.

- Message ID: 2
- 1 byte severity (1=error)
- 2-byte error message length
- variable length error message (UTF-8 text)

### 3 - Play/Pause

This message is sent from the browser to the server to pause
or resume playback.

- Message ID: 3
- 1-byte code (0=play, 1=pause)

### 4 - Seek

This message is used to seek to a new position in the recording.

- Message ID: 4
- 8-byte timestamp (milliseconds since session start)

### 5 - Resize

This message is used to indicate that the terminal was resized.

- Message ID: 5
- 2-byte width
- 2-byte height

*/
