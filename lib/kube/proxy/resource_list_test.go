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
	"io"
	"log/slog"
	"testing"

	template "github.com/DataDog/datadog-agent/pkg/template/text"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
	"github.com/gravitational/teleport/lib/kube/proxy/streamfilter"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

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

	// JSON templates for realistic Kubernetes responses.
	podListTpl := template.Must(template.New("podlist").Parse(`{
		"apiVersion":"v1","kind":"PodList",
		"metadata":{"resourceVersion":"999"},
		"items":[
			{{- range $i, $p := .Pods}}{{if $i}},{{end}}
			{"apiVersion":"v1","kind":"Pod","metadata":{"name":"{{$p.Name}}","namespace":"{{$p.Namespace}}","uid":"uid-{{$p.Name}}"}}
			{{- end}}
		]
	}`))

	tableTpl := template.Must(template.New("table").Parse(`{
		"kind":"Table","apiVersion":"meta.k8s.io/v1",
		"metadata":{"resourceVersion":"999"},
		"columnDefinitions":[{"name":"Name","type":"string"}],
		"rows":[
			{{- range $i, $p := .Pods}}{{if $i}},{{end}}
			{"cells":["{{$p.Name}}"],"object":{"kind":"PartialObjectMetadata","apiVersion":"meta.k8s.io/v1","metadata":{"name":"{{$p.Name}}","namespace":"{{$p.Namespace}}","uid":"uid-{{$p.Name}}"}}}
			{{- end}}
		]
	}`))

	tableFullObjTpl := template.Must(template.New("tablefull").Parse(`{
		"kind":"Table","apiVersion":"meta.k8s.io/v1",
		"metadata":{"resourceVersion":"999"},
		"columnDefinitions":[{"name":"Name","type":"string"}],
		"rows":[
			{{- range $i, $p := .Pods}}{{if $i}},{{end}}
			{"cells":["{{$p.Name}}"],"object":{"apiVersion":"v1","kind":"Pod","metadata":{"name":"{{$p.Name}}","namespace":"{{$p.Namespace}}","uid":"uid-{{$p.Name}}"}}}
			{{- end}}
		]
	}`))

	type pod struct{ Name, Namespace string }
	pods := struct{ Pods []pod }{
		Pods: []pod{
			{"nginx", "default"},
			{"redis", "kube-system"},
			{"postgres", "default"},
			{"etcd", "kube-system"},
		},
	}

	tests := []struct {
		name string
		tpl  *template.Template
		key  string // "items" or "rows"
	}{
		{"resource list", podListTpl, "items"},
		{"table response", tableTpl, "rows"},
		{"table response full object", tableFullObjTpl, "rows"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var rawBuf bytes.Buffer
			require.NoError(t, tt.tpl.Execute(&rawBuf, pods))
			rawJSON := rawBuf.Bytes()

			// --- Streaming path ---
			matcher := newMatcher(mr, allowedResources, nil, log)
			sf := streamfilter.NewJSONFilter(matcher, log)
			var streamOut bytes.Buffer
			require.NoError(t, sf.Filter(bytes.NewReader(rawJSON), &streamOut))

			var streamParsed map[string]any
			require.NoError(t, json.Unmarshal(streamOut.Bytes(), &streamParsed))

			var streamNames []string
			if _, ok := streamParsed["items"]; ok {
				streamNames = itemNames(t, streamParsed, "items")
			} else if _, ok := streamParsed["rows"]; ok {
				streamNames = itemNames(t, streamParsed, "rows")
			}

			// --- Buffered path ---
			buf := responsewriters.NewMemoryResponseWriter()
			buf.Header().Set(responsewriters.ContentTypeHeader, responsewriters.DefaultContentType)
			buf.Write(rawJSON)
			filter := newResourceFilterer(mr, &globalKubeCodecs, newMatcher(mr, allowedResources, nil, log), log)
			require.NoError(t, filterBuffer(filter, buf))

			// Parse the buffered output as generic JSON to extract names,
			// same as the streaming path comparison.
			var bufferedParsed map[string]any
			require.NoError(t, json.Unmarshal(buf.Buffer().Bytes(), &bufferedParsed))
			var bufferedNames []string
			if _, ok := bufferedParsed["items"]; ok {
				bufferedNames = itemNames(t, bufferedParsed, "items")
			} else if _, ok := bufferedParsed["rows"]; ok {
				bufferedNames = itemNames(t, bufferedParsed, "rows")
			}

			require.ElementsMatch(t, bufferedNames, streamNames,
				"streaming and buffered paths should produce the same filtered set")
		})
	}
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
				sf := streamfilter.NewJSONFilter(matcher, log)
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					if err := sf.Filter(bytes.NewReader(jsonPayload), io.Discard); err != nil {
						b.Fatal(err)
					}
				}
			})

			b.Run(prefix+"/buffered_filter", func(b *testing.B) {
				factory := newResourceFilterer(mr, &globalKubeCodecs, newMatcher(mr, allowed, denied, log), log)
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					buf := responsewriters.NewMemoryResponseWriter()
					buf.Header().Set(responsewriters.ContentTypeHeader, responsewriters.DefaultContentType)
					buf.Write(jsonPayload)
					if err := filterBuffer(factory, buf); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
