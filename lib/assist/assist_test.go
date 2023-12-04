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

package assist

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/ai/model/tools"
	aitest "github.com/gravitational/teleport/lib/ai/testutils"
	"github.com/gravitational/teleport/lib/auth"
)

func TestChatComplete(t *testing.T) {
	t.Parallel()

	// Given an OpenAI server that returns a response for a chat completion request.
	responses := []string{
		generateCommandResponse(),
	}

	server := httptest.NewServer(aitest.GetTestHandlerFn(t, responses))
	t.Cleanup(server.Close)

	cfg := openai.DefaultConfig("secret-test-token")
	cfg.BaseURL = server.URL

	// And a chat client.
	ctx := context.Background()
	client, err := NewClient(ctx, &mockPluginGetter{}, &apiKeyMock{}, &cfg)
	require.NoError(t, err)

	// And a test auth server.
	authSrv, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	// And created conversation.
	toolContext := &tools.ToolContext{
		User: "bob",
	}
	conversationResp, err := authSrv.AuthServer.CreateAssistantConversation(ctx, &assist.CreateAssistantConversationRequest{
		Username:    toolContext.User,
		CreatedTime: timestamppb.Now(),
	})
	require.NoError(t, err)

	// When a chat is created.
	chat, err := client.NewChat(ctx, authSrv.AuthServer, toolContext, conversationResp.Id)
	require.NoError(t, err)

	t.Run("new conversation is new", func(t *testing.T) {
		// Then the chat is new.
		require.True(t, chat.IsNewConversation())
	})

	t.Run("the first message is the hey message", func(t *testing.T) {
		// Use called to make sure that the callback is called.
		called := false
		// The first message is the welcome message.
		_, err = chat.ProcessComplete(ctx, func(kind MessageType, payload []byte, createdTime time.Time) error {
			require.Equal(t, MessageKindAssistantMessage, kind)
			require.Contains(t, string(payload), "Hey, I'm Teleport")
			called = true
			return nil
		}, "")
		require.NoError(t, err)
		require.True(t, called)
	})

	t.Run("command should be returned in the response", func(t *testing.T) {
		called := false
		// The second message is the command response.
		_, err = chat.ProcessComplete(ctx, func(kind MessageType, payload []byte, createdTime time.Time) error {
			if kind == MessageKindProgressUpdate {
				return nil
			}
			require.Equal(t, MessageKindCommand, kind)
			require.Equal(t, `{"command":"df -h","nodes":["localhost"]}`, string(payload))
			called = true
			return nil
		}, "Show free disk space on localhost")
		require.NoError(t, err)
		require.True(t, called)
	})

	t.Run("check what messages are stored in the backend", func(t *testing.T) {
		// backend should have 3 messages: welcome message, user message, command response.
		messages, err := authSrv.AuthServer.GetAssistantMessages(ctx, &assist.GetAssistantMessagesRequest{
			Username:       toolContext.User,
			ConversationId: conversationResp.Id,
		})
		require.NoError(t, err)
		require.Len(t, messages.Messages, 3)

		require.Equal(t, string(MessageKindAssistantMessage), messages.Messages[0].Type)
		require.Equal(t, string(MessageKindUserMessage), messages.Messages[1].Type)
		require.Equal(t, string(MessageKindCommand), messages.Messages[2].Type)
	})
}

func TestClassifyMessage(t *testing.T) {
	// Given an OpenAI server that returns a response for a chat completion request.
	responses := []string{
		"troubleshooting",
		"Troubleshooting",
		"Troubleshooting.",
		"non-existent",
	}

	server := httptest.NewServer(aitest.GetTestHandlerFn(t, responses))
	t.Cleanup(server.Close)

	cfg := openai.DefaultConfig("secret-test-token")
	cfg.BaseURL = server.URL

	// And a chat client.
	ctx := context.Background()
	client, err := NewClient(ctx, &mockPluginGetter{}, &apiKeyMock{}, &cfg)
	require.NoError(t, err)

	t.Run("Valid class", func(t *testing.T) {
		class, err := client.ClassifyMessage(ctx, "whatever", MessageClasses)
		require.NoError(t, err)
		require.Equal(t, "troubleshooting", class)
	})

	t.Run("Valid class starting with upper-case", func(t *testing.T) {
		class, err := client.ClassifyMessage(ctx, "whatever", MessageClasses)
		require.NoError(t, err)
		require.Equal(t, "troubleshooting", class)
	})

	t.Run("Valid class starting with upper-case and ending with dot", func(t *testing.T) {
		class, err := client.ClassifyMessage(ctx, "whatever", MessageClasses)
		require.NoError(t, err)
		require.Equal(t, "troubleshooting", class)
	})

	t.Run("Model hallucinates", func(t *testing.T) {
		class, err := client.ClassifyMessage(ctx, "whatever", MessageClasses)
		require.Error(t, err)
		require.Empty(t, class)
	})
}

type apiKeyMock struct{}

// GetOpenAIAPIKey returns a mock API key.
func (m *apiKeyMock) GetOpenAIAPIKey() string {
	return "123"
}

type mockPluginGetter struct{}

func (m *mockPluginGetter) PluginsClient() pluginsv1.PluginServiceClient {
	return &mockPluginServiceClient{}
}

type mockPluginServiceClient struct {
	pluginsv1.PluginServiceClient
}

// GetPlugin always returns an error, so the assist fallbacks to the default config.
func (m *mockPluginServiceClient) GetPlugin(_ context.Context, _ *pluginsv1.GetPluginRequest, _ ...grpc.CallOption) (*types.PluginV1, error) {
	return nil, errors.New("not implemented")
}

// generateCommandResponse generates a response for the command "df -h" on the node "localhost"
func generateCommandResponse() string {
	return "```" + `json
	{
	    "action": "Command Execution",
	    "action_input": "{\"command\":\"df -h\",\"nodes\":[\"localhost\"],\"labels\":[]}"
	}
	` + "```"
}
