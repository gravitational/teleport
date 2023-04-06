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

// Complete completes the conversation with a message from the assistant based on the current context.
func (chat *Chat) Complete(ctx context.Context, maxTokens int) (Message, error) {
	request := openai.ChatCompletionRequest{
		Model:     openai.GPT4,
		MaxTokens: maxTokens,
		Messages:  chat.messages,
	}

	response, err := chat.client.api.CreateChatCompletion(ctx, request)
	if err != nil {
		return Message{}, trace.Wrap(err)
	}

	// there's always one choice but the API happens to model it as a list
	content := response.Choices[0].Message.Content
	chat.messages = append(chat.messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: content,
	})

	return Message{
		Role:    openai.ChatMessageRoleAssistant,
		Content: content,
		Idx:     len(chat.messages) - 1,
	}, nil

}
