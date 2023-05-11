/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package web

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/session"
)

func TestExecuteCommand(t *testing.T) {
	t.Parallel()
	s := newWebSuite(t)

	ws, _, err := s.makeCommand(t, s.authPack(t, "foo"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ws.Close()) })

	stream := NewWStream(ws)

	require.NoError(t, waitForCommandOutput(stream, "teleport"))
}

func (s *WebSuite) makeCommand(t *testing.T, pack *authPack) (*websocket.Conn, *session.Session, error) {
	req := CommandRequest{
		Query:          fmt.Sprintf("name == \"%s\"", s.srvID),
		Login:          pack.login,
		ConversationID: uuid.New().String(),
		ExecutionID:    uuid.New().String(),
		Command:        "echo txlxport | sed 's/x/e/g'",
	}

	u := url.URL{
		Host:   s.url().Host,
		Scheme: client.WSS,
		Path:   fmt.Sprintf("/v1/webapi/command/%v/execute", currentSiteShortcut),
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, nil, err
	}

	q := u.Query()
	q.Set("params", string(data))
	q.Set(roundtrip.AccessTokenQueryParam, pack.session.Token)
	u.RawQuery = q.Encode()

	dialer := websocket.Dialer{}
	dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	header := http.Header{}
	header.Add("Origin", "http://localhost")
	for _, cookie := range pack.cookies {
		header.Add("Cookie", cookie.String())
	}

	ws, resp, err := dialer.Dial(u.String(), header)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	ty, raw, err := ws.ReadMessage()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	require.Equal(t, websocket.BinaryMessage, ty)
	var env Envelope

	err = proto.Unmarshal(raw, &env)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	var sessResp siteSessionGenerateResponse

	err = json.Unmarshal([]byte(env.Payload), &sessResp)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	err = resp.Body.Close()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return ws, &sessResp.Session, nil
}

func waitForCommandOutput(stream io.Reader, substr string) error {
	timeoutCh := time.After(10 * time.Second)

	for {
		select {
		case <-timeoutCh:
			return trace.BadParameter("timeout waiting on terminal for output: %v", substr)
		default:
		}

		out := make([]byte, 100)
		n, err := stream.Read(out)
		if err != nil {
			return trace.Wrap(err)
		}

		var env Envelope
		err = json.Unmarshal(out[:n], &env)
		if err != nil {
			return trace.Wrap(err)
		}

		d, err := base64.StdEncoding.DecodeString(env.Payload)
		if err != nil {
			return trace.Wrap(err)
		}
		data := removeSpace(string(d))
		if n > 0 && strings.Contains(data, substr) {
			return nil
		}
		if err != nil {
			return trace.Wrap(err)
		}
	}
}
