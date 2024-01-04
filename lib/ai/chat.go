/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package ai

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/sashabaranov/go-openai"

	"github.com/gravitational/teleport/lib/ai/model"
	"github.com/gravitational/teleport/lib/ai/model/output"
	"github.com/gravitational/teleport/lib/ai/tokens"
)

// Chat represents a conversation between a user and an assistant with context memory.
type Chat struct {
	client   *Client
	messages []openai.ChatCompletionMessage
	agent    *model.Agent
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
// - AccessRequest: an access request suggestion from the assistant
func (chat *Chat) Complete(ctx context.Context, userInput string, progressUpdates func(*model.AgentAction)) (any, *tokens.TokenCount, error) {
	// if the chat is empty, return the initial response we predefine instead of querying GPT-4
	if len(chat.messages) == 1 {
		return &output.Message{
			Content: model.InitialAIResponse,
		}, tokens.NewTokenCount(), nil
	}

	return chat.Reply(ctx, userInput, progressUpdates)
}

// Reply replies to the user input with a message from the assistant based on the current context.
func (chat *Chat) Reply(ctx context.Context, userInput string, progressUpdates func(*model.AgentAction)) (any, *tokens.TokenCount, error) {
	userMessage := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: userInput,
	}

	response, tokenCount, err := chat.agent.PlanAndExecute(ctx, chat.client.svc, chat.messages, userMessage, progressUpdates)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return response, tokenCount, nil
}

// Clear clears the conversation.
func (chat *Chat) Clear() {
	chat.messages = []openai.ChatCompletionMessage{}
}
