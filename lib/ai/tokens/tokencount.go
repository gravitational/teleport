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

package tokens

import (
	"sync"

	"github.com/gravitational/trace"
	"github.com/sashabaranov/go-openai"
)

// TokenCount holds TokenCounters for both Prompt and Completion tokens.
// As the agent performs multiple calls to the model, each call creates its own
// prompt and completion TokenCounter.
//
// Prompt TokenCounters can be created before doing the call as we know the
// full prompt and can tokenize it. This is the PromptTokenCounter purpose.
//
// Completion TokenCounters can be created after receiving the model response.
// Depending on the response type, we might have the full result already or get
// a stream that will provide the completion result in the future. For the latter,
// the token count will be evaluated lazily and asynchronously.
// StaticTokenCounter count tokens synchronously, while
// AsynchronousTokenCounter supports the streaming use-cases.
type TokenCount struct {
	Prompt     TokenCounters
	Completion TokenCounters
}

// AddPromptCounter adds a TokenCounter to the Prompt list.
func (tc *TokenCount) AddPromptCounter(prompt TokenCounter) {
	if prompt != nil {
		tc.Prompt = append(tc.Prompt, prompt)
	}
}

// AddCompletionCounter adds a TokenCounter to the Completion list.
func (tc *TokenCount) AddCompletionCounter(completion TokenCounter) {
	if completion != nil {
		tc.Completion = append(tc.Completion, completion)
	}
}

// CountAll iterates over all counters and returns how many prompt and
// completion tokens were used. As completion token counting can require waiting
// for a response to be streamed, the caller should pass a context and use it to
// implement some kind of deadline to avoid hanging infinitely if something goes
// wrong (e.g. use `context.WithTimeout()`).
func (tc *TokenCount) CountAll() (int, int) {
	return tc.Prompt.CountAll(), tc.Completion.CountAll()
}

// NewTokenCount initializes a new TokenCount struct.
func NewTokenCount() *TokenCount {
	return &TokenCount{
		Prompt:     TokenCounters{},
		Completion: TokenCounters{},
	}
}

// TokenCounter is an interface for all token counters, regardless of the kind
// of token they count (prompt/completion) or the tokenizer used.
// TokenCount must be idempotent.
type TokenCounter interface {
	TokenCount() int
}

// TokenCounters is a list of TokenCounter and offers function to iterate over
// all counters and compute the total.
type TokenCounters []TokenCounter

// CountAll iterates over a list of TokenCounter and returns the sum of the
// results of all counters. As the counting process might be blocking/take some
// time, the caller should set a Deadline on the context.
func (tc TokenCounters) CountAll() int {
	var total int
	for _, counter := range tc {
		total += counter.TokenCount()
	}
	return total
}

// StaticTokenCounter is a token counter whose count has already been evaluated.
// This can be used to count prompt tokens (we already know the exact count),
// or to count how many tokens were used by an already finished completion
// request.
type StaticTokenCounter int

// TokenCount implements the TokenCounter interface.
func (tc *StaticTokenCounter) TokenCount() int {
	return int(*tc)
}

// NewPromptTokenCounter takes a list of openai.ChatCompletionMessage and
// computes how many tokens are used by sending those messages to the model.
func NewPromptTokenCounter(prompt []openai.ChatCompletionMessage) (*StaticTokenCounter, error) {
	var promptCount int
	for _, message := range prompt {
		promptTokens := countTokens(message.Content)

		promptCount = promptCount + perMessage + perRole + promptTokens
	}
	tc := StaticTokenCounter(promptCount)

	return &tc, nil
}

// NewSynchronousTokenCounter takes the completion request output and
// computes how many tokens were used by the model to generate this result.
func NewSynchronousTokenCounter(completion string) (*StaticTokenCounter, error) {
	completionTokens := countTokens(completion)
	completionCount := perRequest + completionTokens

	tc := StaticTokenCounter(completionCount)
	return &tc, nil
}

// AsynchronousTokenCounter counts completion tokens that are used by a
// streamed completion request. When creating a AsynchronousTokenCounter,
// the streaming might not be finished, and we can't evaluate how many tokens
// will be used. In this case, the streaming routine must add streamed
// completion result with the Add() method and call Finish() once the
// completion is finished. TokenCount() will hang until either Finish() is
// called or the context is Done.
type AsynchronousTokenCounter struct {
	count int

	// mutex protects all fields of the AsynchronousTokenCounter, it must be
	// acquired before any read or write operation.
	mutex sync.Mutex
	// finished tells if the count is finished or not.
	// TokenCount() finishes the count. Once the count is finished, Add() will
	// throw errors.
	finished bool
}

// TokenCount implements the TokenCounter interface.
// It returns how many tokens have been counted. It also marks the counter as
// finished. Once a counter is finished, tokens cannot be added anymore.
func (tc *AsynchronousTokenCounter) TokenCount() int {
	// If the count is already finished, we return the values
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	tc.finished = true
	return tc.count + perRequest
}

// Add a streamed token to the count.
func (tc *AsynchronousTokenCounter) Add() error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()

	if tc.finished {
		return trace.Errorf("Count is already finished, cannot add more content")
	}
	tc.count += 1
	return nil
}

// NewAsynchronousTokenCounter takes the partial completion request output
// and creates a token counter that can be already returned even if not all
// the content has been streamed yet. Streamed content can be added a posteriori
// with Add(). Once all the content is streamed, Finish() must be called.
func NewAsynchronousTokenCounter(completionStart string) (*AsynchronousTokenCounter, error) {
	completionTokens := countTokens(completionStart)

	return &AsynchronousTokenCounter{
		count:    completionTokens,
		mutex:    sync.Mutex{},
		finished: false,
	}, nil
}

// countTokens returns an estimated number of tokens in the text.
func countTokens(text string) int {
	// Rough estimations that each token is around 4 characters.
	return len(text) / 4
}
