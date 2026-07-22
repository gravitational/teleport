/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// fwTestMatcher is a mock resourceMatcher for filteringResponseWriter tests.
type fwTestMatcher struct {
	allowFn func(name, namespace string) (bool, error)
}

func (m *fwTestMatcher) Match(name, ns string) (bool, error) { return m.allowFn(name, ns) }

func fwMatcherAllowAll() *fwTestMatcher {
	return &fwTestMatcher{allowFn: func(_, _ string) (bool, error) { return true, nil }}
}

func fwMatcherAllowNamespace(ns string) *fwTestMatcher {
	return &fwTestMatcher{allowFn: func(_, namespace string) (bool, error) { return namespace == ns, nil }}
}

func newTestFW(t *testing.T, matcher resourceMatcher) (*filteringResponseWriter, *httptest.ResponseRecorder) {
	t.Helper()
	rec := httptest.NewRecorder()
	fw := newFilteringResponseWriter(
		rec,
		matcher,
		nil,
		logtest.NewLogger(),
		t.Context(),
		noop.NewTracerProvider().Tracer("test"),
		"test",
	)
	t.Cleanup(func() { fw.Finish() })
	return fw, rec
}

func TestFilteringResponseWriter_routing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		ct        string
		encoding  string
		status    int
		streaming bool
	}{
		{"json 200 streams", "application/json", "", http.StatusOK, true},
		{"json with charset streams", "application/json; charset=utf-8", "", http.StatusOK, true},
		{"json table streams", "application/json;as=Table;g=meta.k8s.io;v=v1", "", http.StatusOK, true},
		{"empty ct defaults to json", "", "", http.StatusOK, true},
		{"identity encoding streams", "application/json", "identity", http.StatusOK, true},
		{"gzip encoding streams", "application/json", "gzip", http.StatusOK, true},
		{"protobuf buffers", "application/vnd.kubernetes.protobuf", "", http.StatusOK, false},
		{"yaml buffers", "application/yaml", "", http.StatusOK, false},
		{"non-200 buffers", "application/json", "", http.StatusForbidden, false},
		{"unsupported encoding buffers", "application/json", "br", http.StatusOK, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fw, _ := newTestFW(t, fwMatcherAllowAll())
			if tt.ct != "" {
				fw.Header().Set("Content-Type", tt.ct)
			}
			if tt.encoding != "" {
				fw.Header().Set("Content-Encoding", tt.encoding)
			}
			fw.WriteHeader(tt.status)
			require.Equal(t, tt.streaming, fw.streaming)
			if !tt.streaming {
				require.NotNil(t, fw.memBuffer)
			}
		})
	}
}

func TestFilteringResponseWriter_streaming_filters_items(t *testing.T) {
	t.Parallel()
	fw, rec := newTestFW(t, fwMatcherAllowNamespace("default"))

	fw.Header().Set("Content-Type", "application/json")
	fw.Header().Set("X-Custom", "hello")
	fw.WriteHeader(http.StatusOK)

	fw.Write([]byte(`{"apiVersion":"v1","kind":"PodList","metadata":{},"items":[` +
		`{"metadata":{"name":"nginx","namespace":"default"}},` +
		`{"metadata":{"name":"redis","namespace":"kube-system"}}` +
		`]}`))

	status, err := fw.Finish()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Header().Get("Content-Length"))
	require.Equal(t, "hello", rec.Header().Get("X-Custom"))

	body := rec.Body.String()
	require.Contains(t, body, `"nginx"`)
	require.NotContains(t, body, `"redis"`)
}

func TestFilteringResponseWriter_implicit_200(t *testing.T) {
	t.Parallel()
	fw, _ := newTestFW(t, fwMatcherAllowAll())

	fw.Header().Set("Content-Type", "application/json")
	fw.Write([]byte(`{"items":[]}`))

	require.Equal(t, http.StatusOK, fw.status)
	require.True(t, fw.streaming)
}

func TestFilteringResponseWriter_finish_no_writeheader(t *testing.T) {
	t.Parallel()
	fw, _ := newTestFW(t, fwMatcherAllowAll())

	status, err := fw.Finish()
	require.Error(t, err)
	require.Equal(t, http.StatusBadGateway, status)
}

func TestFilteringResponseWriter_writeheader_idempotent(t *testing.T) {
	t.Parallel()
	fw, _ := newTestFW(t, fwMatcherAllowAll())

	fw.Header().Set("Content-Type", "application/json")
	fw.WriteHeader(http.StatusOK)
	require.True(t, fw.streaming)

	fw.WriteHeader(http.StatusNotFound)
	require.Equal(t, http.StatusOK, fw.status)
	require.True(t, fw.streaming)
}
