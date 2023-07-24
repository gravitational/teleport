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

package model

import "github.com/gravitational/teleport/api/types"

// Ref: https://github.com/openai/openai-cookbook/blob/594fc6c952425810e9ea5bd1a275c8ca5f32e8f9/examples/How_to_count_tokens_with_tiktoken.ipynb
const (
	// perMessage is the token "overhead" for each message
	perMessage = 3

	// perRequest is the number of tokens used up for each completion request
	perRequest = 3

	// perRole is the number of tokens used to encode a message's role
	perRole = 1
)

// Message represents a new message within a live conversation.
type Message struct {
	Content string
}

// StreamingMessage represents a new message that is being streamed from the LLM.
type StreamingMessage struct {
	Parts <-chan string
}

// Label represents a label returned by OpenAI's completion API.
type Label struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// CompletionCommand represents a command suggestion returned by OpenAI's completion API.
type CompletionCommand struct {
	Command string   `json:"command,omitempty"`
	Nodes   []string `json:"nodes,omitempty"`
	Labels  []Label  `json:"labels,omitempty"`
}

// CompletionRequest represents an access request suggestion returned by OpenAI's completion API.
type AccessRequest struct {
	Roles              []string `json:"roles"`
	Reason             string   `json:"reason"`
	SuggestedReviewers []string `json:"suggested_reviewers"`
}

// AccessRequestsDisplay represents an indication to the frontend to display one or more access requests.
type AccessRequestsDisplay struct {
	AccessRequests []types.AccessRequest
}
