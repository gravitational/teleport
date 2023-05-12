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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/assist"
	"github.com/gravitational/teleport/lib/client"
)

func Test_runAssistant(t *testing.T) {
	t.Parallel()

	responses := [][]byte{
		generateTextResponse(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		require.GreaterOrEqual(t, len(responses), 1, "Unexpected request")
		dataBytes := responses[0]

		_, err := w.Write(dataBytes)
		require.NoError(t, err, "Write error")

		responses = responses[1:]
	}))
	defer server.Close()

	openaiCfg := openai.DefaultConfig("test-token")
	openaiCfg.BaseURL = server.URL
	s := newWebSuiteWithConfig(t, webSuiteConfig{OpenAIConfig: &openaiCfg})

	ws, err := s.makeAssistant(t, s.authPack(t, "foo"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ws.Close()) })

	_, payload, err := ws.ReadMessage()
	require.NoError(t, err)

	var msg assistantMessage
	err = json.Unmarshal(payload, &msg)
	require.NoError(t, err)

	require.Equal(t, assist.MessageKindAssistantMessage, msg.Type)
	require.Contains(t, msg.Payload, "Hey, I'm Teleport")

	err = ws.WriteMessage(websocket.TextMessage, []byte(`{"payload": "show free disk space"}`))
	require.NoError(t, err)

	readPartialMessage := func() string {
		_, payload, err = ws.ReadMessage()
		require.NoError(t, err)

		err = json.Unmarshal(payload, &msg)
		require.NoError(t, err)

		require.Equal(t, assist.MessageKindAssistantPartialMessage, msg.Type)
		return msg.Payload
	}

	require.Contains(t, readPartialMessage(), "Which")
	require.Contains(t, readPartialMessage(), "node do")
	require.Contains(t, readPartialMessage(), "you want")
	require.Contains(t, readPartialMessage(), "use?")

	readStraemEnd := func() {
		_, payload, err = ws.ReadMessage()
		require.NoError(t, err)

		err = json.Unmarshal(payload, &msg)
		require.NoError(t, err)

		require.Equal(t, assist.MessageKindAssistantPartialFinalize, msg.Type)
	}

	readStraemEnd()
}

func (s *WebSuite) makeAssistant(t *testing.T, pack *authPack) (*websocket.Conn, error) {
	u := url.URL{
		Host:   s.url().Host,
		Scheme: client.WSS,
		Path:   fmt.Sprintf("/v1/webapi/sites/%s/assistant", currentSiteShortcut),
	}

	q := u.Query()
	q.Set("conversation_id", uuid.New().String())
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
		res, err2 := io.ReadAll(resp.Body)
		t.Log("response body:", string(res), err2)
		return nil, trace.Wrap(err)
	}

	err = resp.Body.Close()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ws, nil
}

func generateTextResponse() []byte {
	dataBytes := []byte{}
	dataBytes = append(dataBytes, []byte("event: message\n")...)

	data := `{"id":"1","object":"completion","created":1598069254,"model":"gpt-4","choices":[{"index": 0, "delta":{"content": "Which ", "role": "assistant"}}]}`
	dataBytes = append(dataBytes, []byte("data: "+data+"\n\n")...)
	dataBytes = append(dataBytes, []byte("event: message\n")...)

	data = `{"id":"2","object":"completion","created":1598069254,"model":"gpt-4","choices":[{"index": 0, "delta":{"content": "node do ", "role": "assistant"}}]}`
	dataBytes = append(dataBytes, []byte("data: "+data+"\n\n")...)
	dataBytes = append(dataBytes, []byte("event: message\n")...)

	data = `{"id":"3","object":"completion","created":1598069255,"model":"gpt-4","choices":[{"index": 0, "delta":{"content": "you want ", "role": "assistant"}}]}`
	dataBytes = append(dataBytes, []byte("data: "+data+"\n\n")...)
	dataBytes = append(dataBytes, []byte("event: message\n")...)

	data = `{"id":"4","object":"completion","created":1598069254,"model":"gpt-4","choices":[{"index": 0, "delta":{"content": "use?", "role": "assistant"}}]}`
	dataBytes = append(dataBytes, []byte("data: "+data+"\n\n")...)
	dataBytes = append(dataBytes, []byte("event: done\n")...)

	dataBytes = append(dataBytes, []byte("data: [DONE]\n\n")...)

	return dataBytes
}
