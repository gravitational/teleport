/*
Copyright 2024 Gravitational, Inc.

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

package client

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/gobwas/ws"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
)

func applyWebSocketUpgradeHeaders(req *http.Request, alpnUpgradeType, challengeKey string) {
	// Set standard WebSocket upgrade type.
	req.Header.Add(constants.WebAPIConnUpgradeHeader, constants.WebAPIConnUpgradeTypeWebSocket)

	// Set "Connection" header to meet RFC spec:
	// https://datatracker.ietf.org/doc/html/rfc2616#section-14.42
	// Quote: "the upgrade keyword MUST be supplied within a Connection header
	// field (section 14.10) whenever Upgrade is present in an HTTP/1.1
	// message."
	req.Header.Set(constants.WebAPIConnUpgradeConnectionHeader, constants.WebAPIConnUpgradeConnectionType)

	// Set alpnUpgradeType as sub protocol.
	req.Header.Set(websocketHeaderKeyProtocol, alpnUpgradeType)
	req.Header.Set(websocketHeaderKeyVersion, "13")
	req.Header.Set(websocketHeaderKeyChallengeKey, challengeKey)
}

func computeWebSocketAcceptKey(challengeKey string) string {
	h := sha1.New()
	h.Write([]byte(challengeKey))
	h.Write([]byte(websocketAcceptKeyMagicString))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func generateWebSocketChallengeKey() (string, error) {
	// Quote from https://www.rfc-editor.org/rfc/rfc6455:
	//
	// A |Sec-WebSocket-Key| header field with a base64-encoded (see Section 4
	// of [RFC4648]) value that, when decoded, is 16 bytes in length.
	p := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, p); err != nil {
		return "", trace.Wrap(err)
	}
	return base64.StdEncoding.EncodeToString(p), nil
}

func checkWebSocketUpgradeResponse(resp *http.Response, alpnUpgradeType, challengeKey string) error {
	if alpnUpgradeType != resp.Header.Get(websocketHeaderKeyProtocol) {
		return trace.BadParameter("WebSocket handshake failed: Sec-WebSocket-Protocol does not match")
	}
	if computeWebSocketAcceptKey(challengeKey) != resp.Header.Get(websocketHeaderKeyAccept) {
		return trace.BadParameter("WebSocket handshake failed: invalid Sec-WebSocket-Accept")
	}
	return nil
}

type websocketALPNClientConn struct {
	net.Conn
	readBuffer []byte
	readMutex  sync.Mutex
	writeMutex sync.Mutex
}

func newWebSocketALPNClientConn(conn net.Conn) *websocketALPNClientConn {
	return &websocketALPNClientConn{
		Conn: conn,
	}
}

func (c *websocketALPNClientConn) Read(b []byte) (int, error) {
	c.readMutex.Lock()
	defer c.readMutex.Unlock()

	n, err := c.readLocked(b)
	return n, trace.Wrap(err)
}

func (c *websocketALPNClientConn) readLocked(b []byte) (int, error) {
	if len(c.readBuffer) > 0 {
		n := copy(b, c.readBuffer)
		if n < len(c.readBuffer) {
			c.readBuffer = c.readBuffer[n:]
		} else {
			c.readBuffer = nil
		}
		return n, nil
	}

	for {
		frame, err := ws.ReadFrame(c.Conn)
		if err != nil {
			return 0, trace.Wrap(err)
		}

		switch frame.Header.OpCode {
		case ws.OpClose:
			// TODO(greedy52) properly exchange close message.
			return 0, io.EOF
		case ws.OpPing:
			pong := ws.NewPongFrame(frame.Payload)
			if err := c.writeFrame(pong); err != nil {
				return 0, trace.Wrap(err)
			}
		case ws.OpBinary:
			c.readBuffer = frame.Payload
			return c.readLocked(b)
		}
	}
}

func (c *websocketALPNClientConn) Write(b []byte) (int, error) {
	frame := ws.NewFrame(ws.OpBinary, true, b)
	return len(b), trace.Wrap(c.writeFrame(frame))
}

func (c *websocketALPNClientConn) writeFrame(frame ws.Frame) error {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	// By RFC standard, all client frames must be masked:
	// https://datatracker.ietf.org/doc/html/rfc6455#section-5.1
	// TODO(greedy52) properly mask the frame with ws.MaskFrame once server
	// side is updated to properly unmask the frame. Currently a zero-mask is
	// used and the XOR operation does not alter the payload at all.
	frame.Header.Masked = true
	return trace.Wrap(ws.WriteFrame(c.Conn, frame))
}

const (
	websocketHeaderKeyProtocol     = "Sec-WebSocket-Protocol"
	websocketHeaderKeyVersion      = "Sec-WebSocket-Version"
	websocketHeaderKeyChallengeKey = "Sec-WebSocket-Key"
	websocketHeaderKeyAccept       = "Sec-WebSocket-Accept"

	// websocketAcceptKeyMagicString is the magic string used for computing
	// the accept key during WebSocket handshake.
	//
	// RFC reference:
	// https://www.rfc-editor.org/rfc/rfc6455
	//
	// Server side uses gorilla:
	// https://github.com/gorilla/websocket/blob/dcea2f088ce10b1b0722c4eb995a4e145b5e9047/util.go#L17-L24
	websocketAcceptKeyMagicString = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
)
