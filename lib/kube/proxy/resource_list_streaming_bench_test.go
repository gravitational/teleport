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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// buildRealisticPodListJSON generates a JSON PodList with n pods that have
// realistic metadata (labels, annotations, ownerReferences) to simulate
// real-world list response sizes.
func buildRealisticPodListJSON(b *testing.B, n int) []byte {
	b.Helper()
	var items []json.RawMessage
	for i := range n {
		pod := fmt.Sprintf(`{
			"apiVersion":"v1","kind":"Pod",
			"metadata":{
				"name":"app-deployment-%d-abc12",
				"namespace":"default",
				"uid":"uid-%d",
				"resourceVersion":"999%d",
				"labels":{"app":"myapp","version":"v1","env":"production","team":"platform","tier":"backend"},
				"annotations":{"kubectl.kubernetes.io/last-applied-configuration":"{}","checksum/config":"abc123def456"}
			},
			"spec":{
				"containers":[{"name":"app","image":"registry.example.com/app:v1.2.3","ports":[{"containerPort":8080}],"resources":{"requests":{"cpu":"100m","memory":"128Mi"},"limits":{"cpu":"500m","memory":"512Mi"}}}],
				"serviceAccountName":"app-sa","nodeName":"node-%d"
			},
			"status":{"phase":"Running","conditions":[{"type":"Ready","status":"True"}],"containerStatuses":[{"name":"app","ready":true,"restartCount":0,"image":"registry.example.com/app:v1.2.3","started":true}]}
		}`, i, i, i, i%10)
		items = append(items, json.RawMessage(pod))
	}
	list := map[string]any{
		"apiVersion": "v1",
		"kind":       "PodList",
		"metadata":   map[string]any{"resourceVersion": "12345", "continue": ""},
		"items":      items,
	}
	data, err := json.Marshal(list)
	require.NoError(b, err)
	return data
}

func BenchmarkStreamingJSONFilter(b *testing.B) {
	allowed := []types.KubernetesResource{
		{Kind: "pods", Name: "*", Namespace: "default", Verbs: []string{"*"}},
	}
	denied := []types.KubernetesResource{
		{Kind: "pods", Name: "app-deployment-0-*", Namespace: "*", Verbs: []string{"*"}},
	}

	for _, n := range []int{100, 1000, 5000} {
		data := buildRealisticPodListJSON(b, n)
		b.Run(fmt.Sprintf("streaming/pods=%d", n), func(b *testing.B) {
			fm, err := compileFastMatcher(allowed, denied)
			require.NoError(b, err)
			require.NotNil(b, fm)

			b.ResetTimer()
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for range b.N {
				err := streamFilterJSON(bytes.NewReader(data), io.Discard, fm, "")
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run(fmt.Sprintf("buffered/pods=%d", n), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for range b.N {
				// Simulate the buffered path: full JSON unmarshal + marshal.
				var list map[string]json.RawMessage
				if err := json.Unmarshal(data, &list); err != nil {
					b.Fatal(err)
				}
				var items []json.RawMessage
				if err := json.Unmarshal(list["items"], &items); err != nil {
					b.Fatal(err)
				}
				filtered := make([]json.RawMessage, 0, len(items))
				for _, item := range items {
					var env kubeItemEnvelope
					if err := json.Unmarshal(item, &env); err != nil {
						b.Fatal(err)
					}
					filtered = append(filtered, item)
				}
				list["items"], _ = json.Marshal(filtered)
				result, err := json.Marshal(list)
				if err != nil {
					b.Fatal(err)
				}
				_ = result
			}
		})
	}
}
