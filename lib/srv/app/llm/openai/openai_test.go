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

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	llmrequest "github.com/gravitational/teleport/lib/srv/app/llm/request"
	"github.com/gravitational/teleport/lib/utils"
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
		app             types.Application
		request         func() *http.Request
		signAWSRequest  func(context.Context, types.Application, *http.Request, []byte) error
		expectedError   require.ErrorAssertionFunc
		expectedRequest require.ValueAssertionFunc
		expectedInfo    require.ValueAssertionFunc
	}{
		"successful responses": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderOpenAI,
			}, nil /* appAWS */),
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/responses",
					strings.NewReader(`{"model":"gpt-5","input":"Hello"}`),
				)
				return r
			},
			expectedError: require.NoError,
			expectedRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				req, _ := i1.(*http.Request)
				require.Equal(tt, "/v1/responses", req.URL.Path)
				require.Equal(tt, http.MethodPost, req.Method)
				require.Equal(tt, "Bearer "+apiKey, req.Header.Get("Authorization"))
				require.NotEmpty(tt, req.Header.Get("content-type"))
			},
			expectedInfo: func(tt require.TestingT, i1 any, i2 ...any) {
				info, _ := i1.(*RequestInfo)
				require.Equal(tt, "gpt-5", info.RequestedModel())
				require.Equal(tt, "gpt-5", info.ProviderModel())
			},
		},
		"includes single /v1 prefix": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderOpenAI,
			}, nil /* appAWS */),
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/v1/responses",
					strings.NewReader(`{"model":"gpt-5","input":"Hello"}`),
				)
				return r
			},
			expectedError: require.NoError,
			expectedRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				req, _ := i1.(*http.Request)
				require.Equal(tt, "/v1/responses", req.URL.Path)
			},
			expectedInfo: require.NotNil,
		},
		"convert model name": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderOpenAI,
				Models: []*types.LLM_Model{
					{ProviderName: "gpt-5", Name: "gpt-4o"},
				},
			}, nil /* appAWS */),
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/responses",
					strings.NewReader(`{"model":"gpt-4o","input":"Hello"}`),
				)
				return r
			},
			expectedError: require.NoError,
			expectedRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				req, _ := i1.(*http.Request)
				body, err := io.ReadAll(req.Body)
				require.NoError(tt, err)
				require.Contains(tt, string(body), "gpt-5")
			},
			expectedInfo: func(tt require.TestingT, i1 any, i2 ...any) {
				info, _ := i1.(*RequestInfo)
				require.Equal(tt, "gpt-4o", info.RequestedModel())
				require.Equal(tt, "gpt-5", info.ProviderModel())
			},
		},
		"fallback model name": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderOpenAI,
				Models: []*types.LLM_Model{
					{ProviderName: "gpt-5", Name: "gpt-4o"},
				},
				FallbackModel: "gpt-4o",
			}, nil /* appAWS */),
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/responses",
					strings.NewReader(`{"model":"gpt-4o-mini","input":"Hello"}`),
				)
				return r
			},
			expectedError: require.NoError,
			expectedRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				req, _ := i1.(*http.Request)
				body, err := io.ReadAll(req.Body)
				require.NoError(tt, err)
				require.Contains(tt, string(body), "gpt-5")
			},
			expectedInfo: func(tt require.TestingT, i1 any, i2 ...any) {
				info, _ := i1.(*RequestInfo)
				require.Equal(tt, "gpt-4o-mini", info.RequestedModel())
				require.Equal(tt, "gpt-5", info.ProviderModel())
			},
		},
		"unsupported model name": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderOpenAI,
				Models: []*types.LLM_Model{
					{ProviderName: "gpt-5", Name: "gpt-4o"},
				},
			}, nil /* appAWS */),
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/responses",
					strings.NewReader(`{"model":"gpt-4o-mini","input":"Hello"}`),
				)
				return r
			},
			expectedError:   require.Error,
			expectedRequest: require.Nil,
			expectedInfo: func(tt require.TestingT, i1 any, i2 ...any) {
				info, _ := i1.(*RequestInfo)
				require.Equal(tt, "gpt-4o-mini", info.RequestedModel())
				require.Empty(tt, info.ProviderModel())
			},
		},
		"request exceeds max size": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderOpenAI,
			}, nil /* appAWS */),
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/responses",
					strings.NewReader(
						`{"model":"gpt-5","input":"`+strings.Repeat("a", teleport.MaxHTTPRequestSize)+`"}`,
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
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderOpenAI,
			}, nil /* appAWS */),
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/embeddings",
					strings.NewReader(`{"model":"gpt-5","input":"Hello"}`),
				)
				return r
			},
			expectedError:   require.Error,
			expectedRequest: require.Nil,
			expectedInfo:    require.NotNil,
		},
		"unsupported method on responses": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderOpenAI,
			}, nil /* appAWS */),
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodGet,
					"/responses",
					nil,
				)
				return r
			},
			expectedError:   require.Error,
			expectedRequest: require.Nil,
			expectedInfo:    require.NotNil,
		},
		"websocket mode is not supported": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderOpenAI,
			}, nil /* appAWS */),
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/responses",
					strings.NewReader(`{"model":"gpt-5","input":"Hello"}`),
				)
				r.Header.Set("Connection", "upgrade")
				r.Header.Set("Upgrade", "websocket")
				return r
			},
			expectedError:   require.Error,
			expectedRequest: require.Nil,
			expectedInfo:    require.NotNil,
		},
		"responses background requests is not supported": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderOpenAI,
			}, nil /* appAWS */),
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/responses",
					strings.NewReader(`{"model":"gpt-5","input":"Hello", "background": true}`),
				)
				return r
			},
			expectedError:   require.Error,
			expectedRequest: require.Nil,
			expectedInfo:    require.NotNil,
		},
		"responses store requests is not supported": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderOpenAI,
			}, nil /* appAWS */),
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/responses",
					strings.NewReader(`{"model":"gpt-5","input":"Hello", "store": true}`),
				)
				return r
			},
			expectedError:   require.Error,
			expectedRequest: require.Nil,
			expectedInfo:    require.NotNil,
		},
		"null requests is rejected": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderOpenAI,
			}, nil /* appAWS */),
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/responses",
					strings.NewReader(`null`),
				)
				return r
			},
			expectedError:   require.Error,
			expectedRequest: require.Nil,
			expectedInfo:    require.NotNil,
		},
		"successful chat completions": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderOpenAI,
			}, nil /* appAWS */),
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/chat/completions",
					strings.NewReader(`{"model":"gpt-5","stream": true,"messages":[{"role":"user","content":"Hello"}]}`),
				)
				return r
			},
			expectedError: require.NoError,
			expectedRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				req, _ := i1.(*http.Request)
				require.Equal(tt, "/v1/chat/completions", req.URL.Path)
				require.Equal(tt, http.MethodPost, req.Method)
				require.Equal(tt, "Bearer "+apiKey, req.Header.Get("Authorization"))
				require.NotEmpty(tt, req.Header.Get("content-type"))
				// Usage reporting is enabled on chat completions requests.
				body, err := io.ReadAll(req.Body)
				require.NoError(tt, err)
				require.Contains(tt, string(body), `"include_usage":true`)
			},
			expectedInfo: func(tt require.TestingT, i1 any, i2 ...any) {
				info, _ := i1.(*RequestInfo)
				require.Equal(tt, "gpt-5", info.RequestedModel())
				require.Equal(tt, "gpt-5", info.ProviderModel())
			},
		},
		"chat completions store is not supported": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderOpenAI,
			}, nil /* appAWS */),
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/chat/completions",
					strings.NewReader(`{"model":"gpt-5","stream": true,"store":true,"messages":[{"role":"user","content":"Hello"}]}`),
				)
				return r
			},
			expectedError:   require.Error,
			expectedRequest: require.Nil,
			expectedInfo:    require.NotNil,
		},
		"unsupported method on chat completions": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderOpenAI,
			}, nil /* appAWS */),
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodGet,
					"/chat/completions",
					nil,
				)
				return r
			},
			expectedError:   require.Error,
			expectedRequest: require.Nil,
			expectedInfo:    require.NotNil,
		},
		"bedrock responses successful messages": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderAWSBedrock,
				Models: []*types.LLM_Model{
					{ProviderName: "openai.gpt-5.6-terra", Name: "gpt-5.6-terra"},
				},
			}, &types.AppAWS{
				Region: "us-east-2",
			}),
			signAWSRequest: signAWSRequest,
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/responses",
					strings.NewReader(`{"model":"gpt-5.6-terra","input":"Hello"}`),
				)
				return r
			},
			expectedError: require.NoError,
			expectedRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				req, _ := i1.(*http.Request)
				require.Equal(tt, "bedrock-mantle.us-east-2.api.aws", req.URL.Host)
				require.Equal(tt, "/openai/v1/responses", req.URL.Path)
				require.NotEmpty(tt, req.Header.Get("content-type"))
				require.NotEmpty(tt, req.Header.Get("Authorization"))

				body, err := io.ReadAll(req.Body)
				require.NoError(tt, err)
				require.Contains(tt, string(body), "openai.gpt-5.6-terra")
				bodyHash := sha256.Sum256(body)
				require.Equal(tt, hex.EncodeToString(bodyHash[:]), req.Header.Get("X-Amz-Content-Sha256"))
			},
			expectedInfo: func(tt require.TestingT, i1 any, i2 ...any) {
				info, _ := i1.(*RequestInfo)
				require.Equal(tt, "gpt-5.6-terra", info.RequestedModel())
				require.Equal(tt, "openai.gpt-5.6-terra", info.ProviderModel())
			},
		},
		"bedrock chat completions successful messages": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderAWSBedrock,
				Models: []*types.LLM_Model{
					{ProviderName: "openai.gpt-5.6-terra", Name: "gpt-5.6-terra"},
				},
			}, &types.AppAWS{
				Region: "us-east-2",
			}),
			signAWSRequest: signAWSRequest,
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/chat/completions",
					strings.NewReader(`{"model":"gpt-5.6-terra","stream": true,"messages":[{"role":"user","content":"Hello"}]}`),
				)
				return r
			},
			expectedError: require.NoError,
			expectedRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				req, _ := i1.(*http.Request)
				require.Equal(tt, "bedrock-mantle.us-east-2.api.aws", req.URL.Host)
				require.Equal(tt, "/v1/chat/completions", req.URL.Path)
				require.NotEmpty(tt, req.Header.Get("content-type"))
				require.NotEmpty(tt, req.Header.Get("Authorization"))

				body, err := io.ReadAll(req.Body)
				require.NoError(tt, err)
				require.Contains(tt, string(body), "openai.gpt-5.6-terra")
				bodyHash := sha256.Sum256(body)
				require.Equal(tt, hex.EncodeToString(bodyHash[:]), req.Header.Get("X-Amz-Content-Sha256"))
			},
			expectedInfo: func(tt require.TestingT, i1 any, i2 ...any) {
				info, _ := i1.(*RequestInfo)
				require.Equal(tt, "gpt-5.6-terra", info.RequestedModel())
				require.Equal(tt, "openai.gpt-5.6-terra", info.ProviderModel())
			},
		},
		"bedrock signer failure": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderAWSBedrock,
			}, &types.AppAWS{
				Region: "us-east-2",
			}),
			signAWSRequest: func(ctx context.Context, a types.Application, r *http.Request, b []byte) error {
				return errors.New("signing failed")
			},
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/responses",
					strings.NewReader(`{"model":"gpt-5.6-terra","input":"Hello"}`),
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
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderAWSBedrock,
			}, &types.AppAWS{
				Region: "us-east-2",
			}),
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/responses",
					strings.NewReader(`{"model":"gpt-5.6-terra","input":"Hello"}`),
				)
				return r
			},
			expectedError:   require.Error,
			expectedRequest: require.Nil,
			expectedInfo:    require.NotNil,
		},
		"bedrock unsupported endpoint": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderAWSBedrock,
			}, &types.AppAWS{
				Region: "us-east-2",
			}),
			signAWSRequest: signAWSRequest,
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/complete",
					strings.NewReader(`{"model":"gpt-5.6-terra","input":"Hello"}`),
				)
				return r
			},
			expectedError:   require.Error,
			expectedRequest: require.Nil,
			expectedInfo:    require.NotNil,
		},
		"bedrock unsupported method": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderAWSBedrock,
			}, &types.AppAWS{
				Region: "us-east-2",
			}),
			signAWSRequest: signAWSRequest,
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodGet,
					"/responses",
					nil,
				)
				return r
			},
			expectedError:   require.Error,
			expectedRequest: require.Nil,
			expectedInfo:    require.NotNil,
		},
		"compressed request": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderOpenAI,
			}, nil /* appAWS */),
			request: func() *http.Request {
				encoded := zstd.EncodeTo([]byte{}, []byte(`{"model":"gpt-5","input":"Hello"}`))
				r, _ := http.NewRequest(
					http.MethodPost,
					"/responses",
					bytes.NewReader(encoded),
				)
				r.Header.Add("Content-Encoding", "zstd")
				return r
			},
			expectedError: require.NoError,
			expectedRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				req, _ := i1.(*http.Request)
				require.Equal(tt, "/v1/responses", req.URL.Path)
				require.Equal(tt, http.MethodPost, req.Method)
				require.Equal(tt, "Bearer "+apiKey, req.Header.Get("Authorization"))
				require.NotEmpty(tt, req.Header.Get("content-type"))
			},
			expectedInfo: func(tt require.TestingT, i1 any, i2 ...any) {
				info, _ := i1.(*RequestInfo)
				require.Equal(tt, "gpt-5", info.RequestedModel())
				require.Equal(tt, "gpt-5", info.ProviderModel())
			},
		},
		"compressed request does not exceed max size but decompressed does": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderOpenAI,
			}, nil /* appAWS */),
			request: func() *http.Request {
				// Generate a body that exceeds the max size, but not its
				// compressed version.
				body := `{"model":"gpt-5","input":"` + strings.Repeat("a", teleport.MaxHTTPRequestSize) + `"}`
				encoded := zstd.EncodeTo([]byte{}, []byte(body))
				r, _ := http.NewRequest(
					http.MethodPost,
					"/responses",
					bytes.NewReader(encoded),
				)
				r.Header.Add("Content-Encoding", "zstd")
				return r
			},
			expectedError:   require.Error,
			expectedRequest: require.Nil,
			expectedInfo:    require.NotNil,
		},
		"compressed request exceeds max size": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderOpenAI,
			}, nil /* appAWS */),
			request: func() *http.Request {
				// Generate data until it exceeds the size. The content should
				// not matter since we expect the handler to not try to parse
				// it.
				var body bytes.Buffer
				w, _ := zstd.NewWriter(&body, zstd.WithEncoderConcurrency(1))
				for body.Len() < teleport.MaxHTTPRequestSize {
					d, _ := utils.CryptoRandomHex(teleport.MaxHTTPRequestSize)
					_, _ = w.Write([]byte(d))
				}
				w.Close()
				r, _ := http.NewRequest(
					http.MethodPost,
					"/responses",
					&body,
				)
				r.Header.Add("Content-Encoding", "zstd")
				return r
			},
			expectedError:   require.Error,
			expectedRequest: require.Nil,
			expectedInfo:    require.NotNil,
		},
		"unsupported encoding format": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderOpenAI,
			}, nil /* appAWS */),
			request: func() *http.Request {
				r, _ := http.NewRequest(
					http.MethodPost,
					"/responses",
					strings.NewReader("not read"),
				)
				r.Header.Add("Content-Encoding", "gzip")
				return r
			},
			expectedError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(tt, err, "encoding format")
			},
			expectedRequest: require.Nil,
			expectedInfo:    require.NotNil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			req, info, err := NewRequest(&llmrequest.Config{
				App:               tc.app,
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

// buildRequestBody returns a valid responses API request body whose total size
// is roughly fillerBytes.
func buildRequestBody(fillerBytes int) []byte {
	content := strings.Repeat("A", fillerBytes)
	return fmt.Appendf(nil,
		`{"model":"gpt-4o","max_output_tokens":1024,"stream":false,"input":%q}`,
		content,
	)
}

// BenchmarkNewRequest tracks the cost of the downstream request "parsing" and
// generation of the provider request, across different body sizes.
//
//	go test ./lib/srv/app/llm/openai/ -run '^$' -bench BenchmarkNewRequest -benchmem
func BenchmarkNewRequest(b *testing.B) {
	// Maps the requested model to a provider model so the conversion path
	// (the common case in production) is exercised.
	app, err := types.NewAppV3(types.Metadata{Name: "benchmark-app"}, types.AppSpecV3{
		LLM: &types.LLM{
			Provider: types.LLMProviderOpenAI,
			Format:   types.LLMFormatOpenAI,
			Models: []*types.LLM_Model{
				{ProviderName: "gpt-5", Name: "gpt-4o"},
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
			r, err := http.NewRequest(http.MethodPost, "/responses", nil)
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
