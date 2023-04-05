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

package llmchain

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	openai "github.com/sashabaranov/go-openai"
)

type Client struct {
	api *openai.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		api: openai.NewClient(apiKey),
	}
}

func (client *Client) NewChain() *Chain {
	return &Chain{
		client: client,
	}
}

type Message struct {
	Role    string
	Content string
	Idx     int
}

type Chain struct {
	client   *Client
	messages []openai.ChatCompletionMessage
	mu       sync.Mutex
}

func (chain *Chain) Insert(role string, content string) Message {
	chain.mu.Lock()
	defer chain.mu.Unlock()

	chain.messages = append(chain.messages, openai.ChatCompletionMessage{
		Role:    role,
		Content: content,
	})

	return Message{
		Role:    role,
		Content: content,
		Idx:     len(chain.messages) - 1,
	}
}

func (chain *Chain) Complete(ctx context.Context, maxTokens int) (<-chan Message, error) {
	chain.mu.Lock()
	request := openai.ChatCompletionRequest{
		Model:     openai.GPT3Dot5Turbo,
		MaxTokens: maxTokens,
		Messages:  chain.messages,
		Stream:    true,
	}

	deltaCh := make(chan Message)
	stream, err := chain.client.api.CreateChatCompletionStream(ctx, request)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go func() {
		defer chain.mu.Unlock()
		defer close(deltaCh)
		var completion strings.Builder

	loop:
		for {
			response, err := stream.Recv()
			switch {
			case errors.Is(err, io.EOF):
				break loop
			case err != nil:
				return
			}

			delta := response.Choices[0].Delta.Content
			completion.WriteString(delta)
			deltaCh <- Message{
				Role:    openai.ChatMessageRoleAssistant,
				Content: delta,
				Idx:     len(chain.messages),
			}
		}

		chain.messages = append(chain.messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: completion.String(),
		})
	}()

	return deltaCh, nil
}
