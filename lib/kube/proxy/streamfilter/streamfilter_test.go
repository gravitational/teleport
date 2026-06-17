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

package streamfilter

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"
)

// testMatcher is a mock Matcher for testing.
type testMatcher struct {
	allowFn func(name, namespace string) (bool, error)
}

func (m *testMatcher) Match(name, ns string) (bool, error) { return m.allowFn(name, ns) }

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

func filterJSON(t *testing.T, input []byte, matcher Matcher) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	sf := NewJSONFilter(matcher, nil)
	err := sf.Filter(bytes.NewReader(input), &buf)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result), "output is not valid JSON: %s", buf.String())
	return result
}

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

func TestFilter_items(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		items   []testItem
		matcher Matcher
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

			require.Equal(t, "v1", result["apiVersion"])
			require.Equal(t, "PodList", result["kind"])
			meta, ok := result["metadata"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "12345", meta["resourceVersion"])
		})
	}
}

func TestFilter_table(t *testing.T) {
	t.Parallel()

	rows := []testItem{
		{"kubernetes", "default"},
		{"kube-dns", "kube-system"},
		{"metrics-server", "kube-system"},
	}

	tests := []struct {
		name    string
		matcher Matcher
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

			require.Equal(t, "Table", result["kind"])
			require.Equal(t, "meta.k8s.io/v1", result["apiVersion"])
			colDefs, ok := result["columnDefinitions"].([]any)
			require.True(t, ok)
			require.Len(t, colDefs, 3)
		})
	}
}

func TestFilter_edgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   []byte
		matcher Matcher
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
			name:    "item with missing metadata denied",
			input:   []byte(`{"apiVersion":"v1","kind":"PodList","metadata":{},"items":[{"kind":"Pod"},{"metadata":{"name":"good","namespace":"default"}}]}`),
			matcher: matcherAllowNamespace("default"),
			check: func(t *testing.T, result map[string]any) {
				got := itemNames(t, result, "items")
				require.Equal(t, []string{"default/good"}, got)
			},
		},
		{
			name:    "malformed item metadata skipped",
			input:   []byte(`{"apiVersion":"v1","kind":"PodList","metadata":{},"items":[{"metadata":"broken"},{"metadata":{"name":"good","namespace":"ns"}}]}`),
			matcher: matcherAllowAll(),
			check: func(t *testing.T, result map[string]any) {
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
			name:    "matcher error denies item",
			input:   []byte(`{"apiVersion":"v1","kind":"PodList","metadata":{},"items":[{"metadata":{"name":"x","namespace":"y"}}]}`),
			matcher: &testMatcher{allowFn: func(_, _ string) (bool, error) { return false, errors.New("boom") }},
			check: func(t *testing.T, result map[string]any) {
				arr, ok := result["items"].([]any)
				require.True(t, ok)
				require.Empty(t, arr)
			},
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
			sf := NewJSONFilter(tt.matcher, nil)
			err := sf.Filter(bytes.NewReader(tt.input), &buf)
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

func TestExtractItemMeta(t *testing.T) {
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
		{"missing metadata", `{"kind":"Pod"}`, "", "", true},
		{"empty object", `{}`, "", "", true},
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

func TestExtractTableRowMeta(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		input     string
		wantName  string
		wantNS    string
		wantError bool
	}{
		{"valid", `{"object":{"metadata":{"name":"svc","namespace":"default"}}}`, "svc", "default", false},
		{"missing object", `{"cells":["a"]}`, "", "", true},
		{"missing metadata in object", `{"object":{"kind":"Pod"}}`, "", "", true},
		{"empty object", `{}`, "", "", true},
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

// TestFilter_streaming verifies that items are written to the output incrementally
// as they arrive from the upstream, not buffered until the entire response is received.
// It also verifies that denied items never appear in the output.
func TestFilter_streaming(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		matcher := matcherAllowNamespace("default")
		sf := NewJSONFilter(matcher, nil)

		// src pipe: test writes upstream JSON, filter reads.
		// dst: plain buffer is safe because synctest.Wait() guarantees the
		// filter goroutine is idle (blocked on pipe read) before we inspect output.
		srcR, srcW := io.Pipe()
		var dst bytes.Buffer

		filterErr := make(chan error, 1)
		go func() {
			filterErr <- sf.Filter(srcR, &dst)
		}()

		// Write preamble + first item (allowed).
		io.WriteString(srcW, `{"apiVersion":"v1","kind":"PodList","metadata":{"resourceVersion":"1"},"items":[`)
		io.WriteString(srcW, `{"metadata":{"name":"nginx","namespace":"default"}}`)
		synctest.Wait()

		out := dst.String()
		require.Contains(t, out, `"apiVersion"`, "preamble should be written before all items arrive")
		require.Contains(t, out, `"nginx"`, "first allowed item should arrive incrementally")

		// Write denied item.
		io.WriteString(srcW, `,{"metadata":{"name":"coredns","namespace":"kube-system"}}`)
		synctest.Wait()

		out = dst.String()
		require.NotContains(t, out, `"coredns"`, "denied item must not appear in output")

		// Write second allowed item.
		io.WriteString(srcW, `,{"metadata":{"name":"redis","namespace":"default"}}`)
		synctest.Wait()

		out = dst.String()
		require.Contains(t, out, `"redis"`, "second allowed item should arrive incrementally")

		// Close the JSON list and upstream pipe.
		io.WriteString(srcW, `]}`)
		srcW.Close()
		synctest.Wait()

		require.NoError(t, <-filterErr)

		// Final output should be valid JSON with exactly the allowed items.
		var result map[string]any
		require.NoError(t, json.Unmarshal(dst.Bytes(), &result), "output should be valid JSON: %s", dst.String())
		names := itemNames(t, result, "items")
		require.Equal(t, []string{"default/nginx", "default/redis"}, names)
	})
}
