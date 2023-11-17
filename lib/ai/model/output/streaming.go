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

package output

import (
	"errors"
	"io"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sashabaranov/go-openai"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/ai/tokens"
)

// StreamToDeltas converts an openai.CompletionStream into a channel of strings.
// This channel can then be consumed manually to search for specific markers,
// or directly converted into a StreamingMessage with NewStreamingMessage.
func StreamToDeltas(stream *openai.ChatCompletionStream) chan string {
	deltas := make(chan string)
	go func() {
		defer close(deltas)

		for {
			response, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return
			} else if err != nil {
				log.Tracef("agent encountered an error while streaming: %v", err)
				return
			}

			delta := response.Choices[0].Delta.Content
			deltas <- delta
		}
	}()
	return deltas
}

// NewStreamingMessage takes a string channel and converts it to
// a StreamingMessage.
// If content was already streamed, it must be passed through the alreadyStreamed parameter.
// If the already streamed content contains a prefix that must be stripped
// (like a marker to identify the kind of response the model is providing),
// the prefix can be passed through the prefix parameter. It will be stripped
// but will still be reflected in the token count.
func NewStreamingMessage(deltas <-chan string, alreadyStreamed, prefix string) (*StreamingMessage, *tokens.AsynchronousTokenCounter, error) {
	parts := make(chan string)
	streamingTokenCounter, err := tokens.NewAsynchronousTokenCounter(alreadyStreamed)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	go func() {
		defer close(parts)

		parts <- strings.TrimPrefix(alreadyStreamed, prefix)
		for delta := range deltas {
			parts <- delta
			errCount := streamingTokenCounter.Add()
			if errCount != nil {
				log.WithError(errCount).Debug("Failed to add streamed completion text to the token counter")
			}
		}
	}()
	return &StreamingMessage{Parts: parts}, streamingTokenCounter, nil
}
