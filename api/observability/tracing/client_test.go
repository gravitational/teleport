// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tracing

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	otlp "go.opentelemetry.io/proto/otlp/trace/v1"
)

var _ otlptrace.Client = (*mockClient)(nil)

type mockClient struct {
	uploadError error
	spans       []*otlp.ResourceSpans
}

func (m mockClient) Start(ctx context.Context) error {
	return nil
}

func (m mockClient) Stop(ctx context.Context) error {
	return nil
}

func (m *mockClient) UploadTraces(ctx context.Context, protoSpans []*otlp.ResourceSpans) error {
	m.spans = append(m.spans, protoSpans...)
	return m.uploadError
}

func TestUploadTraces(t *testing.T) {
	const (
		spanCount   = 10
		uploadCount = 5
	)

	cases := []struct {
		name           string
		client         mockClient
		spans          []*otlp.ResourceSpans
		errorAssertion require.ErrorAssertionFunc
		spanAssertion  require.ValueAssertionFunc
	}{
		{
			name:           "no spans to upload",
			spans:          make([]*otlp.ResourceSpans, 0, spanCount),
			errorAssertion: require.NoError,
			spanAssertion:  require.Empty,
		},
		{
			name:           "successfully uploads spans",
			spans:          make([]*otlp.ResourceSpans, spanCount),
			errorAssertion: require.NoError,
			spanAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.NotEmpty(t, i, i2...)
				require.Len(t, i, spanCount*uploadCount, i2...)
			},
		},
		{
			name:           "error uploading spans",
			spans:          make([]*otlp.ResourceSpans, spanCount),
			client:         mockClient{uploadError: trace.ConnectionProblem(nil, "test")},
			errorAssertion: require.Error,
			spanAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.NotEmpty(t, i, i2...)
				require.Len(t, i, spanCount*uploadCount, i2...)
			},
		},
		{
			name:           "not implemented",
			spans:          make([]*otlp.ResourceSpans, spanCount),
			client:         mockClient{uploadError: trace.NotImplemented("test")},
			errorAssertion: require.NoError,
			spanAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.NotEmpty(t, i, i2...)
				require.Len(t, i, spanCount, i2...)
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				Client: &tt.client,
			}

			for i := 0; i < uploadCount; i++ {
				tt.errorAssertion(t, client.UploadTraces(context.Background(), tt.spans))
			}
			tt.spanAssertion(t, tt.client.spans)
		})
	}
}
