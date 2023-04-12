/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gravitational/trace"
	openai "github.com/sashabaranov/go-openai"
)

// Message represents a message within a live conversation.
// Indexed by ID for frontend ordering and future partial message streaming.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Idx     int    `json:"idx"`
}

// Chat represents a conversation between a user and an assistant with context memory.
type Chat struct {
	client   *Client
	username string
	messages []openai.ChatCompletionMessage
}

// Insert inserts a message into the conversation. This is commonly in the
// form of a user's input but may also take the form of a system messages used for instructions.
func (chat *Chat) Insert(role string, content string) Message {
	chat.messages = append(chat.messages, openai.ChatCompletionMessage{
		Role:    role,
		Content: content,
	})

	return Message{
		Role:    role,
		Content: content,
		Idx:     len(chat.messages) - 1,
	}
}

type completionRequest struct {
	Username string                         `json:"username"`
	Messages []openai.ChatCompletionMessage `json:"messages"`
}

// Complete completes the conversation with a message from the assistant based on the current context.
func (chat *Chat) Complete(ctx context.Context, maxTokens int) (Message, error) {
	payload, err := json.Marshal(completionRequest{
		Username: chat.username,
		Messages: chat.messages,
	})
	if err != nil {
		return Message{}, trace.Wrap(err)
	}

	// TODO(joel): respond with configuration status of api url at features endpoint
	request, err := http.NewRequest("POST", chat.client.apiURL+"/assistant_query", bytes.NewBuffer(payload))
	if err != nil {
		return Message{}, trace.Wrap(err)
	}

	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return Message{}, trace.Wrap(err)
	}
	defer response.Body.Close()

	content, err := io.ReadAll(response.Body)
	if err != nil {
		return Message{}, trace.Wrap(err)
	}

	return Message{
		Role:    openai.ChatMessageRoleAssistant,
		Content: string(content),
		Idx:     len(chat.messages) - 1,
	}, nil

}
