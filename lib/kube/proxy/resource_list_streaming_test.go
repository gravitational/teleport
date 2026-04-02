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
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// testMatcher is a mock resourceMatcher for testing.
type testMatcher struct {
	allowFn func(name, namespace string) (bool, error)
}

func (m *testMatcher) match(name, ns string) (bool, error) { return m.allowFn(name, ns) }

func matcherAllowAll() *testMatcher {
	return &testMatcher{allowFn: func(_, _ string) (bool, error) { return true, nil }}
}

func matcherDenyAll() *testMatcher {
	return &testMatcher{allowFn: func(_, _ string) (bool, error) { return false, nil }}
}

func matcherAllowNamespace(ns string) *testMatcher {
	return &testMatcher{allowFn: func(_, namespace string) (bool, error) { return namespace == ns, nil }}
}

func matcherAllowNames(names ...string) *testMatcher {
	return &testMatcher{allowFn: func(name, _ string) (bool, error) { return slices.Contains(names, name), nil }}
}

// testItem represents a Kubernetes resource for building test JSON.
type testItem struct {
	Name      string
	Namespace string
}

// buildListJSON builds a Kubernetes list JSON with the given items.
func buildListJSON(t *testing.T, apiVersion, kind string, items []testItem) []byte {
	t.Helper()
	list := map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   map[string]any{"resourceVersion": "12345"},
	}
	jsonItems := make([]map[string]any, len(items))
	for i, item := range items {
		jsonItems[i] = map[string]any{
			"apiVersion": apiVersion,
			"kind":       kind[:len(kind)-len("List")],
			"metadata": map[string]any{
				"name":      item.Name,
				"namespace": item.Namespace,
			},
		}
	}
	list["items"] = jsonItems
	data, err := json.Marshal(list)
	require.NoError(t, err)
	return data
}

// buildTableJSON builds a Kubernetes Table JSON with the given rows.
func buildTableJSON(t *testing.T, rows []testItem) []byte {
	t.Helper()
	tableRows := make([]map[string]any, len(rows))
	for i, item := range rows {
		tableRows[i] = map[string]any{
			"cells": []any{item.Name, "ClusterIP", "10.0.0.1"},
			"object": map[string]any{
				"kind":       "PartialObjectMetadata",
				"apiVersion": "meta.k8s.io/v1",
				"metadata": map[string]any{
					"name":      item.Name,
					"namespace": item.Namespace,
				},
			},
		}
	}
	table := map[string]any{
		"kind":       "Table",
		"apiVersion": "meta.k8s.io/v1",
		"metadata":   map[string]any{"resourceVersion": "99999"},
		"columnDefinitions": []map[string]any{
			{"name": "Name", "type": "string"},
			{"name": "Type", "type": "string"},
			{"name": "Cluster-IP", "type": "string"},
		},
		"rows": tableRows,
	}
	data, err := json.Marshal(table)
	require.NoError(t, err)
	return data
}

// filterJSON runs the streaming filter and returns the parsed output.
func filterJSON(t *testing.T, input []byte, matcher resourceMatcher) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	sf := &jsonStreamFilter{matcher: matcher}
	err := sf.filter(bytes.NewReader(input), &buf)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result), "output is not valid JSON: %s", buf.String())
	return result
}

// itemNames extracts item names from parsed JSON output.
func itemNames(t *testing.T, parsed map[string]any, key string) []string {
	t.Helper()
	arr, ok := parsed[key].([]any)
	if !ok {
		return nil
	}
	var names []string
	for _, raw := range arr {
		item, ok := raw.(map[string]any)
		require.True(t, ok)
		var name string
		switch key {
		case "items":
			meta := item["metadata"].(map[string]any)
			name = meta["namespace"].(string) + "/" + meta["name"].(string)
		case "rows":
			obj := item["object"].(map[string]any)
			meta := obj["metadata"].(map[string]any)
			name = meta["namespace"].(string) + "/" + meta["name"].(string)
		}
		names = append(names, name)
	}
	return names
}

func Test_newStreamFilter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		contentType string
		wantNil     bool
	}{
		{"json", "application/json", false},
		{"json with charset", "application/json; charset=utf-8", false},
		{"json table params", "application/json;as=Table;g=meta.k8s.io;v=v1", false},
		{"protobuf", "application/vnd.kubernetes.protobuf", true},
		{"empty", "", true},
		{"text html", "text/html", true},
		{"yaml", "application/yaml", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sf := newStreamFilter(tt.contentType, matcherAllowAll())
			if tt.wantNil {
				require.Nil(t, sf)
			} else {
				require.NotNil(t, sf)
				require.IsType(t, &jsonStreamFilter{}, sf)
			}
		})
	}
}

func Test_jsonStreamFilter_items(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		items   []testItem
		matcher resourceMatcher
		want    []string
	}{
		{
			name:    "all allowed",
			items:   []testItem{{"nginx-1", "default"}, {"nginx-2", "default"}, {"redis-1", "kube-system"}, {"postgres-1", "kube-system"}},
			matcher: matcherAllowAll(),
			want:    []string{"default/nginx-1", "default/nginx-2", "kube-system/redis-1", "kube-system/postgres-1"},
		},
		{
			name:    "filter by namespace",
			items:   []testItem{{"nginx-1", "default"}, {"nginx-2", "default"}, {"redis-1", "kube-system"}, {"postgres-1", "kube-system"}},
			matcher: matcherAllowNamespace("default"),
			want:    []string{"default/nginx-1", "default/nginx-2"},
		},
		{
			name:    "filter by name",
			items:   []testItem{{"nginx-1", "default"}, {"nginx-2", "default"}, {"redis-1", "kube-system"}, {"postgres-1", "kube-system"}},
			matcher: matcherAllowNames("nginx-1", "postgres-1"),
			want:    []string{"default/nginx-1", "kube-system/postgres-1"},
		},
		{
			name:    "all denied",
			items:   []testItem{{"nginx-1", "default"}, {"nginx-2", "default"}, {"redis-1", "kube-system"}, {"postgres-1", "kube-system"}},
			matcher: matcherDenyAll(),
			want:    nil,
		},
		{
			name:    "single item allowed",
			items:   []testItem{{"nginx", "default"}},
			matcher: matcherAllowAll(),
			want:    []string{"default/nginx"},
		},
		{
			name:    "single item denied",
			items:   []testItem{{"nginx", "default"}},
			matcher: matcherDenyAll(),
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			input := buildListJSON(t, "v1", "PodList", tt.items)
			result := filterJSON(t, input, tt.matcher)
			got := itemNames(t, result, "items")
			require.ElementsMatch(t, tt.want, got)

			// Verify envelope fields are preserved.
			require.Equal(t, "v1", result["apiVersion"])
			require.Equal(t, "PodList", result["kind"])
			meta, ok := result["metadata"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "12345", meta["resourceVersion"])
		})
	}
}

func Test_jsonStreamFilter_table(t *testing.T) {
	t.Parallel()

	rows := []testItem{
		{"kubernetes", "default"},
		{"kube-dns", "kube-system"},
		{"metrics-server", "kube-system"},
	}

	tests := []struct {
		name    string
		matcher resourceMatcher
		want    []string
	}{
		{
			name:    "all rows allowed",
			matcher: matcherAllowAll(),
			want:    []string{"default/kubernetes", "kube-system/kube-dns", "kube-system/metrics-server"},
		},
		{
			name:    "filter by namespace",
			matcher: matcherAllowNamespace("default"),
			want:    []string{"default/kubernetes"},
		},
		{
			name:    "all rows denied",
			matcher: matcherDenyAll(),
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			input := buildTableJSON(t, rows)
			result := filterJSON(t, input, tt.matcher)
			got := itemNames(t, result, "rows")
			require.ElementsMatch(t, tt.want, got)

			// Table metadata preserved.
			require.Equal(t, "Table", result["kind"])
			require.Equal(t, "meta.k8s.io/v1", result["apiVersion"])
			colDefs, ok := result["columnDefinitions"].([]any)
			require.True(t, ok)
			require.Len(t, colDefs, 3)
		})
	}
}

func Test_jsonStreamFilter_edgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   []byte
		matcher resourceMatcher
		wantErr string
		check   func(t *testing.T, result map[string]any)
	}{
		{
			name:    "null items array",
			input:   []byte(`{"apiVersion":"v1","kind":"PodList","metadata":{},"items":null}`),
			matcher: matcherAllowAll(),
			check: func(t *testing.T, result map[string]any) {
				require.Nil(t, result["items"])
			},
		},
		{
			name:    "empty items array",
			input:   []byte(`{"apiVersion":"v1","kind":"PodList","metadata":{},"items":[]}`),
			matcher: matcherAllowAll(),
			check: func(t *testing.T, result map[string]any) {
				arr, ok := result["items"].([]any)
				require.True(t, ok)
				require.Empty(t, arr)
			},
		},
		{
			name:    "item with missing metadata passed to matcher",
			input:   []byte(`{"apiVersion":"v1","kind":"PodList","metadata":{},"items":[{"kind":"Pod"},{"metadata":{"name":"good","namespace":"default"}}]}`),
			matcher: matcherAllowNamespace("default"),
			check: func(t *testing.T, result map[string]any) {
				// extractItemMeta on {"kind":"Pod"} returns ("","",nil) - empty name/ns.
				// Namespace filter excludes it; only the valid item passes.
				got := itemNames(t, result, "items")
				require.Equal(t, []string{"default/good"}, got)
			},
		},
		{
			name:    "malformed item metadata skipped",
			input:   []byte(`{"apiVersion":"v1","kind":"PodList","metadata":{},"items":[{"metadata":"broken"},{"metadata":{"name":"good","namespace":"ns"}}]}`),
			matcher: matcherAllowAll(),
			check: func(t *testing.T, result map[string]any) {
				// "metadata" is a string - unmarshal fails -> skipped (fail-closed).
				got := itemNames(t, result, "items")
				require.Equal(t, []string{"ns/good"}, got)
			},
		},
		{
			name:    "extra top-level fields preserved",
			input:   []byte(`{"apiVersion":"v1","kind":"PodList","metadata":{},"extraField":"hello","count":42,"items":[]}`),
			matcher: matcherAllowAll(),
			check: func(t *testing.T, result map[string]any) {
				require.Equal(t, "hello", result["extraField"])
				require.InDelta(t, float64(42), result["count"], 0)
			},
		},
		{
			name:    "matcher error propagated",
			input:   []byte(`{"apiVersion":"v1","kind":"PodList","metadata":{},"items":[{"metadata":{"name":"x","namespace":"y"}}]}`),
			matcher: &testMatcher{allowFn: func(_, _ string) (bool, error) { return false, errors.New("boom") }},
			wantErr: "boom",
		},
		{
			name:    "empty JSON object",
			input:   []byte(`{}`),
			matcher: matcherAllowAll(),
			check: func(t *testing.T, result map[string]any) {
				require.Empty(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			sf := &jsonStreamFilter{matcher: tt.matcher}
			err := sf.filter(bytes.NewReader(tt.input), &buf)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			var result map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &result), "output is not valid JSON: %s", buf.String())
			if tt.check != nil {
				tt.check(t, result)
			}
		})
	}
}

func Test_extractItemMeta(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		input     string
		wantName  string
		wantNS    string
		wantError bool
	}{
		{"valid", `{"metadata":{"name":"nginx","namespace":"default"}}`, "nginx", "default", false},
		{"missing namespace", `{"metadata":{"name":"nginx"}}`, "nginx", "", false},
		{"missing metadata", `{"kind":"Pod"}`, "", "", false},
		{"empty object", `{}`, "", "", false},
		{"invalid JSON", `not-json`, "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			name, ns, err := extractItemMeta(json.RawMessage(tt.input))
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantName, name)
				require.Equal(t, tt.wantNS, ns)
			}
		})
	}
}

func Test_extractTableRowMeta(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		input     string
		wantName  string
		wantNS    string
		wantError bool
	}{
		{"valid", `{"object":{"metadata":{"name":"svc","namespace":"default"}}}`, "svc", "default", false},
		{"missing object", `{"cells":["a"]}`, "", "", false},
		{"missing metadata in object", `{"object":{"kind":"Pod"}}`, "", "", false},
		{"empty object", `{}`, "", "", false},
		{"invalid JSON", `{broken`, "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			name, ns, err := extractTableRowMeta(json.RawMessage(tt.input))
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantName, name)
				require.Equal(t, tt.wantNS, ns)
			}
		})
	}
}

func Test_filterAcceptJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		accept string
		want   string
	}{
		{"empty header", "", "application/json"},
		{"wildcard", "*/*", "application/json"},
		{"explicit json", "application/json", "application/json"},
		{"json among multiple", "application/vnd.kubernetes.protobuf, application/json", "application/json"},
		{"json table format", "application/json;as=Table;g=meta.k8s.io;v=v1", "application/json;as=Table;g=meta.k8s.io;v=v1"},
		{"table plus plain json", "application/json;as=Table;v=v1;g=meta.k8s.io,application/json,application/vnd.kubernetes.protobuf", "application/json;as=Table;v=v1;g=meta.k8s.io,application/json"},
		{"protobuf only", "application/vnd.kubernetes.protobuf", ""},
		{"yaml only", "application/yaml", ""},
		{"text html", "text/html", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req, err := http.NewRequest("GET", "/api/v1/pods", nil)
			require.NoError(t, err)
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}
			require.Equal(t, tt.want, filterAcceptJSON(req))
		})
	}
}

func Test_wrapContentEncoding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		encoding string
		wantErr  string
		check    func(t *testing.T)
	}{
		{
			name:     "unsupported encoding",
			encoding: "br",
			wantErr:  "unsupported Content-Encoding",
		},
		{
			name:     "deflate unsupported",
			encoding: "deflate",
			wantErr:  "unsupported Content-Encoding",
		},
		{
			name:     "empty encoding",
			encoding: "",
			check: func(t *testing.T) {
				r, w, err := wrapContentEncoding(bytes.NewReader(nil), io.Discard, "")
				require.NoError(t, err)
				require.NoError(t, r.Close())
				require.NoError(t, w.Close())
			},
		},
		{
			name:     "identity encoding",
			encoding: "identity",
			check: func(t *testing.T) {
				r, w, err := wrapContentEncoding(bytes.NewReader(nil), io.Discard, "identity")
				require.NoError(t, err)
				require.NoError(t, r.Close())
				require.NoError(t, w.Close())
			},
		},
		{
			name:     "gzip round trip",
			encoding: "gzip",
			check: func(t *testing.T) {
				original := []byte("hello, gzip world!")

				var compressed bytes.Buffer
				gz := gzip.NewWriter(&compressed)
				_, err := gz.Write(original)
				require.NoError(t, err)
				require.NoError(t, gz.Close())

				var recompressed bytes.Buffer
				reader, writer, err := wrapContentEncoding(&compressed, &recompressed, "gzip")
				require.NoError(t, err)

				decompressed, err := io.ReadAll(reader)
				require.NoError(t, err)
				require.NoError(t, reader.Close())
				require.Equal(t, original, decompressed)

				_, err = writer.Write(original)
				require.NoError(t, err)
				require.NoError(t, writer.Close())

				gzr, err := gzip.NewReader(&recompressed)
				require.NoError(t, err)
				result, err := io.ReadAll(gzr)
				require.NoError(t, err)
				require.Equal(t, original, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.wantErr != "" {
				_, _, err := wrapContentEncoding(bytes.NewReader(nil), io.Discard, tt.encoding)
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			tt.check(t)
		})
	}
}

func Test_headerCapturer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup func(hc *headerCapturer)
		check func(t *testing.T, hc *headerCapturer, body *bytes.Buffer)
	}{
		{
			name: "captures headers and status",
			setup: func(hc *headerCapturer) {
				hc.Header().Set("Content-Type", "application/json")
				hc.WriteHeader(201)
			},
			check: func(t *testing.T, hc *headerCapturer, _ *bytes.Buffer) {
				require.Equal(t, 201, hc.status)
				require.Equal(t, "application/json", hc.headers.Get("Content-Type"))
			},
		},
		{
			name: "WriteHeader called twice keeps first",
			setup: func(hc *headerCapturer) {
				hc.WriteHeader(200)
				hc.WriteHeader(404)
			},
			check: func(t *testing.T, hc *headerCapturer, _ *bytes.Buffer) {
				require.Equal(t, 200, hc.status)
			},
		},
		{
			name: "Write triggers implicit 200",
			setup: func(hc *headerCapturer) {
				hc.Write([]byte("hello"))
			},
			check: func(t *testing.T, hc *headerCapturer, _ *bytes.Buffer) {
				require.Equal(t, http.StatusOK, hc.status)
			},
		},
		{
			name: "body written to underlying writer",
			setup: func(hc *headerCapturer) {
				hc.Write([]byte("hello world"))
			},
			check: func(t *testing.T, _ *headerCapturer, body *bytes.Buffer) {
				require.Equal(t, "hello world", body.String())
			},
		},
		{
			name: "wroteHeader channel closed on WriteHeader",
			setup: func(hc *headerCapturer) {
				hc.WriteHeader(200)
			},
			check: func(t *testing.T, hc *headerCapturer, _ *bytes.Buffer) {
				select {
				case <-hc.wroteHeader:
				default:
					t.Fatal("wroteHeader channel not closed")
				}
			},
		},
		{
			name: "wroteHeader channel closed on Write",
			setup: func(hc *headerCapturer) {
				hc.Write([]byte("x"))
			},
			check: func(t *testing.T, hc *headerCapturer, _ *bytes.Buffer) {
				select {
				case <-hc.wroteHeader:
				default:
					t.Fatal("wroteHeader channel not closed")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			hc := newHeaderCapturer(&buf)
			tt.setup(hc)
			tt.check(t, hc, &buf)
		})
	}
}

func Test_jsonStreamFilter_roundTrip(t *testing.T) {
	t.Parallel()

	allowedResources := []types.KubernetesResource{
		{
			Kind:      types.KindKubePod,
			APIGroup:  "*",
			Namespace: "default",
			Name:      "*",
			Verbs:     []string{types.KubeVerbList},
		},
	}

	mr := metaResource{
		requestedResource: apiResource{
			resourceKind: types.KindKubePod,
			apiGroup:     "",
		},
		resourceDefinition: &metav1.APIResource{Namespaced: true},
		verb:               types.KubeVerbList,
	}
	log := logtest.NewLogger()

	type testCase struct {
		name     string
		dataFile string
		want     []string
	}
	tests := []testCase{
		{
			name:     "resource list",
			dataFile: "testing/data/resources_list.tmpl",
			want: []string{
				"default/nginx-deployment-6595874d85-6j2zm",
				"default/nginx-deployment-6595874d85-7xgmb",
				"default/nginx-deployment-6595874d85-c4kz8",
			},
		},
		{
			name:     "table response",
			dataFile: "testing/data/partial_table.json",
			want:     []string{"default/kubernetes"},
		},
		{
			name:     "table response full object",
			dataFile: "testing/data/partial_table_full_obj.json",
			want:     []string{"default/kubernetes"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Parse template/file to get raw JSON.
			temp, err := template.ParseFiles(tt.dataFile)
			require.NoError(t, err)
			var data bytes.Buffer
			err = temp.ExecuteTemplate(&data, baseName(tt.dataFile), map[string]any{
				"Kind": "Pod",
				"API":  "v1",
			})
			require.NoError(t, err)
			rawJSON := data.Bytes()

			// --- Streaming path ---
			matcher := newMatcher(mr, allowedResources, nil, log)
			sf := newStreamFilter("application/json", matcher)
			require.NotNil(t, sf)
			var streamOut bytes.Buffer
			require.NoError(t, sf.filter(bytes.NewReader(rawJSON), &streamOut))

			// Parse streaming output to extract item/row names.
			var streamParsed map[string]any
			require.NoError(t, json.Unmarshal(streamOut.Bytes(), &streamParsed))

			var streamNames []string
			if items, ok := streamParsed["items"]; ok {
				streamNames = itemNames(t, streamParsed, "items")
				_ = items
			} else if _, ok := streamParsed["rows"]; ok {
				streamNames = itemNames(t, streamParsed, "rows")
			}

			// --- Buffered path ---
			buf := responsewriters.NewMemoryResponseWriter()
			buf.Header().Set(responsewriters.ContentTypeHeader, responsewriters.DefaultContentType)
			buf.Write(rawJSON)
			filterWrapper := newResourceFilterer(mr, &globalKubeCodecs, allowedResources, nil, log)
			require.NoError(t, filterBuffer(filterWrapper, buf))

			_, decoder, err := newEncoderAndDecoderForContentType(responsewriters.DefaultContentType, newClientNegotiator(&globalKubeCodecs))
			require.NoError(t, err)
			obj, _, err := decoder.Decode(buf.Buffer().Bytes(), nil, nil)
			require.NoError(t, err)

			var bufferedNames []string
			switch o := obj.(type) {
			case *metav1.Table:
				for i := range o.Rows {
					row := &o.Rows[i]
					if row.Object.Object == nil {
						row.Object.Object, err = decodeAndSetGVK(decoder, row.Object.Raw, nil)
						require.NoError(t, err)
					}
					resource, err := getKubeResourcePartialMetadataObject(types.KindKubePod, "", "list", row.Object.Object)
					require.NoError(t, err)
					bufferedNames = append(bufferedNames, resource.Namespace+"/"+resource.Name)
				}
			default:
				// For lists, re-encode to JSON and extract items.
				reencoded, err := json.Marshal(obj)
				require.NoError(t, err)
				var parsed map[string]any
				require.NoError(t, json.Unmarshal(reencoded, &parsed))
				if items, ok := parsed["items"].([]any); ok {
					for _, raw := range items {
						item := raw.(map[string]any)
						meta := item["metadata"].(map[string]any)
						bufferedNames = append(bufferedNames, meta["namespace"].(string)+"/"+meta["name"].(string))
					}
				}
			}

			// Both paths must produce the same set of resources.
			require.ElementsMatch(t, tt.want, streamNames, "streaming path mismatch")
			require.ElementsMatch(t, tt.want, bufferedNames, "buffered path mismatch")
			require.ElementsMatch(t, streamNames, bufferedNames, "streaming vs buffered mismatch")
		})
	}
}

// baseName returns the base name of a file path for template execution.
func baseName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}

func BenchmarkStreamFilter(b *testing.B) {
	log := slog.New(slog.DiscardHandler)

	mr := metaResource{
		requestedResource: apiResource{
			resourceKind: types.KindKubePod,
			apiGroup:     "",
		},
		resourceDefinition: &metav1.APIResource{Namespaced: true},
		verb:               types.KubeVerbList,
	}

	namespaces := []string{"default", "staging", "monitoring", "kube-system", "production"}

	buildRules := func(ruleCount int) (allowed, denied []types.KubernetesResource) {
		for i := range ruleCount {
			allowed = append(allowed, types.KubernetesResource{
				Kind:      types.KindKubePod,
				Namespace: namespaces[i%len(namespaces)],
				Name:      fmt.Sprintf("app-%d-*", i),
				Verbs:     []string{types.KubeVerbList},
				APIGroup:  "",
			})
		}
		denied = []types.KubernetesResource{
			{Kind: types.KindKubePod, Namespace: "default", Name: "secret-*", Verbs: []string{types.KubeVerbList}, APIGroup: ""},
		}
		return allowed, denied
	}

	buildJSON := func(itemCount int) []byte {
		items := make([]map[string]any, itemCount)
		for i := range items {
			items[i] = map[string]any{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]any{
					"name":      fmt.Sprintf("pod-%d", i),
					"namespace": namespaces[i%len(namespaces)],
				},
			}
		}
		data, err := json.Marshal(map[string]any{
			"apiVersion": "v1",
			"kind":       "PodList",
			"metadata":   map[string]any{"resourceVersion": ""},
			"items":      items,
		})
		if err != nil {
			b.Fatal(err)
		}
		return data
	}

	for _, ruleCount := range []int{4, 50, 150} {
		allowed, denied := buildRules(ruleCount)

		for _, itemCount := range []int{500, 5000} {
			jsonPayload := buildJSON(itemCount)
			prefix := fmt.Sprintf("%d_rules/%d_items", ruleCount, itemCount)

			b.Run(prefix+"/stream_filter", func(b *testing.B) {
				matcher := newMatcher(mr, allowed, denied, log)
				sf := newStreamFilter("application/json", matcher)
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					sf.filter(bytes.NewReader(jsonPayload), io.Discard)
				}
			})

			b.Run(prefix+"/buffered_filter", func(b *testing.B) {
				filterWrapper := newResourceFilterer(mr, &globalKubeCodecs, allowed, denied, log)
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					buf := responsewriters.NewMemoryResponseWriter()
					buf.Header().Set(responsewriters.ContentTypeHeader, responsewriters.DefaultContentType)
					buf.Write(jsonPayload)
					filterBuffer(filterWrapper, buf)
				}
			})
		}
	}
}
