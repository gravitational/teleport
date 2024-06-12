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
		if r.Method != http.MethodPost || !(r.URL.Path == "/chat/completions") {
			http.Error(w, "Unexpected request", http.StatusBadRequest)
			return
		}

		switch r.Header.Get("Accept") {
		case "application/json; charset=utf-8", "application/json":
			responses = messageResponse(w, r, t, responses)
		case "text/event-stream":
			responses = streamResponse(w, t, responses)
		default:
			http.Error(w, "Unexpected request", http.StatusBadRequest)
		}
	}
}

func streamResponse(w http.ResponseWriter, t *testing.T, responses []string) []string {
	w.Header().Set("Content-Type", "text/event-stream")

	if !assert.NotEmpty(t, responses, "Unexpected request") {
		http.Error(w, "Unexpected request", http.StatusBadRequest)
		return responses
	}

	resp := &openai.ChatCompletionStreamResponse{
		ID:      strconv.Itoa(int(time.Now().Unix())),
		Object:  "completion",
		Created: time.Now().Unix(),
		Model:   openai.GPT4,
		Choices: []openai.ChatCompletionStreamChoice{
			{
				Index: 0,
				Delta: openai.ChatCompletionStreamChoiceDelta{
					Content: responses[0],
					Role:    openai.ChatMessageRoleAssistant,
				},
				FinishReason: "",
			},
		},
	}

	respBytes, err := json.Marshal(resp)
	assert.NoError(t, err, "Marshal error")

	_, err = w.Write([]byte("data: "))
	assert.NoError(t, err, "Write error")
	_, err = w.Write(respBytes)
	assert.NoError(t, err, "Write error")
	_, err = w.Write([]byte("\n\nevent: done\ndata: [DONE]\n\n"))
	assert.NoError(t, err, "Write error")

	return responses[1:]
}

func messageResponse(w http.ResponseWriter, r *http.Request, t *testing.T, responses []string) []string {
	w.Header().Set("Content-Type", "application/json")

	req := &openai.ChatCompletionRequest{}
	err := json.NewDecoder(r.Body).Decode(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	// Use assert as require doesn't work when called from a goroutine
	if !assert.NotEmpty(t, responses, "Unexpected request") {
		http.Error(w, "Unexpected request", http.StatusBadRequest)
		return responses
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

	return responses[1:]
}
