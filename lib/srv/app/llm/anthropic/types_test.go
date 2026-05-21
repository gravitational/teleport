// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package anthropic

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

func TestParseMessagesRequest(t *testing.T) {
	for name, tc := range map[string]struct {
		input       string
		expectError require.ErrorAssertionFunc
		expectValue require.ValueAssertionFunc
	}{
		"valid json with fields": {
			input:       `{"model": "claude-opus-4-7", "stream": true, "max_tokens": 1024, "messages":[{"role":"user","content":"Hello"}]}`,
			expectError: require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				req, ok := i1.(messagesAPIRequest)
				require.True(tt, ok, "expect type to be %T but got %T", req, i1)
				require.Equal(tt, "claude-opus-4-7", req.Model, i2...)
				require.True(tt, req.Stream, i2...)
				require.Equal(tt, 1024, req.MaxTokens, i2...)
				require.Contains(tt, req.Raw, "messages")
			},
		},
		"valid json missing fields": {
			input:       `{"messages":[{"role":"user","content":"Hello"}]}`,
			expectError: require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				req, ok := i1.(messagesAPIRequest)
				require.True(tt, ok, "expect type to be %T but got %T", req, i1)
				require.Empty(tt, req.Model, i2...)
				require.False(tt, req.Stream, i2...)
				require.Empty(tt, req.MaxTokens, i2...)
				require.Contains(tt, req.Raw, "messages")
			},
		},
		"model duplicates": {
			input:       `{"model":"claude-sonnet-4-20250514","model":"claude-opus-4-7","model": "claude-haiku-4-5","MODEL":"claude-sonnet-4-7","messages":[{"role":"user","content":"Hello"}]}`,
			expectError: require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				req, ok := i1.(messagesAPIRequest)
				require.True(tt, ok, "expect type to be %T but got %T", req, i1)
				require.Equal(tt, "claude-haiku-4-5", req.Model, i2...)
				require.Contains(tt, req.Raw, "messages")
			},
		},
		"stream duplicates": {
			input:       `{"model":"claude-sonnet-4-20250514","stream":true,"STREAM":false,"messages":[{"role":"user","content":"Hello"}]}`,
			expectError: require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				req, ok := i1.(messagesAPIRequest)
				require.True(tt, ok, "expect type to be %T but got %T", req, i1)
				require.True(tt, req.Stream, i2...)
				require.Contains(tt, req.Raw, "messages")
			},
		},
		"max tokens duplicates": {
			input:       `{"model":"claude-sonnet-4-20250514","max_tokens":1024,"max_tokens":5555,"MAX_TOKENS":9999,"messages":[{"role":"user","content":"Hello"}]}`,
			expectError: require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				req, ok := i1.(messagesAPIRequest)
				require.True(tt, ok, "expect type to be %T but got %T", req, i1)
				require.Equal(tt, 5555, req.MaxTokens, i2...)
				require.Contains(tt, req.Raw, "messages")
			},
		},
		"stream invalid format": {
			input:       `{"model":"claude-sonnet-4-20250514","stream":"yes","messages":[{"role":"user","content":"Hello"}]}`,
			expectError: require.Error,
			expectValue: require.NotEmpty,
		},
		"invalid json": {
			input:       `{random}`,
			expectError: require.Error,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				req, ok := i1.(messagesAPIRequest)
				require.True(tt, ok, "expect type to be %T but got %T", req, i1)
				require.Empty(tt, req.Model, i2...)
				require.False(tt, req.Stream, i2...)
				require.Empty(tt, req.MaxTokens, i2...)
				require.Empty(tt, req.Raw, i2...)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			var r messagesAPIRequest
			tc.expectError(t, utils.FastUnmarshal([]byte(tc.input), &r))
			tc.expectValue(t, r)
		})
	}
}

func TestEncodeMessagesRequest(t *testing.T) {
	// For this test we use the value generated from a raw AnthropicAPI message,
	// so effectively we're also testing the parsing/decoding part as well.

	for name, tc := range map[string]struct {
		input         string
		modifyRequest func(*messagesAPIRequest)
		output        string
		expectError   require.ErrorAssertionFunc
		expectValue   require.ValueAssertionFunc
	}{
		"unmodified fields": {
			input:         `{"model": "claude-opus-4-7", "stream": true, "max_tokens": 1024, "messages":[{"role":"user","content":"Hello"}]}`,
			modifyRequest: func(r *messagesAPIRequest) {},
			expectError:   require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				resp, ok := i1.(string)
				require.True(tt, ok, "expect type to be %T but got %T", resp, i1)
				require.JSONEq(tt, `{"model": "claude-opus-4-7", "stream": true, "max_tokens": 1024, "messages":[{"role":"user","content":"Hello"}]}`, resp)
			},
		},
		"modified fields": {
			input: `{"model": "claude-opus-4-7", "stream": true, "max_tokens": 1024, "messages":[{"role":"user","content":"Hello"}]}`,
			modifyRequest: func(r *messagesAPIRequest) {
				r.Model = "claude-sonnet-4-7"
				r.Stream = false
				r.MaxTokens = 2048
			},
			expectError: require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				resp, ok := i1.(string)
				require.True(tt, ok, "expect type to be %T but got %T", resp, i1)
				require.JSONEq(tt, `{"model": "claude-sonnet-4-7", "stream": false, "max_tokens": 2048, "messages":[{"role":"user","content":"Hello"}]}`, resp)
			},
		},
		"duplicate fields": {
			input:         `{"model": "claude-opus-4-7", "model": "claude-sonnet-4-7", "stream": true, "stream": false, "max_tokens": 1024, "max_tokens": 5555, "messages":[{"role":"user","content":"Hello"}]}`,
			modifyRequest: func(r *messagesAPIRequest) {},
			expectError:   require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				resp, ok := i1.(string)
				require.True(tt, ok, "expect type to be %T but got %T", resp, i1)
				require.JSONEq(tt, `{"model": "claude-sonnet-4-7", "stream": false, "max_tokens": 5555, "messages":[{"role":"user","content":"Hello"}]}`, resp)
			},
		},
		"valid json missing fields": {
			input:         `{"messages":[{"role":"user","content":"Hello"}]}`,
			modifyRequest: func(r *messagesAPIRequest) {},
			expectError:   require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				resp, ok := i1.(string)
				require.True(tt, ok, "expect type to be %T but got %T", resp, i1)
				require.JSONEq(tt, `{"model": "", "stream": false, "max_tokens": 0, "messages":[{"role":"user","content":"Hello"}]}`, resp)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			var r messagesAPIRequest
			require.NoError(t, utils.FastUnmarshal([]byte(tc.input), &r))
			tc.modifyRequest(&r)

			res, err := utils.FastMarshal(r)
			tc.expectError(t, err)
			tc.expectValue(t, string(res))
		})
	}
}
