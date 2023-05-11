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

// Message represents a message within a live conversation.
// Indexed by ID for frontend ordering and future partial message streaming.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Idx     int    `json:"idx"`
	// NumTokens is the number of completion tokens for the (non-streaming) message
	NumTokens int `json:"-"`
}

// Label represents a label returned by OpenAI's completion API.
type Label struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// CompletionCommand represents a command returned by OpenAI's completion API.
type CompletionCommand struct {
	Command string   `json:"command,omitempty"`
	Nodes   []string `json:"nodes,omitempty"`
	Labels  []Label  `json:"labels,omitempty"`
	// NumTokens is the number of completion tokens for the (non-streaming) message
	NumTokens int `json:"-"`
}

// StreamingMessage represents a message that is streamed from the assistant and will later be stored as a normal message in the conversation store.
type StreamingMessage struct {
	// Role describes the OpenAI role of the message, i.e its sender.
	Role string

	// Idx is a semi-unique ID assigned when loading a conversation so that the UI can group partial messages together.
	Idx int

	// Chunks is a channel of message chunks that are streamed from the assistant.
	Chunks <-chan string

	// Error is a channel which may receive one error if the assistant encounters an error while streaming.
	// Consumers should stop reading from all channels if they receive an error and abort.
	Error <-chan error
}
