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

package testutils

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
)

// GetTestHandlerFn returns a handler function that can be used to OpenAI API used by
// the chat API. It takes a list of responses that will be returned in order.
func GetTestHandlerFn(t *testing.T, responses []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req := &openai.ChatCompletionRequest{}
		err := json.NewDecoder(r.Body).Decode(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}

		// Use assert as require doesn't work when called from a goroutine
		if !assert.GreaterOrEqual(t, len(responses), 1, "Unexpected request") {
			http.Error(w, "Unexpected request", http.StatusBadRequest)
			return
		}

		dataBytes := responses[0]

		resp := openai.ChatCompletionResponse{
			ID:      strconv.Itoa(int(time.Now().Unix())),
			Object:  "test-object",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []openai.ChatCompletionChoice{
				{
					Message: openai.ChatCompletionMessage{
						Role:    openai.ChatMessageRoleAssistant,
						Content: dataBytes,
						Name:    "",
					},
				},
			},
			Usage: openai.Usage{},
		}

		respBytes, err := json.Marshal(resp)
		assert.NoError(t, err, "Marshal error")

		_, err = w.Write(respBytes)
		assert.NoError(t, err, "Write error")

		responses = responses[1:]
	}
}
