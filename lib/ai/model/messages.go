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

import (
	"github.com/gravitational/trace"
	"github.com/sashabaranov/go-openai"
	"github.com/tiktoken-go/tokenizer"
	"github.com/tiktoken-go/tokenizer/codec"
)

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
	*TokensUsed
	Content string
}

// Label represents a label returned by OpenAI's completion API.
type Label struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// CompletionCommand represents a command returned by OpenAI's completion API.
type CompletionCommand struct {
	*TokensUsed
	Command string   `json:"command,omitempty"`
	Nodes   []string `json:"nodes,omitempty"`
	Labels  []Label  `json:"labels,omitempty"`
}

// TokensUsed is used to track the number of tokens used during a single invocation of the agent.
type TokensUsed struct {
	tokenizer tokenizer.Codec

	// Prompt is the number of prompt-class tokens used.
	Prompt int

	// Completion is the number of completion-class tokens used.
	Completion int
}

// UsedTokens returns the number of tokens used during a single invocation of the agent.
// This method creates a convenient way to get TokensUsed from embedded structs.
func (t *TokensUsed) UsedTokens() *TokensUsed {
	return t
}

// newTokensUsed_Cl100kBase creates a new TokensUsed instance with a Cl100kBase tokenizer.
// This tokenizer is used by GPT-3 and GPT-4.
func newTokensUsed_Cl100kBase() *TokensUsed {
	return &TokensUsed{
		tokenizer:  codec.NewCl100kBase(),
		Prompt:     0,
		Completion: 0,
	}
}

// AddTokens updates TokensUsed with the tokens used for a single call to an LLM.
func (t *TokensUsed) AddTokens(prompt []openai.ChatCompletionMessage, completion string) error {
	for _, message := range prompt {
		promptTokens, _, err := t.tokenizer.Encode(message.Content)
		if err != nil {
			return trace.Wrap(err)
		}

		t.Prompt = t.Prompt + perMessage + perRole + len(promptTokens)
	}

	completionTokens, _, err := t.tokenizer.Encode(completion)
	if err != nil {
		return trace.Wrap(err)
	}

	t.Completion = t.Completion + perRequest + len(completionTokens)
	return err
}
