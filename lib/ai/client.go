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
	"github.com/tiktoken-go/tokenizer/codec"

	"github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	"github.com/gravitational/teleport/lib/ai/model"
)

// Client is a client for OpenAI API.
type Client struct {
	svc *openai.Client
}

// NewClient creates a new client for OpenAI API.
func NewClient(apiURL string) *Client {
	return &Client{openai.NewClient(apiURL)}
}

// NewClientFromConfig creates a new client for OpenAI API from config.
func NewClientFromConfig(config openai.ClientConfig) *Client {
	return &Client{openai.NewClientWithConfig(config)}
}

// NewChat creates a new chat. The username is set in the conversation context,
// so that the AI can use it to personalize the conversation.
func (client *Client) NewChat(assistClient assist.AssistEmbeddingServiceClient, username string) *Chat {
	return &Chat{
		client: client,
		messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: model.PromptCharacter(username),
			},
		},
		// Initialize a tokenizer for prompt token accounting.
		// Cl100k is used by GPT-3 and GPT-4.
		tokenizer: codec.NewCl100kBase(),
		agent:     model.NewAgent(assistClient, username),
	}
}

// Summary creates a short summary for the given input.
func (client *Client) Summary(ctx context.Context, message string) (string, error) {
	resp, err := client.svc.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT4,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: model.PromptSummarizeTitle},
				{Role: openai.ChatMessageRoleUser, Content: message},
			},
		},
	)

	if err != nil {
		return "", trace.Wrap(err)
	}

	return resp.Choices[0].Message.Content, nil
}

// CommandSummary creates a command summary based on the command output.
// The message history is also passed to the model in order to keep context
// and extract relevant information from the output.
func (client *Client) CommandSummary(ctx context.Context, messages []openai.ChatCompletionMessage, output map[string][]byte) (string, error) {
	messages = append(messages, openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleUser, Content: model.ConversationCommandResult(output)})

	resp, err := client.svc.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:    openai.GPT4,
			Messages: messages,
		},
	)

	if err != nil {
		return "", trace.Wrap(err)
	}

	return resp.Choices[0].Message.Content, nil
}
