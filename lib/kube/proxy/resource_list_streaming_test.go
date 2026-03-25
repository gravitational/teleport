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
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// buildPodListJSON generates a JSON PodList with n pods.
// Pods are named "pod-0" through "pod-(n-1)" in namespace "default".
func buildPodListJSON(t *testing.T, n int) []byte {
	t.Helper()
	var items []json.RawMessage
	for i := range n {
		pod := fmt.Sprintf(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-%d","namespace":"default"},"spec":{}}`, i)
		items = append(items, json.RawMessage(pod))
	}
	list := map[string]any{
		"apiVersion": "v1",
		"kind":       "PodList",
		"metadata":   map[string]any{"resourceVersion": "12345"},
		"items":      items,
	}
	data, err := json.Marshal(list)
	require.NoError(t, err)
	return data
}

// buildTableJSON generates a JSON Table response with n rows.
func buildTableJSON(t *testing.T, n int) []byte {
	t.Helper()
	var rows []json.RawMessage
	for i := range n {
		row := fmt.Sprintf(`{"cells":["pod-%d","1/1","Running"],"object":{"apiVersion":"v1","kind":"PartialObjectMetadata","metadata":{"name":"pod-%d","namespace":"default"}}}`, i, i)
		rows = append(rows, json.RawMessage(row))
	}
	table := map[string]any{
		"apiVersion":        "meta.k8s.io/v1",
		"kind":              "Table",
		"columnDefinitions": []any{},
		"metadata":          map[string]any{"resourceVersion": "12345"},
		"rows":              rows,
	}
	data, err := json.Marshal(table)
	require.NoError(t, err)
	return data
}

// compileFastMatcherForTest compiles a fast matcher from allow/deny rules.
// Returns the matcher and fails the test if compilation fails.
func compileFastMatcherForTest(t *testing.T, allowed, denied []types.KubernetesResource) resourceMatcher {
	t.Helper()
	fm, err := compileFastMatcher(allowed, denied)
	require.NoError(t, err)
	require.NotNil(t, fm, "fast matcher should compile for test rules")
	return fm
}

func TestStreamFilterJSON_RegularList(t *testing.T) {
	input := buildPodListJSON(t, 5)

	// Allow only even-numbered pods.
	fm := compileFastMatcherForTest(t, []types.KubernetesResource{
		{Kind: "pods", Name: "pod-0", Namespace: "*", Verbs: []string{"*"}},
		{Kind: "pods", Name: "pod-2", Namespace: "*", Verbs: []string{"*"}},
		{Kind: "pods", Name: "pod-4", Namespace: "*", Verbs: []string{"*"}},
	}, nil)

	var out bytes.Buffer
	err := streamFilterJSON(bytes.NewReader(input), &out, fm, "")
	require.NoError(t, err)

	var result map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))

	// Verify envelope fields are preserved.
	require.Contains(t, string(result["kind"]), "PodList")
	require.Contains(t, string(result["apiVersion"]), "v1")
	require.NotEmpty(t, result["metadata"])

	// Verify filtered items.
	var items []json.RawMessage
	require.NoError(t, json.Unmarshal(result["items"], &items))
	require.Len(t, items, 3)

	for i, expected := range []string{"pod-0", "pod-2", "pod-4"} {
		var env kubeItemEnvelope
		require.NoError(t, json.Unmarshal(items[i], &env))
		require.Equal(t, expected, env.Metadata.Name)
	}
}

func TestStreamFilterJSON_TableFormat(t *testing.T) {
	input := buildTableJSON(t, 4)

	// Allow only pod-1 and pod-3.
	fm := compileFastMatcherForTest(t, []types.KubernetesResource{
		{Kind: "pods", Name: "pod-1", Namespace: "*", Verbs: []string{"*"}},
		{Kind: "pods", Name: "pod-3", Namespace: "*", Verbs: []string{"*"}},
	}, nil)

	var out bytes.Buffer
	err := streamFilterJSON(bytes.NewReader(input), &out, fm, "")
	require.NoError(t, err)

	var result map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))

	require.Contains(t, string(result["kind"]), "Table")

	var rows []json.RawMessage
	require.NoError(t, json.Unmarshal(result["rows"], &rows))
	require.Len(t, rows, 2)

	for i, expected := range []string{"pod-1", "pod-3"} {
		var env kubeTableRowEnvelope
		require.NoError(t, json.Unmarshal(rows[i], &env))
		require.Equal(t, expected, env.Object.Metadata.Name)
	}
}

func TestStreamFilterJSON_EmptyItems(t *testing.T) {
	input := `{"apiVersion":"v1","kind":"PodList","metadata":{},"items":[]}`

	fm := compileFastMatcherForTest(t, []types.KubernetesResource{
		{Kind: "pods", Name: "*", Namespace: "*", Verbs: []string{"*"}},
	}, nil)

	var out bytes.Buffer
	err := streamFilterJSON(strings.NewReader(input), &out, fm, "")
	require.NoError(t, err)

	var result map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))

	var items []json.RawMessage
	require.NoError(t, json.Unmarshal(result["items"], &items))
	require.Empty(t, items)
}

func TestStreamFilterJSON_AllFiltered(t *testing.T) {
	input := buildPodListJSON(t, 3)

	// Deny all pods.
	fm := compileFastMatcherForTest(t, []types.KubernetesResource{
		{Kind: "pods", Name: "nonexistent-*", Namespace: "*", Verbs: []string{"*"}},
	}, nil)

	var out bytes.Buffer
	err := streamFilterJSON(bytes.NewReader(input), &out, fm, "")
	require.NoError(t, err)

	var result map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))

	var items []json.RawMessage
	require.NoError(t, json.Unmarshal(result["items"], &items))
	require.Empty(t, items)
}

func TestStreamFilterJSON_NullItems(t *testing.T) {
	input := `{"apiVersion":"v1","kind":"PodList","metadata":{},"items":null}`

	fm := compileFastMatcherForTest(t, []types.KubernetesResource{
		{Kind: "pods", Name: "*", Namespace: "*", Verbs: []string{"*"}},
	}, nil)

	var out bytes.Buffer
	err := streamFilterJSON(strings.NewReader(input), &out, fm, "")
	require.NoError(t, err)

	var result map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))
	require.Equal(t, "null", string(result["items"]))
}

func TestStreamFilterJSON_SingleItemAllowed(t *testing.T) {
	input := buildPodListJSON(t, 1)

	fm := compileFastMatcherForTest(t, []types.KubernetesResource{
		{Kind: "pods", Name: "*", Namespace: "*", Verbs: []string{"*"}},
	}, nil)

	var out bytes.Buffer
	err := streamFilterJSON(bytes.NewReader(input), &out, fm, "")
	require.NoError(t, err)

	var result map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))

	var items []json.RawMessage
	require.NoError(t, json.Unmarshal(result["items"], &items))
	require.Len(t, items, 1)
}

func TestStreamFilterJSON_SingleItemDenied(t *testing.T) {
	input := buildPodListJSON(t, 1)

	fm := compileFastMatcherForTest(t, []types.KubernetesResource{
		{Kind: "pods", Name: "nonexistent", Namespace: "*", Verbs: []string{"*"}},
	}, nil)

	var out bytes.Buffer
	err := streamFilterJSON(bytes.NewReader(input), &out, fm, "")
	require.NoError(t, err)

	var result map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))

	var items []json.RawMessage
	require.NoError(t, json.Unmarshal(result["items"], &items))
	require.Empty(t, items)
}

func TestStreamFilterJSON_DenyRules(t *testing.T) {
	input := buildPodListJSON(t, 5)

	// Allow all, deny pod-2.
	fm := compileFastMatcherForTest(t, []types.KubernetesResource{
		{Kind: "pods", Name: "*", Namespace: "*", Verbs: []string{"*"}},
	}, []types.KubernetesResource{
		{Kind: "pods", Name: "pod-2", Namespace: "*", Verbs: []string{"*"}},
	})

	var out bytes.Buffer
	err := streamFilterJSON(bytes.NewReader(input), &out, fm, "")
	require.NoError(t, err)

	var result map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))

	var items []json.RawMessage
	require.NoError(t, json.Unmarshal(result["items"], &items))
	require.Len(t, items, 4)

	for _, item := range items {
		var env kubeItemEnvelope
		require.NoError(t, json.Unmarshal(item, &env))
		require.NotEqual(t, "pod-2", env.Metadata.Name)
	}
}

func TestStreamFilterJSON_MalformedJSON(t *testing.T) {
	input := `{"items": not valid json}`

	fm := compileFastMatcherForTest(t, []types.KubernetesResource{
		{Kind: "pods", Name: "*", Namespace: "*", Verbs: []string{"*"}},
	}, nil)

	var out bytes.Buffer
	err := streamFilterJSON(strings.NewReader(input), &out, fm, "")
	require.Error(t, err)
}

func TestStreamFilterJSON_ValidOutputJSON(t *testing.T) {
	// Verify the output is valid JSON for various sizes.
	for _, n := range []int{0, 1, 5, 100} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			input := buildPodListJSON(t, n)

			fm := compileFastMatcherForTest(t, []types.KubernetesResource{
				{Kind: "pods", Name: "pod-*", Namespace: "*", Verbs: []string{"*"}},
			}, nil)

			var out bytes.Buffer
			err := streamFilterJSON(bytes.NewReader(input), &out, fm, "")
			require.NoError(t, err)

			// Output must be valid JSON.
			require.True(t, json.Valid(out.Bytes()), "output is not valid JSON: %s", out.String())
		})
	}
}
