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
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestNewRequest(t *testing.T) {
	apiKey := "random-api-key"
	baseAddr := "https://api.anthropic.com/v1"
	t.Setenv(apiKeyEnvVarName, apiKey)
	t.Setenv(addressEnvVarName, baseAddr)

	for name, tc := range map[string]struct {
		llm             *types.LLM
		request         func() *http.Request
		expectedError   require.ErrorAssertionFunc
		expectedRequest require.ValueAssertionFunc
		expectedInfo    require.ValueAssertionFunc
	}{
		"successful messages": {
			llm: &types.LLM{
				Provider: types.LLMProviderAnthropic,
			},
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/messages",
					strings.NewReader(`{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hello"}]}`),
				)
				return r
			},
			expectedError: require.NoError,
			expectedRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				req, _ := i1.(*http.Request)
				require.Equal(tt, "/v1/messages", req.URL.Path)
				require.Equal(tt, "api.anthropic.com", req.URL.Host)
				require.Equal(tt, apiKey, req.Header.Get("x-api-key"))
				require.NotEmpty(tt, req.Header.Get("anthropic-version"))
				require.NotEmpty(tt, req.Header.Get("content-type"))
			},
			expectedInfo: func(tt require.TestingT, i1 any, i2 ...any) {
				info, _ := i1.(*RequestInfo)
				require.Equal(tt, "claude-sonnet-4-20250514", info.RequestedModel())
				require.Equal(tt, "claude-sonnet-4-20250514", info.ProviderModel())
			},
		},
		"includes single /v1 prefix": {
			llm: &types.LLM{
				Provider: types.LLMProviderAnthropic,
			},
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/v1/messages",
					strings.NewReader(`{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hello"}]}`),
				)
				return r
			},
			expectedError: require.NoError,
			expectedRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				req, _ := i1.(*http.Request)
				require.Equal(tt, "/v1/messages", req.URL.Path)
			},
			expectedInfo: require.NotNil,
		},
		"convert model name": {
			llm: &types.LLM{
				Provider: types.LLMProviderAnthropic,
				Models: []*types.LLM_Model{
					{ProviderName: "claude-opus-4-8", Name: "claude-sonnet-4-20250514"},
				},
			},
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/messages",
					strings.NewReader(`{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hello"}]}`),
				)
				return r
			},
			expectedError: require.NoError,
			expectedRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				req, _ := i1.(*http.Request)
				body, err := io.ReadAll(req.Body)
				require.NoError(tt, err)
				require.Contains(tt, string(body), "claude-opus-4-8")
			},
			expectedInfo: func(tt require.TestingT, i1 any, i2 ...any) {
				info, _ := i1.(*RequestInfo)
				require.Equal(tt, "claude-sonnet-4-20250514", info.RequestedModel())
				require.Equal(tt, "claude-opus-4-8", info.ProviderModel())
			},
		},
		"fallback model name": {
			llm: &types.LLM{
				Provider: types.LLMProviderAnthropic,
				Models: []*types.LLM_Model{
					{ProviderName: "claude-opus-4-8", Name: "claude-sonnet-4-20250514"},
				},
				FallbackModel: "claude-sonnet-4-20250514",
			},
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/messages",
					strings.NewReader(`{"model":"claude-haiku-4-20250514","messages":[{"role":"user","content":"Hello"}]}`),
				)
				return r
			},
			expectedError: require.NoError,
			expectedRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				req, _ := i1.(*http.Request)
				body, err := io.ReadAll(req.Body)
				require.NoError(tt, err)
				require.Contains(tt, string(body), "claude-opus-4-8")
			},
			expectedInfo: func(tt require.TestingT, i1 any, i2 ...any) {
				info, _ := i1.(*RequestInfo)
				require.Equal(tt, "claude-haiku-4-20250514", info.RequestedModel())
				require.Equal(tt, "claude-opus-4-8", info.ProviderModel())
			},
		},
		"unsupported model name": {
			llm: &types.LLM{
				Provider: types.LLMProviderAnthropic,
				Models: []*types.LLM_Model{
					{ProviderName: "claude-opus-4-8", Name: "claude-sonnet-4-20250514"},
				},
			},
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/messages",
					strings.NewReader(`{"model":"claude-haiku-4-20250514","messages":[{"role":"user","content":"Hello"}]}`),
				)
				return r
			},
			expectedError:   require.Error,
			expectedRequest: require.Nil,
			expectedInfo: func(tt require.TestingT, i1 any, i2 ...any) {
				info, _ := i1.(*RequestInfo)
				require.Equal(tt, "claude-haiku-4-20250514", info.RequestedModel())
				require.Empty(tt, info.ProviderModel())
			},
		},
		"exceeds max tokens": {
			llm: &types.LLM{
				Provider: types.LLMProviderAnthropic,
			},
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/messages",
					strings.NewReader(`{"model":"claude-sonnet-4-20250514", "max_tokens": 99999,"messages":[{"role":"user","content":"Hello"}]}`),
				)
				return r
			},
			expectedError:   require.Error,
			expectedRequest: require.Nil,
			expectedInfo: func(tt require.TestingT, i1 any, i2 ...any) {
				info, _ := i1.(*RequestInfo)
				require.Equal(tt, "claude-sonnet-4-20250514", info.RequestedModel())
				require.Equal(tt, "claude-sonnet-4-20250514", info.ProviderModel())
			},
		},
		"unsupported endpoint": {
			llm: &types.LLM{
				Provider: types.LLMProviderAnthropic,
			},
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/complete",
					strings.NewReader(`{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hello"}]}`),
				)
				return r
			},
			expectedError:   require.Error,
			expectedRequest: require.Nil,
			expectedInfo:    require.NotNil,
		},
		"unsupported method": {
			llm: &types.LLM{
				Provider: types.LLMProviderAnthropic,
			},
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodGet,
					"/messages",
					nil,
				)
				return r
			},
			expectedError:   require.Error,
			expectedRequest: require.Nil,
			expectedInfo:    require.NotNil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			req, info, err := NewRequest(tc.llm, tc.request())
			tc.expectedError(t, err)
			tc.expectedRequest(t, req)
			tc.expectedInfo(t, info)
		})
	}
}
