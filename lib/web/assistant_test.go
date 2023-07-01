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

	"github.com/gorilla/websocket"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	authproto "github.com/gravitational/teleport/api/client/proto"
	aitest "github.com/gravitational/teleport/lib/ai/testutils"
	"github.com/gravitational/teleport/lib/assist"
	"github.com/gravitational/teleport/lib/client"
)

func Test_runAssistant(t *testing.T) {
	t.Parallel()

	readStreamResponse := func(t *testing.T, ws *websocket.Conn) string {
		var sb strings.Builder
		for {
			var msg assistantMessage
			_, payload, err := ws.ReadMessage()
			require.NoError(t, err)

			err = json.Unmarshal(payload, &msg)
			require.NoError(t, err)

			if msg.Type == assist.MessageKindAssistantPartialFinalize {
				break
			}

			require.Equal(t, assist.MessageKindAssistantPartialMessage, msg.Type)
			sb.WriteString(msg.Payload)
		}

		return sb.String()
	}

	readRateLimitedMessage := func(t *testing.T, ws *websocket.Conn) {
		var msg assistantMessage
		_, payload, err := ws.ReadMessage()
		require.NoError(t, err)

		err = json.Unmarshal(payload, &msg)
		require.NoError(t, err)

		require.Equal(t, assist.MessageKindError, msg.Type)
		require.Equal(t, msg.Payload, "You have reached the rate limit. Please try again later.")
	}

	testCases := []struct {
		name      string
		responses []string
		cfg       webSuiteConfig
		setup     func(*testing.T, *WebSuite)
		act       func(*testing.T, *websocket.Conn)
	}{
		{
			name: "normal",
			responses: []string{
				generateTextResponse(),
			},
			act: func(t *testing.T, ws *websocket.Conn) {
				err := ws.WriteMessage(websocket.TextMessage, []byte(`{"payload": "show free disk space"}`))
				require.NoError(t, err)

				const expectedMsg = "Which node do you want to use?"
				require.Contains(t, readStreamResponse(t, ws), expectedMsg)
			},
		},
		{
			name: "rate limited",
			responses: []string{
				generateTextResponse(),
			},
			cfg: webSuiteConfig{
				ClusterFeatures: &authproto.Features{
					Cloud: true,
				},
			},
			setup: func(t *testing.T, s *WebSuite) {
				// Assert that rate limiter is set up when Cloud feature is active,
				// before replacing with a lower capacity rate-limiter for test purposes
				require.Equal(t, assistantLimiterRate, s.webHandler.handler.assistantLimiter.Limit())

				// 101 token capacity (lookaheadTokens+1) and a slow replenish rate
				// to let the first completion request succeed, but not the second one
				s.webHandler.handler.assistantLimiter = rate.NewLimiter(rate.Limit(0.001), 101)

			},
			act: func(t *testing.T, ws *websocket.Conn) {
				err := ws.WriteMessage(websocket.TextMessage, []byte(`{"payload": "show free disk space"}`))
				require.NoError(t, err)

				const expectedMsg = "Which node do you want to use?"
				require.Contains(t, readStreamResponse(t, ws), expectedMsg)

				err = ws.WriteMessage(websocket.TextMessage, []byte(`{"payload": "all nodes, please"}`))
				require.NoError(t, err)

				readRateLimitedMessage(t, ws)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			responses := tc.responses
			server := httptest.NewServer(aitest.GetTestHandlerFn(t, responses))
			t.Cleanup(server.Close)

			openaiCfg := openai.DefaultConfig("test-token")
			openaiCfg.BaseURL = server.URL
			tc.cfg.OpenAIConfig = &openaiCfg
			s := newWebSuiteWithConfig(t, tc.cfg)

			if tc.setup != nil {
				tc.setup(t, s)
			}

			ctx := context.Background()
			authPack := s.authPack(t, "foo")
			// Create the conversation
			conversationID := s.makeAssistConversation(t, ctx, authPack)

			// Make WS client and start the conversation
			ws, err := s.makeAssistant(t, authPack, conversationID)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, ws.Close()) })

			_, payload, err := ws.ReadMessage()
			require.NoError(t, err)

			var msg assistantMessage
			err = json.Unmarshal(payload, &msg)
			require.NoError(t, err)

			// Expect "hello" message
			require.Equal(t, assist.MessageKindAssistantMessage, msg.Type)
			require.Contains(t, msg.Payload, "Hey, I'm Teleport")

			tc.act(t, ws)
		})
	}
}

// Test_runAssistError tests that the assistant returns an error message
// when the OpenAI API returns an error.
func Test_runAssistError(t *testing.T) {
	t.Parallel()

	readHelloMsg := func(ws *websocket.Conn) {
		_, payload, err := ws.ReadMessage()
		require.NoError(t, err)

		var msg assistantMessage
		err = json.Unmarshal(payload, &msg)
		require.NoError(t, err)

		// Expect "hello" message
		require.Equal(t, assist.MessageKindAssistantMessage, msg.Type)
		require.Contains(t, msg.Payload, "Hey, I'm Teleport")
	}

	readErrorMsg := func(ws *websocket.Conn) {
		err := ws.WriteMessage(websocket.TextMessage, []byte(`{"payload": "show free disk space"}`))
		require.NoError(t, err)

		_, payload, err := ws.ReadMessage()
		require.NoError(t, err)

		var msg assistantMessage
		err = json.Unmarshal(payload, &msg)
		require.NoError(t, err)

		// Expect OpenAI error message
		require.Equal(t, assist.MessageKindError, msg.Type)
		require.Contains(t, msg.Payload, "An error has occurred. Please try again later.")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Simulate rate limit error
		w.WriteHeader(429)

		errMsg := openai.ErrorResponse{
			Error: &openai.APIError{
				Code:           "rate_limit_reached",
				Message:        "You are sending requests too quickly.",
				Param:          nil,
				Type:           "rate_limit_reached",
				HTTPStatusCode: 429,
			},
		}

		dataBytes, err := json.Marshal(errMsg)
		// Use assert as require doesn't work when called from a goroutine
		assert.NoError(t, err, "Marshal error")

		_, err = w.Write(dataBytes)
		assert.NoError(t, err, "Write error")
	}))
	t.Cleanup(server.Close)

	openaiCfg := openai.DefaultConfig("test-token")
	openaiCfg.BaseURL = server.URL
	s := newWebSuiteWithConfig(t, webSuiteConfig{OpenAIConfig: &openaiCfg})

	ctx := context.Background()
	authPack := s.authPack(t, "foo")
	// Create the conversation
	conversationID := s.makeAssistConversation(t, ctx, authPack)

	// Make WS client and start the conversation
	ws, err := s.makeAssistant(t, authPack, conversationID)
	require.NoError(t, err)
	t.Cleanup(func() {
		// Close should yield an error as the server closes the connection
		require.Error(t, ws.Close())
	})

	// verify responses
	readHelloMsg(ws)
	readErrorMsg(ws)

	// Check for the close message
	_, _, err = ws.ReadMessage()
	closeErr, ok := err.(*websocket.CloseError)
	require.True(t, ok, "Expected close error")
	require.Equal(t, websocket.CloseInternalServerErr, closeErr.Code, "Expected abnormal closure")
}

// makeAssistConversation creates a new assist conversation and returns its ID
func (s *WebSuite) makeAssistConversation(t *testing.T, ctx context.Context, authPack *authPack) string {
	clt := authPack.clt

	resp, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "assistant", "conversations"), nil)
	require.NoError(t, err)

	convResp := struct {
		ConversationID string `json:"id"`
	}{}
	err = json.Unmarshal(resp.Bytes(), &convResp)
	require.NoError(t, err)

	return convResp.ConversationID
}

// makeAssistant creates a new assistant websocket connection.
func (s *WebSuite) makeAssistant(t *testing.T, pack *authPack, conversationID string) (*websocket.Conn, error) {
	u := url.URL{
		Host:   s.url().Host,
		Scheme: client.WSS,
		Path:   fmt.Sprintf("/v1/webapi/sites/%s/assistant", currentSiteShortcut),
	}

	q := u.Query()
	q.Set("conversation_id", conversationID)
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

// generateTextResponse generates a response for a text completion
func generateTextResponse() string {
	return "<FINAL RESPONSE>\nWhich node do you want to use?"
}
