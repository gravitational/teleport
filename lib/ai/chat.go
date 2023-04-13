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

	assistantservice "github.com/gravitational/teleport/api/gen/proto/go/assistant/v1"
	"github.com/gravitational/trace"
	"github.com/sashabaranov/go-openai"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	messages []*assistantservice.ChatCompletionMessage
}

// Insert inserts a message into the conversation. This is commonly in the
// form of a user's input but may also take the form of a system messages used for instructions.
func (chat *Chat) Insert(role string, content string) Message {
	chat.messages = append(chat.messages, &assistantservice.ChatCompletionMessage{
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

type completionResponse struct {
	Kind    string   `json:"kind"`
	Content string   `json:"content,omitempty"`
	Command string   `json:"command,omitempty"`
	Nodes   []string `json:"nodes,omitempty"`
	Labels  []Label  `json:"labels,omitempty"`
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

func labelToPB(vals []*assistantservice.Label) []Label {
	ret := make([]Label, 0, len(vals))
	for _, v := range vals {
		ret = append(ret, Label{
			Key:   v.Key,
			Value: v.Value,
		})
	}

	return ret
}

// Complete completes the conversation with a message from the assistant based on the current context.
func (chat *Chat) Complete(ctx context.Context, maxTokens int) (*Message, *CompletionCommand, error) {
	var opts []grpc.DialOption

	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	//chat.client.apiURL
	conn, err := grpc.Dial("localhost:50052", opts...)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer conn.Close()

	assistantClient := assistantservice.NewAssistantServiceClient(conn)

	response, err := assistantClient.Complete(ctx, &assistantservice.CompleteRequest{
		Username: chat.username,
		Messages: chat.messages,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	switch {
	case response.Kind == "command":
		command := CompletionCommand{
			Command: response.Command,
			Nodes:   response.Nodes,
			Labels:  labelToPB(response.Labels),
		}

		return nil, &command, nil
	case response.Kind == "chat":
		message := Message{
			Role:    openai.ChatMessageRoleAssistant,
			Content: response.Content,
			Idx:     len(chat.messages) - 1,
		}

		return &message, nil, nil
	default:
		return nil, nil, trace.BadParameter("unknown completion kind: %s", response.Kind)
	}
}
