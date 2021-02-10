/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package local

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/resource"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"

	"github.com/stretchr/testify/assert"
)

// BenchmarkGetNodes verifies the performance of the GetNodes operation
// on local (sqlite) databases (as used by the cache system).
func BenchmarkGetNodes(b *testing.B) {

	type testCase struct {
		validation, memory bool
		nodes              int
	}

	var tts []testCase

	for _, validation := range []bool{true, false} {
		for _, memory := range []bool{true, false} {
			for _, nodes := range []int{100, 1000, 10000} {
				tts = append(tts, testCase{
					validation: validation,
					memory:     memory,
					nodes:      nodes,
				})
			}
		}
	}

	for _, tt := range tts {
		// create a descriptive name for the sub-benchmark.
		name := fmt.Sprintf("tt(validation=%v,memory=%v,nodes=%d)", tt.validation, tt.memory, tt.nodes)

		// run the sub benchmark
		b.Run(name, func(sb *testing.B) {

			sb.StopTimer() // stop timer while running setup

			// set up marshal options
			var opts []auth.MarshalOption
			if !tt.validation {
				opts = append(opts, resource.SkipValidation())
			}

			// configure the backend instance
			var bk backend.Backend
			var err error
			if tt.memory {
				bk, err = memory.New(memory.Config{})
				assert.NoError(b, err)
			} else {
				dir, err := ioutil.TempDir("", "teleport")
				assert.NoError(b, err)
				defer os.RemoveAll(dir)

				bk, err = lite.NewWithConfig(context.TODO(), lite.Config{
					Path: dir,
				})
				assert.NoError(b, err)
			}
			defer bk.Close()

			svc := NewPresenceService(bk)
			// seed the test nodes
			insertNodes(b, svc, tt.nodes)

			sb.StartTimer() // restart timer for benchmark operations

			benchmarkGetNodes(sb, svc, tt.nodes, opts...)

			sb.StopTimer() // stop timer to exclude deferred cleanup
		})
	}
}

// insertNodes inserts a collection of test nodes into a backend.
func insertNodes(t assert.TestingT, svc auth.Presence, nodeCount int) {
	const labelCount = 10
	labels := make(map[string]string, labelCount)
	for i := 0; i < labelCount; i++ {
		labels[fmt.Sprintf("label-key-%d", i)] = fmt.Sprintf("label-val-%d", i)
	}
	for i := 0; i < nodeCount; i++ {
		name, addr := fmt.Sprintf("node-%d", i), fmt.Sprintf("node%d.example.com", i)
		node := &services.ServerV2{
			Kind:    services.KindNode,
			Version: services.V2,
			Metadata: services.Metadata{
				Name:      name,
				Namespace: defaults.Namespace,
				Labels:    labels,
			},
			Spec: services.ServerSpecV2{
				Addr:       addr,
				PublicAddr: addr,
			},
		}
		_, err := svc.UpsertNode(node)
		assert.NoError(t, err)
	}
}

// benchmarkGetNodes runs GetNodes b.N times.
func benchmarkGetNodes(b *testing.B, svc auth.Presence, nodeCount int, opts ...auth.MarshalOption) {
	var nodes []services.Server
	var err error
	for i := 0; i < b.N; i++ {
		nodes, err = svc.GetNodes(defaults.Namespace, opts...)
		assert.NoError(b, err)
	}
	// do *something* with the loop result.  probably unnecessary since the loop
	// contains I/O, but I don't know enough about the optimizer to be 100% certain
	// about that.
	assert.Equal(b, nodeCount, len(nodes))
}
