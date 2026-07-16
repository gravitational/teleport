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

package openai

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

func TestParseResponsesRequest(t *testing.T) {
	for name, tc := range map[string]struct {
		input       string
		expectError require.ErrorAssertionFunc
		expectValue require.ValueAssertionFunc
	}{
		"valid json with fields": {
			input:       `{"model": "gpt-5", "stream": true, "max_output_tokens": 1024, "input":"Hello"}`,
			expectError: require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				req, ok := i1.(responsesAPIRequest)
				require.True(tt, ok, "expect type to be %T but got %T", req, i1)
				require.Equal(tt, "gpt-5", req.Model, i2...)
				require.True(tt, req.Stream, i2...)
				require.Contains(tt, req.raw, "input")
			},
		},
		"valid json missing fields": {
			input:       `{"input":"Hello"}`,
			expectError: require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				req, ok := i1.(responsesAPIRequest)
				require.True(tt, ok, "expect type to be %T but got %T", req, i1)
				require.Empty(tt, req.Model, i2...)
				require.False(tt, req.Stream, i2...)
				require.Contains(tt, req.raw, "input")
			},
		},
		"model duplicates": {
			input:       `{"model":"gpt-5","model":"gpt-5-mini","MODEL":"gpt-4o","max_output_tokens":1024,"stream":true,"input":"Hello"}`,
			expectError: require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				req, ok := i1.(responsesAPIRequest)
				require.True(tt, ok, "expect type to be %T but got %T", req, i1)
				require.Equal(tt, "gpt-5-mini", req.Model, i2...)
				require.Contains(tt, req.raw, "input")
			},
		},
		"model duplicates different casing": {
			input:       `{"model":"gpt-5","MODEL":"gpt-4o","max_output_tokens":1024,"stream":true,"input":"Hello"}`,
			expectError: require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				req, ok := i1.(responsesAPIRequest)
				require.True(tt, ok, "expect type to be %T but got %T", req, i1)
				require.Equal(tt, "gpt-5", req.Model, i2...)
				require.Contains(tt, req.raw, "input")
			},
		},
		"stream duplicates": {
			input:       `{"model":"gpt-5","stream":true,"stream":false,"input":"Hello"}`,
			expectError: require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				req, ok := i1.(responsesAPIRequest)
				require.True(tt, ok, "expect type to be %T but got %T", req, i1)
				require.False(tt, req.Stream, i2...)
				require.Contains(tt, req.raw, "input")
			},
		},
		"stream duplicates different casing": {
			input:       `{"model":"gpt-5","stream":true,"STREAM":false,"input":"Hello"}`,
			expectError: require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				req, ok := i1.(responsesAPIRequest)
				require.True(tt, ok, "expect type to be %T but got %T", req, i1)
				require.True(tt, req.Stream, i2...)
				require.Contains(tt, req.raw, "input")
			},
		},
		"max output tokens duplicates": {
			input:       `{"model":"gpt-5","max_output_tokens":1024,"max_output_tokens":5555,"stream":true,"input":"Hello"}`,
			expectError: require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				req, ok := i1.(responsesAPIRequest)
				require.True(tt, ok, "expect type to be %T but got %T", req, i1)
				require.Contains(tt, req.raw, "input")
			},
		},
		"max output tokens duplicates different casing": {
			input:       `{"model":"gpt-5","max_output_tokens":1024,"MAX_OUTPUT_TOKENS":9999,"stream":true,"input":"Hello"}`,
			expectError: require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				req, ok := i1.(responsesAPIRequest)
				require.True(tt, ok, "expect type to be %T but got %T", req, i1)
				require.Contains(tt, req.raw, "input")
			},
		},
		"fields in uppercase": {
			input:       `{"MODEL":"gpt-5","MAX_OUTPUT_TOKENS":1024,"STREAM":true,"input":"Hello"}`,
			expectError: require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				req, ok := i1.(responsesAPIRequest)
				require.True(tt, ok, "expect type to be %T but got %T", req, i1)
				require.Empty(tt, req.Model, i2...)
				require.False(tt, req.Stream, i2...)
				require.Contains(tt, req.raw, "input")
			},
		},
		"stream invalid format": {
			input:       `{"model":"gpt-5","stream":"yes","input":"Hello"}`,
			expectError: require.Error,
			expectValue: require.NotEmpty,
		},
		"duplicated other fields no error": {
			input:       `{"model": "gpt-5", "stream": true, "max_output_tokens": 1024, "input":"Hello", "reasoning": {"effort": "low"}, "reasoning": {"effort": "high"}}`,
			expectError: require.NoError,
			expectValue: require.NotEmpty,
		},
		"invalid json": {
			input:       `{random}`,
			expectError: require.Error,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				req, ok := i1.(responsesAPIRequest)
				require.True(tt, ok, "expect type to be %T but got %T", req, i1)
				require.Empty(tt, req.Model, i2...)
				require.False(tt, req.Stream, i2...)
				require.Empty(tt, req.raw, i2...)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			var r responsesAPIRequest
			tc.expectError(t, utils.FastUnmarshal([]byte(tc.input), &r))
			tc.expectValue(t, r)
		})
	}
}

func TestEncodeResponsesRequest(t *testing.T) {
	// For this test we use the value generated from a raw responses API request,
	// so effectively we're also testing the parsing/decoding part as well.

	for name, tc := range map[string]struct {
		input         string
		modifyRequest func(*responsesAPIRequest)
		output        string
		expectError   require.ErrorAssertionFunc
		expectValue   require.ValueAssertionFunc
	}{
		"unmodified fields": {
			input:         `{"model": "gpt-5", "stream": true, "max_output_tokens": 1024, "input":"Hello"}`,
			modifyRequest: func(r *responsesAPIRequest) {},
			expectError:   require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				resp, ok := i1.(string)
				require.True(tt, ok, "expect type to be %T but got %T", resp, i1)
				require.JSONEq(tt, `{"model": "gpt-5", "stream": true, "max_output_tokens": 1024, "input":"Hello"}`, resp)
			},
		},
		"modified fields": {
			input: `{"model": "gpt-5", "stream": true, "max_output_tokens": 1024, "input":"Hello"}`,
			modifyRequest: func(r *responsesAPIRequest) {
				r.Model = "gpt-5-mini"
				r.Stream = false
			},
			expectError: require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				resp, ok := i1.(string)
				require.True(tt, ok, "expect type to be %T but got %T", resp, i1)
				require.JSONEq(tt, `{"model": "gpt-5-mini", "max_output_tokens": 1024, "input":"Hello"}`, resp)
			},
		},
		"set model via SetModel": {
			input: `{"model": "gpt-5", "stream": true, "max_output_tokens": 1024, "input":"Hello"}`,
			modifyRequest: func(r *responsesAPIRequest) {
				r.SetModel("gpt-4o")
			},
			expectError: require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				resp, ok := i1.(string)
				require.True(tt, ok, "expect type to be %T but got %T", resp, i1)
				require.JSONEq(tt, `{"model": "gpt-4o", "stream": true, "max_output_tokens": 1024, "input":"Hello"}`, resp)
			},
		},
		"duplicate fields": {
			input:         `{"model": "gpt-5", "model": "gpt-5-mini", "stream": true, "stream": false, "max_output_tokens": 1024, "max_output_tokens": 5555, "input":"Hello"}`,
			modifyRequest: func(r *responsesAPIRequest) {},
			expectError:   require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				resp, ok := i1.(string)
				require.True(tt, ok, "expect type to be %T but got %T", resp, i1)
				require.JSONEq(tt, `{"model": "gpt-5-mini", "max_output_tokens": 5555, "input":"Hello"}`, resp)
			},
		},
		"duplicate fields different casing": {
			input:         `{"model": "gpt-5", "MODEL": "gpt-5-mini", "stream": true, "STREAM": false, "input":"Hello"}`,
			modifyRequest: func(r *responsesAPIRequest) {},
			expectError:   require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				resp, ok := i1.(string)
				require.True(tt, ok, "expect type to be %T but got %T", resp, i1)
				// Expect values from lowercase keys.
				require.JSONEq(tt, `{"model": "gpt-5", "stream": true, "input":"Hello"}`, resp)
			},
		},
		"full uppercase keys": {
			input:         `{"MODEL": "gpt-5", "STREAM": true, "input":"Hello"}`,
			modifyRequest: func(r *responsesAPIRequest) {},
			expectError:   require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				resp, ok := i1.(string)
				require.True(tt, ok, "expect type to be %T but got %T", resp, i1)
				require.JSONEq(tt, `{"model": "", "input":"Hello"}`, resp)
			},
		},
		"valid json missing fields": {
			input:         `{"input":"Hello"}`,
			modifyRequest: func(r *responsesAPIRequest) {},
			expectError:   require.NoError,
			expectValue: func(tt require.TestingT, i1 any, i2 ...any) {
				resp, ok := i1.(string)
				require.True(tt, ok, "expect type to be %T but got %T", resp, i1)
				require.JSONEq(tt, `{"model": "", "input":"Hello"}`, resp)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			var r responsesAPIRequest
			require.NoError(t, utils.FastUnmarshal([]byte(tc.input), &r))
			tc.modifyRequest(&r)

			res, err := utils.FastMarshal(r)
			tc.expectError(t, err)
			tc.expectValue(t, string(res))
		})
	}
}
