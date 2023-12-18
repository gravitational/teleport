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
	"sync/atomic"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/ai/testutils"
	assistlib "github.com/gravitational/teleport/lib/assist"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	testCommand = "echo txlxport | sed 's/x/e/g'"
	testUser    = "foo"
)

func TestExecuteCommand(t *testing.T) {
	t.Parallel()
	openAIMock := mockOpenAISummary(t)
	openAIConfig := openai.DefaultConfig("test-token")
	openAIConfig.BaseURL = openAIMock.URL
	s := newWebSuiteWithConfig(t, webSuiteConfig{
		disableDiskBasedRecording: true,
		OpenAIConfig:              &openAIConfig,
	})

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

	ws, _, err := s.makeCommand(t, s.authPack(t, testUser, assistRole.GetName()), uuid.New())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ws.Close()) })

	stream := NewWStream(context.Background(), ws, utils.NewLoggerForTests(), nil)

	require.NoError(t, waitForCommandOutput(stream, "teleport"))
}

func TestExecuteCommandHistory(t *testing.T) {
	t.Parallel()

	openAIMock := mockOpenAISummary(t)
	openAIConfig := openai.DefaultConfig("test-token")
	openAIConfig.BaseURL = openAIMock.URL
	s := newWebSuiteWithConfig(t, webSuiteConfig{
		disableDiskBasedRecording: true,
		OpenAIConfig:              &openAIConfig,
	})

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

	authPack := s.authPack(t, testUser, assistRole.GetName())

	ctx := context.Background()
	clt, err := s.server.NewClient(auth.TestUser(testUser))
	require.NoError(t, err)

	// Create conversation, otherwise the command execution will not be saved
	conversation, err := clt.CreateAssistantConversation(context.Background(), &assist.CreateAssistantConversationRequest{
		Username:    testUser,
		CreatedTime: timestamppb.Now(),
	})
	require.NoError(t, err)

	require.NotEmpty(t, conversation.GetId())

	conversationID, err := uuid.Parse(conversation.GetId())
	require.NoError(t, err)

	ws, _, err := s.makeCommand(t, authPack, conversationID)
	require.NoError(t, err)

	stream := NewWStream(ctx, ws, utils.NewLoggerForTests(), nil)

	// When command executes
	require.NoError(t, waitForCommandOutput(stream, "teleport"))

	// Close the stream if not already closed
	_ = stream.Close()

	// Then command execution history is saved
	var messages *assist.GetAssistantMessagesResponse
	// Command execution history is saved in asynchronously, so we need to wait for it.
	require.Eventually(t, func() bool {
		messages, err = clt.GetAssistantMessages(ctx, &assist.GetAssistantMessagesRequest{
			ConversationId: conversationID.String(),
			Username:       testUser,
		})
		require.NoError(t, err)

		return len(messagesByType(messages.GetMessages())[assistlib.MessageKindCommandResult]) == 1
	}, 5*time.Second, 100*time.Millisecond)

	// Assert the returned message
	resultMessages, ok := messagesByType(messages.GetMessages())[assistlib.MessageKindCommandResult]
	require.True(t, ok, "Message must be of type COMMAND_RESULT")
	msg := resultMessages[0]
	require.NotZero(t, msg.CreatedTime)

	var result commandExecResult
	err = json.Unmarshal([]byte(msg.GetPayload()), &result)
	require.NoError(t, err)

	require.NotEmpty(t, result.ExecutionID)
	require.NotEmpty(t, result.SessionID)
	require.Equal(t, "node", result.NodeName)
	require.Equal(t, "node", result.NodeID)
}

func TestExecuteCommandSummary(t *testing.T) {
	t.Parallel()

	openAIMock := mockOpenAISummary(t)
	openAIConfig := openai.DefaultConfig("test-token")
	openAIConfig.BaseURL = openAIMock.URL
	s := newWebSuiteWithConfig(t, webSuiteConfig{
		disableDiskBasedRecording: true,
		OpenAIConfig:              &openAIConfig,
	})

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

	authPack := s.authPack(t, testUser, assistRole.GetName())

	ctx := context.Background()
	clt, err := s.server.NewClient(auth.TestUser(testUser))
	require.NoError(t, err)

	// Create conversation, otherwise the command execution will not be saved
	conversation, err := clt.CreateAssistantConversation(context.Background(), &assist.CreateAssistantConversationRequest{
		Username:    testUser,
		CreatedTime: timestamppb.Now(),
	})
	require.NoError(t, err)

	require.NotEmpty(t, conversation.GetId())

	conversationID, err := uuid.Parse(conversation.GetId())
	require.NoError(t, err)

	ws, _, err := s.makeCommand(t, authPack, conversationID)
	require.NoError(t, err)

	// For simplicity, use simple WS to io.Reader adapter
	stream := &wsReader{conn: ws}

	// Wait for command execution to complete
	require.NoError(t, waitForCommandOutput(stream, "teleport"))

	dec := json.NewDecoder(stream)

	// Consume the close message
	var sessionMetadata sessionEndEvent
	err = dec.Decode(&sessionMetadata)
	require.NoError(t, err)
	require.Equal(t, "node", sessionMetadata.NodeID)

	// Consume the summary message
	var env outEnvelope
	err = dec.Decode(&env)
	require.NoError(t, err)
	require.Equal(t, envelopeTypeSummary, env.Type)
	require.NotEmpty(t, env.Payload)

	// Wait for the command execution history to be saved
	var messages *assist.GetAssistantMessagesResponse
	// Command execution history is saved in asynchronously, so we need to wait for it.
	require.Eventually(t, func() bool {
		messages, err = clt.GetAssistantMessages(ctx, &assist.GetAssistantMessagesRequest{
			ConversationId: conversationID.String(),
			Username:       testUser,
		})
		assert.NoError(t, err)

		return len(messagesByType(messages.GetMessages())[assistlib.MessageKindCommandResultSummary]) == 1
	}, 5*time.Second, 100*time.Millisecond)

	// Check the returned summary message
	summaryMessages, ok := messagesByType(messages.GetMessages())[assistlib.MessageKindCommandResultSummary]
	require.True(t, ok, "At least one summary message is expected")
	msg := summaryMessages[0]
	require.NotZero(t, msg.CreatedTime)

	var result assistlib.CommandExecSummary
	err = json.Unmarshal([]byte(msg.GetPayload()), &result)
	require.NoError(t, err)

	require.NotEmpty(t, result.ExecutionID)
	require.Equal(t, testCommand, result.Command)
	require.NotEmpty(t, result.Summary)
}

func (s *WebSuite) makeCommand(t *testing.T, pack *authPack, conversationID uuid.UUID) (*websocket.Conn, *session.Session, error) {
	req := CommandRequest{
		Query:          fmt.Sprintf("name == \"%s\"", s.srvID),
		Login:          pack.login,
		ConversationID: conversationID.String(),
		ExecutionID:    uuid.New().String(),
		Command:        testCommand,
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

		var env outEnvelope
		dec := json.NewDecoder(stream)
		if err := dec.Decode(&env); err != nil {
			return trace.Wrap(err, "decoding envelope JSON from stream")
		}

		data := removeSpace(string(env.Payload))
		if strings.Contains(data, substr) {
			return nil
		}
	}
}

// Test_runCommands tests that runCommands runs the given command on all hosts.
// The commands should run in parallel, but we don't have a deterministic way to
// test that (sleep with checking the execution time in not deterministic).
func Test_runCommands(t *testing.T) {
	const numWorkers = 30
	counter := atomic.Int32{}

	runCmd := func(host *hostInfo) error {
		counter.Add(1)
		return nil
	}

	hosts := make([]hostInfo, 0, 100)
	for i := 0; i < 100; i++ {
		hosts = append(hosts, hostInfo{
			hostName: fmt.Sprintf("localhost%d", i),
		})
	}

	logger := logrus.New()
	logger.Out = io.Discard

	runCommands(hosts, runCmd, numWorkers, logger)

	require.Equal(t, int32(100), counter.Load())
}

func mockOpenAISummary(t *testing.T) *httptest.Server {
	responses := []string{"This is the summary of the command."}
	server := httptest.NewServer(testutils.GetTestHandlerFn(t, responses))
	t.Cleanup(server.Close)
	return server
}

func messagesByType(messages []*assist.AssistantMessage) map[assistlib.MessageType][]*assist.AssistantMessage {
	byType := make(map[assistlib.MessageType][]*assist.AssistantMessage)
	for _, message := range messages {
		byType[assistlib.MessageType(message.GetType())] = append(byType[assistlib.MessageType(message.GetType())], message)
	}
	return byType
}

// wsReader implements io.Reader interface over websocket connection
type wsReader struct {
	conn *websocket.Conn
}

// Read reads data from websocket connection.
// The message should be in web.Envelope format and only the payload will be returned.
// It expects that the passed buffer is big enough to fit the whole message.
func (r *wsReader) Read(p []byte) (int, error) {
	_, data, err := r.conn.ReadMessage()
	if err != nil {
		return 0, trace.Wrap(err)
	}

	var envelope Envelope
	if err := proto.Unmarshal(data, &envelope); err != nil {
		return 0, trace.Errorf("Unable to parse message payload %v", err)
	}

	if len(envelope.Payload) > len(p) {
		return 0, trace.BadParameter("buffer too small")
	}

	return copy(p, envelope.Payload), nil
}
