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
	"sync"

	"github.com/gravitational/trace"
	openai "github.com/sashabaranov/go-openai"
)

type Message struct {
	Role    string
	Content string
	Idx     int
}

type Chat struct {
	client   *Client
	messages []openai.ChatCompletionMessage
	mu       sync.Mutex
}

func (chat *Chat) Insert(role string, content string) Message {
	chat.mu.Lock()
	defer chat.mu.Unlock()

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

func (chat *Chat) Complete(ctx context.Context, maxTokens int) (Message, error) {
	chat.mu.Lock()
	request := openai.ChatCompletionRequest{
		Model:     openai.GPT3Dot5Turbo,
		MaxTokens: maxTokens,
		Messages:  chat.messages,
	}

	response, err := chat.client.api.CreateChatCompletion(ctx, request)
	if err != nil {
		return Message{}, trace.Wrap(err)
	}

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
