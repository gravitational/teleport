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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tiktoken-go/tokenizer/codec"
)

func TestChat_PromptTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		messages []openai.ChatCompletionMessage
		want     int
		wantErr  bool
	}{
		{
			name:     "empty",
			messages: []openai.ChatCompletionMessage{},
			want:     3,
		},
		{
			name: "only system message",
			messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "Hello",
				},
			},
			want: 8,
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
			want: 16,
		},
		{
			name: "tokenize our prompt",
			messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: promptCharacter("Bob"),
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: "Show me free disk space on localhost node.",
				},
			},
			want: 187,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			chat := &Chat{
				messages:  tt.messages,
				tokenizer: codec.NewCl100kBase(),
			}
			usedTokens, err := chat.PromptTokens()
			require.NoError(t, err)
			require.Equal(t, tt.want, usedTokens)
		})
	}
}

func TestChat_Complete(t *testing.T) {
	t.Parallel()

	responses := [][]byte{
		generateTextResponse(),
		generateCommandResponse(),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		// Use assert as require doesn't work when called from a goroutine
		assert.GreaterOrEqual(t, len(responses), 1, "Unexpected request")
		dataBytes := responses[0]

		_, err := w.Write(dataBytes)
		assert.NoError(t, err, "Write error")

		responses = responses[1:]
	}))
	defer server.Close()

	cfg := openai.DefaultConfig("secret-test-token")
	cfg.BaseURL = server.URL + "/v1"
	client := NewClientFromConfig(cfg)

	chat := client.NewChat("Bob")

	t.Run("initial message", func(t *testing.T) {
		msg, err := chat.Complete(context.Background())
		require.NoError(t, err)

		expectedResp := &Message{Role: "assistant",
			Content: "Hey, I'm Teleport - a powerful tool that can assist you in managing your Teleport cluster via OpenAI GPT-4.",
			Idx:     0,
		}
		require.Equal(t, expectedResp, msg)
	})

	t.Run("text completion", func(t *testing.T) {
		chat.Insert(openai.ChatMessageRoleUser, "Show me free disk space")

		msg, err := chat.Complete(context.Background())
		require.NoError(t, err)

		require.IsType(t, &StreamingMessage{}, msg)
		streamingMessage := msg.(*StreamingMessage)
		require.Equal(t, openai.ChatMessageRoleAssistant, streamingMessage.Role)

		require.Equal(t, "Which ", <-streamingMessage.Chunks)
		require.Equal(t, "node do ", <-streamingMessage.Chunks)
		require.Equal(t, "you want ", <-streamingMessage.Chunks)
		require.Equal(t, "use?", <-streamingMessage.Chunks)
	})

	t.Run("command completion", func(t *testing.T) {
		chat.Insert(openai.ChatMessageRoleUser, "localhost")

		msg, err := chat.Complete(context.Background())
		require.NoError(t, err)

		require.IsType(t, &CompletionCommand{}, msg)
		command := msg.(*CompletionCommand)
		require.Equal(t, "df -h", command.Command)
		require.Len(t, command.Nodes, 1)
		require.Equal(t, "localhost", command.Nodes[0])
	})
}

// generateTextResponse generates a response for a text completion
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

// generateCommandResponse generates a response for the command "df -h" on the node "localhost"
func generateCommandResponse() []byte {
	dataBytes := []byte{}
	dataBytes = append(dataBytes, []byte("event: message\n")...)

	data := `{"id":"1","object":"completion","created":1598069254,"model":"gpt-4","choices":[{"index": 0, "delta":{"content": "{\"command\": \"df -h\",", "role": "assistant"}}]}`
	dataBytes = append(dataBytes, []byte("data: "+data+"\n\n")...)

	dataBytes = append(dataBytes, []byte("event: message\n")...)

	data = `{"id":"2","object":"completion","created":1598069254,"model":"gpt-4","choices":[{"index": 0, "delta":{"content": "\"nodes\": [\"localhost\"]}", "role": "assistant"}}]}`
	dataBytes = append(dataBytes, []byte("data: "+data+"\n\n")...)

	dataBytes = append(dataBytes, []byte("event: done\n")...)
	dataBytes = append(dataBytes, []byte("data: [DONE]\n\n")...)

	return dataBytes
}
