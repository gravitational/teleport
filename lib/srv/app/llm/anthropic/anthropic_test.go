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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	llmrequest "github.com/gravitational/teleport/lib/srv/app/llm/request"
)

func TestNewRequest(t *testing.T) {
	apiKey := "random-api-key"

	signAWSRequest := func(_ context.Context, _ types.Application, req *http.Request, reqBody []byte) error {
		hash := sha256.New()
		hash.Write(reqBody)
		req.Header.Set("X-Amz-Content-Sha256", hex.EncodeToString(hash.Sum(nil)))
		req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=test")
		return nil
	}

	for name, tc := range map[string]struct {
		llm             types.Application
		request         func() *http.Request
		signAWSRequest  func(context.Context, types.Application, *http.Request, []byte) error
		expectedError   require.ErrorAssertionFunc
		expectedRequest require.ValueAssertionFunc
		expectedInfo    require.ValueAssertionFunc
	}{
		"successful messages": {
			llm: newApp(t, &types.LLM{
				Format:   types.LLMFormatAnthropic,
				Provider: types.LLMProviderAnthropic,
			}, nil /* appAWS */),
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
			llm: newApp(t, &types.LLM{
				Format:   types.LLMFormatAnthropic,
				Provider: types.LLMProviderAnthropic,
			}, nil /* appAWS */),
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
			llm: newApp(t, &types.LLM{
				Format:   types.LLMFormatAnthropic,
				Provider: types.LLMProviderAnthropic,
				Models: []*types.LLM_Model{
					{ProviderName: "claude-opus-4-8", Name: "claude-sonnet-4-20250514"},
				},
			}, nil /* appAWS */),
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
			llm: newApp(t, &types.LLM{
				Format:   types.LLMFormatAnthropic,
				Provider: types.LLMProviderAnthropic,
				Models: []*types.LLM_Model{
					{ProviderName: "claude-opus-4-8", Name: "claude-sonnet-4-20250514"},
				},
				FallbackModel: "claude-sonnet-4-20250514",
			}, nil /* appAWS */),
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
			llm: newApp(t, &types.LLM{
				Format:   types.LLMFormatAnthropic,
				Provider: types.LLMProviderAnthropic,
				Models: []*types.LLM_Model{
					{ProviderName: "claude-opus-4-8", Name: "claude-sonnet-4-20250514"},
				},
			}, nil /* appAWS */),
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
			llm: newApp(t, &types.LLM{
				Format:   types.LLMFormatAnthropic,
				Provider: types.LLMProviderAnthropic,
			}, nil /* appAWS */),
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
		"request exceeds max size": {
			llm: newApp(t, &types.LLM{
				Format:   types.LLMFormatAnthropic,
				Provider: types.LLMProviderAnthropic,
			}, nil /* appAWS */),
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/messages",
					strings.NewReader(
						`{"model":"claude-sonnet-4-20250514", "max_tokens": 1024,"messages":[{"role":"user","content":"`+strings.Repeat("a", teleport.MaxHTTPRequestSize)+`"}]}`,
					),
				)
				return r
			},
			expectedError:   require.Error,
			expectedRequest: require.Nil,
			expectedInfo: func(tt require.TestingT, i1 any, i2 ...any) {
				info, _ := i1.(*RequestInfo)
				require.Empty(tt, info.RequestedModel())
				require.Empty(tt, info.ProviderModel())
			},
		},
		"unsupported endpoint": {
			llm: newApp(t, &types.LLM{
				Format:   types.LLMFormatAnthropic,
				Provider: types.LLMProviderAnthropic,
			}, nil /* appAWS */),
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
			llm: newApp(t, &types.LLM{
				Format:   types.LLMFormatAnthropic,
				Provider: types.LLMProviderAnthropic,
			}, nil /* appAWS */),
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
		"bedrock successful messages": {
			llm: newApp(t, &types.LLM{
				Format:   types.LLMFormatAnthropic,
				Provider: types.LLMProviderAWSBedrock,
				Models: []*types.LLM_Model{
					{ProviderName: "us.anthropic.claude-sonnet-4-20250514-v1:0", Name: "claude-sonnet-4-20250514"},
				},
			}, &types.AppAWS{
				Region: "us-west-2",
			}),
			signAWSRequest: signAWSRequest,
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
				require.Equal(tt, "bedrock-mantle.us-west-2.api.aws", req.URL.Host)
				require.Equal(tt, "/anthropic/v1/messages", req.URL.Path)
				require.NotEmpty(tt, req.Header.Get("anthropic-version"))
				require.NotEmpty(tt, req.Header.Get("content-type"))
				require.Empty(tt, req.Header.Get("x-api-key"))
				require.NotEmpty(tt, req.Header.Get("Authorization"))

				body, err := io.ReadAll(req.Body)
				require.NoError(tt, err)
				require.Contains(tt, string(body), "us.anthropic.claude-sonnet-4-20250514-v1:0")
				bodyHash := sha256.Sum256(body)
				require.Equal(tt, hex.EncodeToString(bodyHash[:]), req.Header.Get("X-Amz-Content-Sha256"))
			},
			expectedInfo: func(tt require.TestingT, i1 any, i2 ...any) {
				info, _ := i1.(*RequestInfo)
				require.Equal(tt, "claude-sonnet-4-20250514", info.RequestedModel())
				require.Equal(tt, "us.anthropic.claude-sonnet-4-20250514-v1:0", info.ProviderModel())
			},
		},
		"bedrock signer failure": {
			llm: newApp(t, &types.LLM{
				Format:   types.LLMFormatAnthropic,
				Provider: types.LLMProviderAWSBedrock,
			}, &types.AppAWS{
				Region: "us-west-2",
			}),
			signAWSRequest: func(ctx context.Context, a types.Application, r *http.Request, b []byte) error {
				return errors.New("signing failed")
			},
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/messages",
					strings.NewReader(`{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hello"}]}`),
				)
				return r
			},
			expectedError: func(tt require.TestingT, err error, i ...any) {
				require.Error(tt, err)
				// The signing failure cause must not reach clients.
				require.NotContains(tt, err.Error(), "signing failed")
			},
			expectedRequest: require.Nil,
			expectedInfo:    require.NotNil,
		},
		"bedrock missing signer": {
			llm: newApp(t, &types.LLM{
				Format:   types.LLMFormatAnthropic,
				Provider: types.LLMProviderAWSBedrock,
			}, &types.AppAWS{
				Region: "us-west-2",
			}),
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/messages",
					strings.NewReader(`{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hello"}]}`),
				)
				return r
			},
			expectedError:   require.Error,
			expectedRequest: require.Nil,
			expectedInfo:    require.NotNil,
		},
		"bedrock unsupported endpoint": {
			llm: newApp(t, &types.LLM{
				Format:   types.LLMFormatAnthropic,
				Provider: types.LLMProviderAWSBedrock,
			}, &types.AppAWS{
				Region: "us-west-2",
			}),
			signAWSRequest: signAWSRequest,
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
		"bedrock unsupported method": {
			llm: newApp(t, &types.LLM{
				Format:   types.LLMFormatAnthropic,
				Provider: types.LLMProviderAWSBedrock,
			}, &types.AppAWS{
				Region: "us-west-2",
			}),
			signAWSRequest: signAWSRequest,
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
			req, info, err := NewRequest(&llmrequest.Config{
				App:               tc.llm,
				DownstreamRequest: tc.request(),
				GetAPIKeyFunc: func() string {
					return apiKey
				},
				SignBedrockRequest: tc.signAWSRequest,
			})
			tc.expectedError(t, err)
			tc.expectedRequest(t, req)
			tc.expectedInfo(t, info)
		})
	}
}

func newApp(t *testing.T, llm *types.LLM, appAWS *types.AppAWS) types.Application {
	app, err := types.NewAppV3(types.Metadata{Name: "llm-app"}, types.AppSpecV3{
		LLM: llm,
		AWS: appAWS,
	})
	require.NoError(t, err)
	return app
}

// buildRequestBody returns a valid messages API request body whose total size
// is roughly fillerBytes.
func buildRequestBody(fillerBytes int) []byte {
	content := strings.Repeat("A", fillerBytes)
	return fmt.Appendf(nil,
		`{"model":"claude-sonnet-4-20250514","max_tokens":1024,"stream":false,"messages":[{"role":"user","content":%q}]}`,
		content,
	)
}

// BenchmarkNewRequest tracks the cost of the downstream request "parsing" and
// generation of the provider request, across different body sizes.
//
//	go test ./lib/srv/app/llm/anthropic/ -run '^$' -bench BenchmarkNewRequest -benchmem
func BenchmarkNewRequest(b *testing.B) {
	// Maps the requested model to a provider model so the conversion path
	// (the common case in production) is exercised.
	app, err := types.NewAppV3(types.Metadata{Name: "benchmark-app"}, types.AppSpecV3{
		LLM: &types.LLM{
			Format:   types.LLMFormatAnthropic,
			Provider: types.LLMProviderAnthropic,
			Models: []*types.LLM_Model{
				{ProviderName: "claude-opus-4-8", Name: "claude-sonnet-4-20250514"},
			},
		},
	})
	require.NoError(b, err)

	for _, bc := range []struct {
		name string
		body []byte
	}{
		{"small_chat", buildRequestBody(16)},
		{"medium_32KB", buildRequestBody(32 * 1024)},
		{"large_1MB", buildRequestBody(1024 * 1024)},
		{"xlarge_8MB", buildRequestBody(8 * 1024 * 1024)},
	} {
		b.Run(bc.name, func(b *testing.B) {
			// Reuse a single request and only reset the body each iteration.
			r, err := http.NewRequest(http.MethodPost, "/messages", nil)
			require.NoError(b, err)

			b.SetBytes(int64(len(bc.body)))
			b.ReportAllocs()

			for b.Loop() {
				r.Body = io.NopCloser(bytes.NewReader(bc.body))
				if _, _, err := NewRequest(&llmrequest.Config{
					App:               app,
					DownstreamRequest: r,
					GetAPIKeyFunc:     func() string { return "" },
				}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
