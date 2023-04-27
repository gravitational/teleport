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
	"errors"
	"io"

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
		ctx,
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

type StreamingMessage struct {
	Role   string
	Idx    int
	Chunks <-chan string
	Error  <-chan error
}

// Complete completes the conversation with a message from the assistant based on the current context.
func (chat *Chat) Complete(ctx context.Context, maxTokens int) (any, error) {
	// if the chat is empty, return the initial response we predefine instead of querying GPT-4
	if len(chat.messages) == 0 {
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
			MaxTokens: maxTokens,
			Stream:    true,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// fetch the first delta to check for a possible JSON payload
	response, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// if it looks like a JSON payload, let's wait for the entire response and try to parse it
	if response.Choices[0].Delta.Content == "{" {
		payload := response.Choices[0].Delta.Content
	outer:
		for {
			response, err := stream.Recv()
			switch {
			case errors.Is(err, io.EOF):
				break outer
			case err != nil:
				return nil, trace.Wrap(err)
			}

			payload += response.Choices[0].Delta.Content
		}

		// if we can parse it, return the parsed payload, otherwise return a non-streaming message
		var c CompletionCommand
		err = json.Unmarshal([]byte(payload), &c)
		switch err {
		case nil:
			return &c, nil
		default:
			return &Message{
				Role:    openai.ChatMessageRoleAssistant,
				Content: payload,
				Idx:     len(chat.messages) - 1,
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
				errCh <- trace.Wrap(err)
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
