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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
)

// TestTerminalReadFromClosedConn verifies that Teleport recovers
// from a closed websocket connection.
// See https://github.com/gravitational/teleport/issues/21334
func TestTerminalReadFromClosedConn(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var upgrader = websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("couldn't upgrade websocket connection: %v", err)
		}

		envelope := Envelope{
			Type:    defaults.WebsocketRaw,
			Payload: "hello",
		}
		b, err := proto.Marshal(&envelope)
		if err != nil {
			t.Errorf("could not marshal envelope: %v", err)
		}
		conn.WriteMessage(websocket.BinaryMessage, b)
	}))
	t.Cleanup(server.Close)

	u := strings.Replace(server.URL, "http:", "ws:", 1)
	conn, resp, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	stream := NewTerminalStream(ctx, TerminalStreamConfig{WS: conn, Logger: utils.NewLoggerForTests()})

	// close the stream before we attempt to read from it,
	// this will produce a net.ErrClosed error on the read
	require.NoError(t, stream.Close())

	_, err = io.Copy(io.Discard, stream)
	require.NoError(t, err)
}

type terminal struct {
	ws     *websocket.Conn
	stream *TerminalStream

	sessionC chan session.Session
}

type connectConfig struct {
	pack              *authPack
	host              string
	proxy             string
	sessionID         session.ID
	participantMode   types.SessionParticipantMode
	keepAliveInterval time.Duration
	mfaCeremony       func(challenge client.MFAAuthenticateChallenge) []byte
	handlers          map[string]WSHandlerFunc
	pingHandler       func(WSConn, string) error
}

func connectToHost(ctx context.Context, cfg connectConfig) (*terminal, error) {
	req := TerminalRequest{
		Server: cfg.host,
		Login:  cfg.pack.login,
		Term: session.TerminalParams{
			W: 100,
			H: 100,
		},
		SessionID:         cfg.sessionID,
		ParticipantMode:   cfg.participantMode,
		KeepAliveInterval: cfg.keepAliveInterval,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	u := url.URL{
		Host:   cfg.proxy,
		Scheme: client.WSS,
		Path:   "/v1/webapi/sites/-current-/connect/ws",
	}

	q := u.Query()
	q.Set("params", string(data))
	u.RawQuery = q.Encode()

	header := http.Header{}
	header.Add("Origin", "http://localhost")
	for _, cookie := range cfg.pack.cookies {
		header.Add("Cookie", cookie.String())
	}

	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	ws, resp, err := dialer.Dial(u.String(), header)
	if err != nil {
		var sb strings.Builder
		sb.WriteString("websocket dial")
		if resp != nil {
			fmt.Fprintf(&sb, "; status code %v;", resp.StatusCode)
			fmt.Fprintf(&sb, "headers: %v; body: ", resp.Header)
			io.Copy(&sb, resp.Body)
		}
		return nil, trace.Wrap(err, sb.String())
	}
	if err := resp.Body.Close(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := makeAuthReqOverWS(ws, cfg.pack.session.Token); err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.pingHandler != nil {
		ws.SetPingHandler(func(message string) error {
			return cfg.pingHandler(ws, message)
		})
	}

	t := &terminal{ws: ws, sessionC: make(chan session.Session, 1)}

	// If MFA is expected, it should be performed prior to creating
	// the TerminalStream to avoid messages being handled by multiple
	// readers.
	if cfg.mfaCeremony != nil {
		if err := t.performMFACeremony(cfg.mfaCeremony); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if cfg.handlers == nil {
		cfg.handlers = map[string]WSHandlerFunc{}
	}

	if _, ok := cfg.handlers[defaults.WebsocketSessionMetadata]; !ok {
		cfg.handlers[defaults.WebsocketSessionMetadata] = func(ctx context.Context, envelope Envelope) {
			if envelope.Type != defaults.WebsocketSessionMetadata {
				return
			}

			var sessResp siteSessionGenerateResponse
			if err := json.Unmarshal([]byte(envelope.Payload), &sessResp); err != nil {
				return
			}

			t.sessionC <- sessResp.Session
		}
	}

	t.stream = NewTerminalStream(ctx, TerminalStreamConfig{
		WS:       ws,
		Logger:   utils.NewLogger(),
		Handlers: cfg.handlers,
	})

	return t, nil
}

func (t *terminal) GetSession() session.Session {
	sess := <-t.sessionC
	t.sessionC <- sess

	return sess
}

func (t *terminal) Close() error {
	return t.stream.Close()
}

func (t *terminal) Write(p []byte) (int, error) {
	return t.stream.Write(p)
}

func (t *terminal) Read(p []byte) (int, error) {
	return t.stream.Read(p)
}

func (t *terminal) SetReadDeadline(deadline time.Time) error {
	return t.stream.ws.SetReadDeadline(deadline)
}

func (t *terminal) SetWriteDeadline(deadline time.Time) error {
	return t.stream.ws.SetWriteDeadline(deadline)
}

func (t *terminal) performMFACeremony(ceremonyFn func(challenge client.MFAAuthenticateChallenge) []byte) error {
	// Wait for websocket authn challenge event.
	ty, raw, err := t.ws.ReadMessage()
	if err != nil {
		return trace.Wrap(err, "reading ws message")
	}

	if ty != websocket.BinaryMessage {
		return trace.BadParameter("got unexpected websocket message type %d", ty)
	}

	var env Envelope
	if err := proto.Unmarshal(raw, &env); err != nil {
		return trace.Wrap(err, "unmarshalling envelope")
	}

	var challenge client.MFAAuthenticateChallenge
	if err := json.Unmarshal([]byte(env.Payload), &challenge); err != nil {
		return trace.Wrap(err, "unmarshalling challenge")
	}

	// Send response over ws.
	if err := t.ws.WriteMessage(websocket.BinaryMessage, ceremonyFn(challenge)); err != nil {
		return trace.Wrap(err, "sending challenge response")
	}

	return nil
}
