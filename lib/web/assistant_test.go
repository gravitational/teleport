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

	"github.com/gorilla/websocket"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	authproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	aitest "github.com/gravitational/teleport/lib/ai/testutils"
	"github.com/gravitational/teleport/lib/assist"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"
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
		require.Equal(t, "You have reached the rate limit. Please try again later.", msg.Payload)
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
				require.InEpsilon(t, float64(assistantLimiterRate), float64(s.webHandler.handler.assistantLimiter.Limit()), 0.0)

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
			assistRole := allowAssistAccess(t, s)

			ctx := context.Background()
			authPack := s.authPack(t, "foo", assistRole.GetName())
			// Create the conversation
			conversationID := s.makeAssistConversation(t, ctx, authPack)

			// Make WS client and start the conversation
			ws, err := s.makeAssistant(t, authPack, conversationID, "")
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
		require.NoError(t, err, "expected error message, payload: %s", payload)

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

	assistRole := allowAssistAccess(t, s)
	authPack := s.authPack(t, "foo", assistRole.GetName())

	ctx := context.Background()
	// Create the conversation
	conversationID := s.makeAssistConversation(t, ctx, authPack)

	// Make WS client and start the conversation
	ws, err := s.makeAssistant(t, authPack, conversationID, "")
	require.NoError(t, err)
	t.Cleanup(func() {
		// The TLS connection might or might not be closed, this is an implementation detail.
		// We want to check whether the websocket gets appropriately closed, not the underlying TLS connection.
		// The connection will eventually be closed and reclaimed by the server. We can skip checking the error.
		_ = ws.Close()
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

func Test_SSHCommandGeneration(t *testing.T) {
	t.Parallel()

	assertGenCommand := func(ws *websocket.Conn) {
		_, payload, err := ws.ReadMessage()
		require.NoError(t, err)

		var msg assistantMessage
		err = json.Unmarshal(payload, &msg)
		require.NoError(t, err)

		// Expect "hello" message
		require.Equal(t, assist.MessageKindProgressUpdate, msg.Type)
		require.Contains(t, msg.Payload, "openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes -subj")
	}

	responses := []string{generateCommandResponse()}
	server := httptest.NewServer(aitest.GetTestHandlerFn(t, responses))
	t.Cleanup(server.Close)

	openaiCfg := openai.DefaultConfig("test-token")
	openaiCfg.BaseURL = server.URL
	s := newWebSuiteWithConfig(t, webSuiteConfig{OpenAIConfig: &openaiCfg})

	assistRole := allowAssistAccess(t, s)
	authPack := s.authPack(t, "foo", assistRole.GetName())

	// Make WS client and start the conversation
	ws, err := s.makeAssistant(t, authPack, "", "ssh-cmdgen")
	require.NoError(t, err)
	t.Cleanup(func() {
		err := ws.Close()
		require.NoError(t, err)
	})

	err = ws.WriteMessage(websocket.TextMessage, []byte(`{"input:" "My cert expired!!! What is x509?"}`))
	require.NoError(t, err)

	// verify responses
	assertGenCommand(ws)
}

func Test_SSHCommandExplain(t *testing.T) {
	t.Parallel()

	assertResponse := func(ws *websocket.Conn) {
		_, payload, err := ws.ReadMessage()
		require.NoError(t, err)

		var msg assistantMessage
		err = json.Unmarshal(payload, &msg)
		require.NoError(t, err)

		// Expect "hello" message
		require.Equal(t, assist.MessageKindAssistantMessage, msg.Type)
		require.Contains(t, msg.Payload, "The application has failed to connect to the database. The database is not running.")
	}

	responses := []string{commandSummaryResponse()}
	server := httptest.NewServer(aitest.GetTestHandlerFn(t, responses))
	t.Cleanup(server.Close)

	openaiCfg := openai.DefaultConfig("test-token")
	openaiCfg.BaseURL = server.URL
	s := newWebSuiteWithConfig(t, webSuiteConfig{OpenAIConfig: &openaiCfg})

	assistRole := allowAssistAccess(t, s)
	authPack := s.authPack(t, "foo", assistRole.GetName())

	// Make WS client and start the conversation
	ws, err := s.makeAssistant(t, authPack, "", "ssh-explain")
	require.NoError(t, err)
	t.Cleanup(func() {
		err := ws.Close()
		require.NoError(t, err)
	})

	err = ws.WriteMessage(websocket.TextMessage, []byte(`{"input:" "listen tcp 0.0.0.0:5432: bind: address already in use"}`))
	require.NoError(t, err)

	// verify responses
	assertResponse(ws)
}

func Test_generateAssistantTitle(t *testing.T) {
	// Test setup
	t.Parallel()
	ctx := context.Background()

	responses := []string{"This is the message summary.", "troubleshooting"}
	server := httptest.NewServer(aitest.GetTestHandlerFn(t, responses))
	t.Cleanup(server.Close)

	openaiCfg := openai.DefaultConfig("test-token")
	openaiCfg.BaseURL = server.URL
	s := newWebSuiteWithConfig(t, webSuiteConfig{
		ClusterFeatures: &authproto.Features{
			Cloud: true,
		},
		OpenAIConfig: &openaiCfg,
	})

	assistRole := allowAssistAccess(t, s)
	assistRole, err := s.server.Auth().UpsertRole(s.ctx, assistRole)
	require.NoError(t, err)

	pack := s.authPack(t, "foo", assistRole.GetName())

	// Real test: we craft a request asking for a summary
	endpoint := pack.clt.Endpoint("webapi", "assistant", "title", "summary")
	req := generateAssistantTitleRequest{Message: "This is a test user message asking Teleport assist to do something."}

	// Executing the request and validating the output is as expected
	resp, err := pack.clt.PostJSON(ctx, endpoint, &req)
	require.NoError(t, err)

	var info conversationInfo
	body, err := io.ReadAll(resp.Reader())
	require.NoError(t, err)
	err = json.Unmarshal(body, &info)
	require.NoError(t, err)
	require.NotEmpty(t, info.Title)
}

func allowAssistAccess(t *testing.T, s *WebSuite) types.Role {
	assistRole, err := types.NewRole("assist-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindAssistant, services.RW()),
			},
		},
	})
	require.NoError(t, err)
	assistRole, err = s.server.Auth().UpsertRole(s.ctx, assistRole)
	require.NoError(t, err)

	return assistRole
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
func (s *WebSuite) makeAssistant(_ *testing.T, pack *authPack, conversationID, action string) (*websocket.Conn, error) {
	if action == "" && conversationID == "" {
		return nil, trace.BadParameter("must specify either conversation_id or action")
	}

	u := url.URL{
		Host:   s.url().Host,
		Scheme: client.WSS,
		Path:   fmt.Sprintf("/v1/webapi/sites/%s/assistant", currentSiteShortcut),
	}

	q := u.Query()
	if conversationID != "" {
		q.Set("conversation_id", conversationID)
	}

	if action != "" {
		q.Set("action", action)
	}

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

func generateCommandResponse() string {
	return "```" + `json
	{
	    "action": "Command Generation",
	    "action_input": "{\"command\":\"openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes -subj\"}"
	}
	` + "```"
}

func commandSummaryResponse() string {
	return "The application has failed to connect to the database. The database is not running."
}
