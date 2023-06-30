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

package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/ai/model"
	aitest "github.com/gravitational/teleport/lib/ai/testutils"
)

func TestChat_PromptTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		messages []openai.ChatCompletionMessage
		want     int
	}{
		{
			name:     "empty",
			messages: []openai.ChatCompletionMessage{},
			want:     0,
		},
		{
			name: "only system message",
			messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "Hello",
				},
			},
			want: 743,
		},
		{
			name: "system and user messages",
			messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "Hello",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: "Hi LLM.",
				},
			},
			want: 751,
		},
		{
			name: "tokenize our prompt",
			messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: model.PromptCharacter("Bob"),
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: "Show me free disk space on localhost node.",
				},
			},
			want: 954,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			responses := []string{
				generateCommandResponse(t),
			}
			server := httptest.NewServer(aitest.GetTestHandlerFn(t, responses))
			t.Cleanup(server.Close)

			cfg := openai.DefaultConfig("secret-test-token")
			cfg.BaseURL = server.URL + "/v1"

			client := NewClientFromConfig(cfg)
			chat := client.NewChat(nil, "Bob")

			for _, message := range tt.messages {
				chat.Insert(message.Role, message.Content)
			}

			ctx := context.Background()
			message, err := chat.Complete(ctx, "", func(aa *model.AgentAction) {})
			require.NoError(t, err)
			msg, ok := message.(interface{ UsedTokens() *model.TokensUsed })
			require.True(t, ok)

			usedTokens := msg.UsedTokens().Completion + msg.UsedTokens().Prompt
			require.Equal(t, tt.want, usedTokens)
		})
	}
}

func TestChat_Complete(t *testing.T) {
	t.Parallel()

	responses := []string{
		generateTextResponse(t),
		generateCommandResponse(t),
	}
	server := httptest.NewServer(aitest.GetTestHandlerFn(t, responses))
	t.Cleanup(server.Close)

	cfg := openai.DefaultConfig("secret-test-token")
	cfg.BaseURL = server.URL + "/v1"
	client := NewClientFromConfig(cfg)

	chat := client.NewChat(nil, "Bob")

	t.Run("initial message", func(t *testing.T) {
		msgAny, err := chat.Complete(context.Background(), "Hello", func(aa *model.AgentAction) {})
		require.NoError(t, err)

		msg, ok := msgAny.(*model.Message)
		require.True(t, ok)

		expectedResp := &model.Message{
			Content: "Hey, I'm Teleport - a powerful tool that can assist you in managing your Teleport cluster via OpenAI GPT-4.",
		}
		require.Equal(t, expectedResp.Content, msg.Content)
		require.NotNil(t, msg.TokensUsed)
	})

	t.Run("text completion", func(t *testing.T) {
		chat.Insert(openai.ChatMessageRoleUser, "Show me free disk space")

		msg, err := chat.Complete(context.Background(), "", func(aa *model.AgentAction) {})
		require.NoError(t, err)

		require.IsType(t, &model.Message{}, msg)
		streamingMessage := msg.(*model.Message)

		const expectedResponse = "Which node do you want use?"

		require.Equal(t, expectedResponse, streamingMessage.Content)
	})

	t.Run("command completion", func(t *testing.T) {
		chat.Insert(openai.ChatMessageRoleUser, "localhost")

		msg, err := chat.Complete(context.Background(), "", func(aa *model.AgentAction) {})
		require.NoError(t, err)

		require.IsType(t, &model.CompletionCommand{}, msg)
		command := msg.(*model.CompletionCommand)
		require.Equal(t, "df -h", command.Command)
		require.Len(t, command.Nodes, 1)
		require.Equal(t, "localhost", command.Nodes[0])
	})
}

// generateTextResponse generates a response for a text completion
func generateTextResponse(t *testing.T) string {
	dataBytes := []byte{}
	dataBytes = append(dataBytes, []byte("event: message\n")...)
	obj := struct {
		content string
		role    string
	}{
		content: "<FINAL ANSWER>\nWhich node do you want use?",
		role:    "assistant",
	}
	json, err := json.Marshal(obj)
	require.NoError(t, err)
	data := fmt.Sprintf(`{"id":"1","object":"completion","created":1598069254,"model":"gpt-4","choices":[{"index": 0, "delta":%v}]}`, string(json))
	dataBytes = append(dataBytes, []byte("data: "+data+"\n\n")...)
	dataBytes = append(dataBytes, []byte("event: done\n")...)
	dataBytes = append(dataBytes, []byte("data: [DONE]\n\n")...)
	return string(dataBytes)
}

// generateCommandResponse generates a response for the command "df -h" on the node "localhost"
func generateCommandResponse(t *testing.T) string {
	dataBytes := []byte{}
	dataBytes = append(dataBytes, []byte("event: message\n")...)
	obj := struct {
		content string
		role    string
	}{
		content: "```" + `json
		{
			"action": "Command Execution",
			"action_input": "{\"command\":\"df -h\",\"nodes\":[\"localhost\"],\"labels\":[]}"
		}
		` + "```",
		role: "assistant",
	}
	json, err := json.Marshal(obj)
	require.NoError(t, err)
	data := fmt.Sprintf(`{"id":"1","object":"completion","created":1598069254,"model":"gpt-4","choices":[{"index": 0, "delta":%v}]}`, string(json))
	dataBytes = append(dataBytes, []byte("data: "+data+"\n\n")...)
	dataBytes = append(dataBytes, []byte("event: done\n")...)
	dataBytes = append(dataBytes, []byte("data: [DONE]\n\n")...)
	return string(dataBytes)
}
