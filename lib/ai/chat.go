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
	"errors"
	"io"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sashabaranov/go-openai"
	"github.com/tiktoken-go/tokenizer"
)

const maxResponseTokens = 2000

// Chat represents a conversation between a user and an assistant with context memory.
type Chat struct {
	client    *Client
	messages  []openai.ChatCompletionMessage
	tokenizer tokenizer.Codec
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

// GetMessages returns the messages in the conversation.
func (chat *Chat) GetMessages() []openai.ChatCompletionMessage {
	return chat.messages
}

// PromptTokens uses the chat's tokenizer to calculate
// the total number of tokens in the prompt
//
// Ref: https://github.com/openai/openai-cookbook/blob/594fc6c952425810e9ea5bd1a275c8ca5f32e8f9/examples/How_to_count_tokens_with_tiktoken.ipynb
func (chat *Chat) PromptTokens() (int, error) {
	// perRequest is the number of tokens used up for each completion request
	const perRequest = 3
	// perRole is the number of tokens used to encode a message's role
	const perRole = 1
	// perMessage is the token "overhead" for each message
	const perMessage = 3

	sum := perRequest
	for _, m := range chat.messages {
		tokens, _, err := chat.tokenizer.Encode(m.Content)
		if err != nil {
			return 0, trace.Wrap(err)
		}
		sum += len(tokens)
		sum += perRole
		sum += perMessage
	}

	return sum, nil
}

// Complete completes the conversation with a message from the assistant based on the current context.
// On success, it returns the message.
// Returned types:
// - Message: the message from the assistant
// - error: an error if one occurred
// Message types:
// - CompletionCommand: a command from the assistant
// - StreamingMessage: a message that is streamed from the assistant
func (chat *Chat) Complete(ctx context.Context) (any, error) {
	var numTokens int

	// if the chat is empty, return the initial response we predefine instead of querying GPT-4
	if len(chat.messages) == 1 {
		return &Message{
			Role:    openai.ChatMessageRoleAssistant,
			Content: initialAIResponse,
			Idx:     len(chat.messages) - 1,
		}, nil
	}

	// if not, copy the current chat log to a new slice and append the suffix instruction
	messages := make([]openai.ChatCompletionMessage, len(chat.messages)+1)
	copy(messages, chat.messages)
	messages[len(messages)-1] = openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: promptExtractInstruction,
	}

	// create a streaming completion request, we do this to optimistically stream the response when
	// we don't believe it's a payload
	stream, err := chat.client.svc.CreateChatCompletionStream(
		ctx,
		openai.ChatCompletionRequest{
			Model:     openai.GPT4,
			Messages:  messages,
			MaxTokens: maxResponseTokens,
			Stream:    true,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var (
		response openai.ChatCompletionStreamResponse
		trimmed  string
	)
	for trimmed == "" {
		// fetch the first delta to check for a possible JSON payload
		response, err = stream.Recv()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		numTokens++

		trimmed = strings.TrimSpace(response.Choices[0].Delta.Content)
	}

	// if it looks like a JSON payload, let's wait for the entire response and try to parse it
	if strings.HasPrefix(trimmed, "{") {
		payload := strings.Builder{}
		payload.WriteString(response.Choices[0].Delta.Content)

		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, trace.Wrap(err)
			}
			numTokens++

			payload.WriteString(response.Choices[0].Delta.Content)
		}

		// if we can parse it, return the parsed payload, otherwise return a non-streaming message
		var c CompletionCommand
		err = json.Unmarshal([]byte(payload.String()), &c)
		switch err {
		case nil:
			c.NumTokens = numTokens
			return &c, nil
		default:
			return &Message{
				Role:      openai.ChatMessageRoleAssistant,
				Content:   payload.String(),
				Idx:       len(chat.messages) - 1,
				NumTokens: numTokens,
			}, nil
		}
	}

	// if it doesn't look like a JSON payload, return a streaming message to the caller
	chunks := make(chan string, 1)
	errCh := make(chan error)
	chunks <- response.Choices[0].Delta.Content
	go func() {
		defer close(chunks)

		for {
			response, err := stream.Recv()
			switch {
			case errors.Is(err, io.EOF):
				return
			case err != nil:
				select {
				case <-ctx.Done():
				case errCh <- trace.Wrap(err):
				}
				return
			}

			select {
			case chunks <- response.Choices[0].Delta.Content:
			case <-ctx.Done():
				return
			}
		}
	}()

	return &StreamingMessage{
		Role:   openai.ChatMessageRoleAssistant,
		Idx:    len(chat.messages) - 1,
		Chunks: chunks,
		Error:  errCh,
	}, nil
}
