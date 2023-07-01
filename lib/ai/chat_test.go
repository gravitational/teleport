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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/ai/model"
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
			want: 721,
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
			want: 729,
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
			want: 932,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			responses := []string{
				generateCommandResponse(),
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")

				require.GreaterOrEqual(t, len(responses), 1, "Unexpected request")
				dataBytes := responses[0]
				_, err := w.Write([]byte(dataBytes))
				require.NoError(t, err, "Write error")

				responses = responses[1:]
			}))

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

	responses := [][]byte{
		[]byte(generateTextResponse()),
		[]byte(generateCommandResponse()),
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

	cfg := openai.DefaultConfig("secret-test-token")
	cfg.BaseURL = server.URL + "/v1"
	client := NewClientFromConfig(cfg)

	chat := client.NewChat(nil, "Bob")

	ctx := context.Background()
	_, err := chat.Complete(ctx, "Hello", func(aa *model.AgentAction) {})
	require.NoError(t, err)

	chat.Insert(openai.ChatMessageRoleUser, "Show me free disk space on localhost node.")

	t.Run("text completion", func(t *testing.T) {
		msg, err := chat.Complete(ctx, "Show me free disk space", func(aa *model.AgentAction) {})
		require.NoError(t, err)

		require.IsType(t, &model.StreamingMessage{}, msg)
		streamingMessage := msg.(*model.StreamingMessage)
		require.Equal(t, "Which ", <-streamingMessage.Parts)
		require.Equal(t, "node do ", <-streamingMessage.Parts)
		require.Equal(t, "you want ", <-streamingMessage.Parts)
		require.Equal(t, "use?", <-streamingMessage.Parts)
	})

	t.Run("command completion", func(t *testing.T) {
		msg, err := chat.Complete(ctx, "localhost", func(aa *model.AgentAction) {})
		require.NoError(t, err)

		require.IsType(t, &model.CompletionCommand{}, msg)
		command := msg.(*model.CompletionCommand)
		require.Equal(t, "df -h", command.Command)
		require.Len(t, command.Nodes, 1)
		require.Equal(t, "localhost", command.Nodes[0])
	})
}

// generateTextResponse generates a response for a text completion
func generateTextResponse() string {
	dataBytes := []byte{}
	dataBytes = append(dataBytes, []byte("event: message\n")...)

	data := `{"id":"1","object":"completion","created":1598069254,"model":"gpt-4","choices":[{"index": 0, "delta":{"content": "<FINAL RESPONSE>Which ", "role": "assistant"}}]}`
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

	return string(dataBytes)
}

// generateCommandResponse generates a response for the command "df -h" on the node "localhost"
func generateCommandResponse() string {
	dataBytes := []byte{}
	dataBytes = append(dataBytes, []byte("event: message\n")...)

	actionObj := model.PlanOutput{
		Action: "Command Execution",
		Action_input: struct {
			Command string   `json:"command"`
			Nodes   []string `json:"nodes"`
		}{"df -h", []string{"localhost"}},
	}
	actionJson, err := json.Marshal(actionObj)
	if err != nil {
		panic(err)
	}

	obj := struct {
		Content string `json:"content"`
		Role    string `json:"role"`
	}{
		Content: string(actionJson),
		Role:    "assistant",
	}
	json, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}

	data := fmt.Sprintf(`{"id":"1","object":"completion","created":1598069254,"model":"gpt-4","choices":[{"index": 0, "delta":%v}]}`, string(json))
	dataBytes = append(dataBytes, []byte("data: "+data+"\n\n")...)

	dataBytes = append(dataBytes, []byte("event: done\n")...)
	dataBytes = append(dataBytes, []byte("data: [DONE]\n\n")...)

	return string(dataBytes)
}
