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
	cfg.BaseURL = server.URL + "/v1"

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
	const testUser = "bob"
	conversationResp, err := authSrv.AuthServer.CreateAssistantConversation(ctx, &assist.CreateAssistantConversationRequest{
		Username:    testUser,
		CreatedTime: timestamppb.Now(),
	})
	require.NoError(t, err)

	// When a chat is created.
	chat, err := client.NewChat(ctx, authSrv.AuthServer, nil, conversationResp.Id, testUser)
	require.NoError(t, err)

	t.Run("new conversation is new", func(t *testing.T) {
		// Then the chat is new.
		require.True(t, chat.IsNewConversation())
	})

	t.Run("the first message is the hey message", func(t *testing.T) {
		// The first message is the welcome message.
		_, err = chat.ProcessComplete(ctx, func(kind MessageType, payload []byte, createdTime time.Time) error {
			require.Equal(t, MessageKindAssistantMessage, kind)
			require.Contains(t, string(payload), "Hey, I'm Teleport")
			return nil
		}, "")
		require.NoError(t, err)
	})

	t.Run("command should be returned in the response", func(t *testing.T) {
		// The second message is the command response.
		_, err = chat.ProcessComplete(ctx, func(kind MessageType, payload []byte, createdTime time.Time) error {
			require.Equal(t, MessageKindCommand, kind)
			require.Equal(t, string(payload), `{"command":"df -h","nodes":["localhost"]}`)
			return nil
		}, "Show free disk space on localhost")
		require.NoError(t, err)
	})

	t.Run("check what messages are stored in the backend", func(t *testing.T) {
		// backend should have 3 messages: welcome message, user message, command response.
		messages, err := authSrv.AuthServer.GetAssistantMessages(ctx, &assist.GetAssistantMessagesRequest{
			Username:       testUser,
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
	cfg.BaseURL = server.URL + "/v1"

	// And a chat client.
	ctx := context.Background()
	client, err := NewClient(ctx, &mockPluginGetter{}, &apiKeyMock{}, &cfg)
	require.NoError(t, err)

	t.Run("Valid class", func(t *testing.T) {
		class, err := client.ClassifyMessage(ctx, "whatever", MessageClasses)
		require.NoError(t, err)
		require.Equal(t, class, "troubleshooting")
	})

	t.Run("Valid class starting with upper-case", func(t *testing.T) {
		class, err := client.ClassifyMessage(ctx, "whatever", MessageClasses)
		require.NoError(t, err)
		require.Equal(t, class, "troubleshooting")
	})

	t.Run("Valid class starting with upper-case and ending with dot", func(t *testing.T) {
		class, err := client.ClassifyMessage(ctx, "whatever", MessageClasses)
		require.NoError(t, err)
		require.Equal(t, class, "troubleshooting")
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
