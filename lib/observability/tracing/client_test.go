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

package tracing

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	resourcev1 "go.opentelemetry.io/proto/otlp/resource/v1"
	otlp "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestRotatingFileClient(t *testing.T) {
	t.Parallel()

	// create a span to test with
	span := &otlp.ResourceSpans{
		Resource: &resourcev1.Resource{
			Attributes: []*commonv1.KeyValue{
				{
					Key: "test",
					Value: &commonv1.AnyValue{
						Value: &commonv1.AnyValue_IntValue{
							IntValue: 0,
						},
					},
				},
			},
		},
		ScopeSpans: []*otlp.ScopeSpans{
			{
				Spans: []*otlp.Span{
					{
						TraceId:           []byte{1, 2, 3, 4},
						SpanId:            []byte{5, 6, 7, 8},
						TraceState:        "",
						ParentSpanId:      []byte{9, 10, 11, 12},
						Name:              "test",
						Kind:              otlp.Span_SPAN_KIND_CLIENT,
						StartTimeUnixNano: uint64(time.Now().Add(-1 * time.Minute).Unix()),
						EndTimeUnixNano:   uint64(time.Now().Unix()),
						Attributes: []*commonv1.KeyValue{
							{
								Key: "test",
								Value: &commonv1.AnyValue{
									Value: &commonv1.AnyValue_IntValue{
										IntValue: 0,
									},
								},
							},
						},
						Status: &otlp.Status{
							Message: "success!",
							Code:    otlp.Status_STATUS_CODE_OK,
						},
					},
				},
			},
		},
	}

	const uploadCount = 10
	testSpans := []*otlp.ResourceSpans{span, span, span}

	cases := []struct {
		name         string
		limit        uint64
		filesCreated int
	}{
		{
			name:         "small limit forces rotations",
			limit:        10,
			filesCreated: uploadCount * len(testSpans),
		},
		{
			name:         "larger limit has no rotations",
			limit:        DefaultFileLimit,
			filesCreated: 1,
		},
	}

	for _, tt := range cases {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()

			client, err := NewRotatingFileClient(dir, tt.limit)
			require.NoError(t, err)

			// verify that creating the client creates a file
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			require.Len(t, entries, 1)

			// upload spans a bunch of spans
			for i := 0; i < uploadCount; i++ {
				require.NoError(t, client.UploadTraces(context.Background(), testSpans))
			}

			// stop the client to close and flush the files
			require.NoError(t, client.Stop(context.Background()))

			// ensure that if we try to upload more spans we get back ErrShutdown
			err = client.UploadTraces(context.Background(), testSpans)
			require.ErrorIs(t, err, ErrShutdown)

			// get the names of all the files created and verify that files were rotated
			entries, err = os.ReadDir(dir)
			require.NoError(t, err)
			require.Len(t, entries, tt.filesCreated)

			// read in all the spans that we just exported
			var spans []*otlp.ResourceSpans
			for _, entry := range entries {
				spans = append(spans, readFileTraces(t, filepath.Join(dir, entry.Name()))...)
			}

			// ensure that the number read matches the number of spans we uploaded
			require.Len(t, spans, uploadCount*len(testSpans))

			// confirm that all spans are equivalent to our test span
			for _, fileSpan := range spans {
				require.Empty(t, cmp.Diff(span, fileSpan,
					cmpopts.IgnoreUnexported(
						otlp.ResourceSpans{},
						otlp.ScopeSpans{},
						otlp.Span{},
						otlp.Status{},
						resourcev1.Resource{},
						commonv1.KeyValue{},
						commonv1.AnyValue{},
					),
				))
			}
		})
	}
}

func readFileTraces(t *testing.T, filename string) []*otlp.ResourceSpans {
	t.Helper()

	f, err := os.Open(filename)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, f.Close())
	}()

	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)

	var spans []*otlp.ResourceSpans
	for scanner.Scan() {
		var span otlp.ResourceSpans
		require.NoError(t, protojson.Unmarshal(scanner.Bytes(), &span))

		spans = append(spans, &span)

	}

	require.NoError(t, scanner.Err())

	return spans
}
