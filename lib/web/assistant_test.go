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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	aitest "github.com/gravitational/teleport/lib/ai/testutils"
	"github.com/gravitational/teleport/lib/assist"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"
)

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

// makeAssistant creates a new assistant websocket connection.
func (s *WebSuite) makeAssistant(_ *testing.T, pack *authPack, conversationID, action string) (*websocket.Conn, error) {
	if action == "" && conversationID == "" {
		return nil, trace.BadParameter("must specify either conversation_id or action")
	}

	u := url.URL{
		Host:   s.url().Host,
		Scheme: client.WSS,
		Path:   fmt.Sprintf("/v1/webapi/sites/%s/assistant/ws", currentSiteShortcut),
	}

	q := u.Query()
	if conversationID != "" {
		q.Set("conversation_id", conversationID)
	}

	if action != "" {
		q.Set("action", action)
	}

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

	if err := makeAuthReqOverWS(ws, pack.session.Token); err != nil {
		return nil, trace.Wrap(err)
	}

	err = resp.Body.Close()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ws, nil
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
