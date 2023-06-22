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

	"github.com/gravitational/trace"
	"github.com/sashabaranov/go-openai"
	"github.com/tiktoken-go/tokenizer"

	"github.com/gravitational/teleport/lib/ai/model"
)

// Chat represents a conversation between a user and an assistant with context memory.
type Chat struct {
	client    *Client
	messages  []openai.ChatCompletionMessage
	tokenizer tokenizer.Codec
	agent     *model.Agent
}

// Insert inserts a message into the conversation. Returns the index of the message.
func (chat *Chat) Insert(role string, content string) int {
	chat.messages = append(chat.messages, openai.ChatCompletionMessage{
		Role:    role,
		Content: content,
	})

	return len(chat.messages) - 1
}

// GetMessages returns the messages in the conversation.
func (chat *Chat) GetMessages() []openai.ChatCompletionMessage {
	return chat.messages
}

// Complete completes the conversation with a message from the assistant based on the current context and user input.
// On success, it returns the message.
// Returned types:
// - message: one of the message types below
// - error: an error if one occurred
// Message types:
// - CompletionCommand: a command from the assistant
// - Message: a text message from the assistant
func (chat *Chat) Complete(ctx context.Context, userInput string, progressUpdates chan<- *model.AgentAction) (any, error) {
	// if the chat is empty, return the initial response we predefine instead of querying GPT-4
	if len(chat.messages) == 1 {
		return &model.Message{
			Content:    model.InitialAIResponse,
			TokensUsed: &model.TokensUsed{},
		}, nil
	}

	userMessage := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: userInput,
	}

	response, err := chat.agent.PlanAndExecute(ctx, chat.client.svc, chat.messages, userMessage, progressUpdates)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return response, nil
}

// Clear clears the conversation.
func (chat *Chat) Clear() {
	chat.messages = []openai.ChatCompletionMessage{}
}
