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
	"context"
	"encoding/json"

	"github.com/gravitational/trace"
	"github.com/sashabaranov/go-openai"

	assistantservice "github.com/gravitational/teleport/api/gen/proto/go/assistant/v1"
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

type Label struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type CompletionCommand struct {
	Command string   `json:"command,omitempty"`
	Nodes   []string `json:"nodes,omitempty"`
	Labels  []Label  `json:"labels,omitempty"`
}

func labelsToPbLabels(vals []*assistantservice.Label) []Label {
	ret := make([]Label, 0, len(vals))
	for _, v := range vals {
		ret = append(ret, Label{
			Key:   v.Key,
			Value: v.Value,
		})
	}

	return ret
}

// Summary create a short summary for the given input.
func (chat *Chat) Summary(ctx context.Context, message string) (string, error) {
	resp, err := chat.client.svc.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: promptSummarizeTitle},
				{Role: openai.ChatMessageRoleUser, Content: message},
			},
		},
	)

	if err != nil {
		return "", trace.Wrap(err)
	}

	return resp.Choices[0].Message.Content, nil
}

// Complete completes the conversation with a message from the assistant based on the current context.
func (chat *Chat) Complete(ctx context.Context, maxTokens int) (any, error) {
	if len(chat.messages) == 0 {
		return &Message{
			Role:    openai.ChatMessageRoleAssistant,
			Content: initialAIResponse,
			Idx:     len(chat.messages) - 1,
		}, nil
	}

	resp, err := chat.client.svc.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:    openai.GPT4,
			Messages: chat.messages,
		},
	)

	if err != nil {
		return nil, trace.Wrap(err)
	}

	respBody := resp.Choices[0].Message.Content
	var c CompletionCommand
	err = json.Unmarshal([]byte(respBody), &c)
	switch err {
	case nil:
		return &c, nil
	default:
		return &Message{
			Role:    openai.ChatMessageRoleAssistant,
			Content: respBody,
			Idx:     len(chat.messages) - 1,
		}, nil
	}
}
