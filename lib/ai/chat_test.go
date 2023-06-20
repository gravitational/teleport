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
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/ai/model"
)

func TestChat_PromptTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		messages []openai.ChatCompletionMessage

		want    int
		wantErr bool
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
			want: 44,
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
			want: 44,
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
			want: 44,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			responses := []string{
				generateCommandResponse(),
			}
			server := httptest.NewServer(getTestHandlerFn(t, responses))

			t.Cleanup(server.Close)

			cfg := openai.DefaultConfig("secret-test-token")
			cfg.BaseURL = server.URL + "/v1"

			client := NewClientFromConfig(cfg)
			chat := client.NewChat("Bob")

			for _, message := range tt.messages {
				chat.Insert(message.Role, message.Content)
			}

			ctx := context.Background()
			message, err := chat.Complete(ctx, "")
			require.NoError(t, err)
			msg, ok := message.(interface{ UsedTokens() *model.TokensUsed })
			require.True(t, ok)
			require.Equal(t, tt.want, msg.UsedTokens().Completion)
		})
	}
}

func TestChat_Complete(t *testing.T) {
	t.Parallel()

	responses := []string{
		generateTextResponse(),
		generateCommandResponse(),
	}
	server := httptest.NewServer(getTestHandlerFn(t, responses))
	defer server.Close()

	cfg := openai.DefaultConfig("secret-test-token")
	cfg.BaseURL = server.URL + "/v1"
	client := NewClientFromConfig(cfg)

	chat := client.NewChat("Bob")

	t.Run("initial message", func(t *testing.T) {
		msgAny, err := chat.Complete(context.Background(), "Hello")
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

		msg, err := chat.Complete(context.Background(), "")
		require.NoError(t, err)

		require.IsType(t, &model.Message{}, msg)
		streamingMessage := msg.(*model.Message)

		const expectedResponse = "Which node do you want use?"

		require.Equal(t, expectedResponse, streamingMessage.Content)
	})

	t.Run("command completion", func(t *testing.T) {
		chat.Insert(openai.ChatMessageRoleUser, "localhost")

		msg, err := chat.Complete(context.Background(), "")
		require.NoError(t, err)

		require.IsType(t, &model.CompletionCommand{}, msg)
		command := msg.(*model.CompletionCommand)
		require.Equal(t, "df -h", command.Command)
		require.Len(t, command.Nodes, 1)
		require.Equal(t, "localhost", command.Nodes[0])
	})
}

func getTestHandlerFn(t *testing.T, responses []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req := &openai.ChatCompletionRequest{}
		err := json.NewDecoder(r.Body).Decode(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}

		// Use assert as require doesn't work when called from a goroutine
		if !assert.GreaterOrEqual(t, len(responses), 1, "Unexpected request") {
			http.Error(w, "Unexpected request", http.StatusBadRequest)
			return
		}

		dataBytes := responses[0]

		resp := openai.ChatCompletionResponse{
			ID:      strconv.Itoa(int(time.Now().Unix())),
			Object:  "test-object",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []openai.ChatCompletionChoice{
				{
					Message: openai.ChatCompletionMessage{
						Role:    openai.ChatMessageRoleAssistant,
						Content: dataBytes,
						Name:    "",
					},
				},
			},
			Usage: openai.Usage{},
		}

		respBytes, err := json.Marshal(resp)
		assert.NoError(t, err, "Marshal error")

		_, err = w.Write(respBytes)
		assert.NoError(t, err, "Write error")

		responses = responses[1:]
	}
}

// generateTextResponse generates a response for a text completion
func generateTextResponse() string {
	return "```" + `json
	{
	    "action": "Final Answer",
	    "action_input": "Which node do you want use?"
	}
	` + "```"
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
