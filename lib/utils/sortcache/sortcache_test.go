/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
package sortcache

import (
	"context"
	"slices"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

const (
	Kind = "Kind"
	Name = "Name"
)

type resource struct {
	kind string
	name string
}

// TestBasics verifies basic expected behaviors for a SortCache.
func TestBasics(t *testing.T) {
	t.Parallel()

	cache := New(Config[resource]{
		Indexes: map[string]func(resource) string{
			Kind: func(r resource) string {
				return r.kind + "/" + r.name
			},
			Name: func(r resource) string {
				return r.name + "/" + r.kind
			},
		},
	})

	// verify index checking method
	require.True(t, cache.HasIndex(Name))
	require.False(t, cache.HasIndex("Foo"))

	// set up some test resources
	rscs := []resource{
		{"node", "001"},
		{"node", "002"},
		{"kube", "001"},
		{"kube", "002"},
	}

	for _, rsc := range rscs {
		require.Equal(t, 0, cache.Put(rsc))
	}

	require.Equal(t, 4, cache.Len())

	// perform some basic lookups
	r, ok := cache.Get(Kind, "node/001")
	require.True(t, ok)
	require.Equal(t, resource{"node", "001"}, r)

	r, ok = cache.Get(Name, "002/kube")
	require.True(t, ok)
	require.Equal(t, resource{"kube", "002"}, r)

	// check ascending iteration
	out := slices.Collect(cache.Ascend(Kind, "kube/", NextKey("kube/")))

	require.Len(t, out, 2)
	require.Equal(t, []resource{
		{"kube", "001"},
		{"kube", "002"},
	}, out)

	// check descending iteration
	out = slices.Collect(cache.Descend(Kind, NextKey("kube/"), "kube/"))

	require.Len(t, out, 2)
	require.Equal(t, []resource{
		{"kube", "002"},
		{"kube", "001"},
	}, out)

	// check removal
	cache.Delete(Kind, "kube/002")
	require.Equal(t, 3, cache.Len())
	_, ok = cache.Get(Kind, "kube/002")
	require.False(t, ok)

	// check clear resets cache
	cache.Clear()
	require.Equal(t, 0, cache.Len())
	_, ok = cache.Get(Kind, "node/001")
	require.False(t, ok)
}

func TestOpenBounds(t *testing.T) {
	t.Parallel()

	cache := New(Config[resource]{
		Indexes: map[string]func(resource) string{
			Kind: func(r resource) string {
				return r.kind + "/" + r.name
			},
			Name: func(r resource) string {
				return r.name + "/" + r.kind
			},
		},
	})

	// set up some test resources
	rscs := []resource{
		{"node", "001"},
		{"node", "002"},
		{"kube", "001"},
		{"kube", "002"},
	}

	for _, rsc := range rscs {
		require.Equal(t, 0, cache.Put(rsc))
	}

	// verify fully open ascend
	out := slices.Collect(cache.Ascend(Name, "", ""))
	require.Equal(t, []resource{
		{"kube", "001"},
		{"node", "001"},
		{"kube", "002"},
		{"node", "002"},
	}, out)

	// verify fully open descend
	out = slices.Collect(cache.Descend(Name, "", ""))
	require.Equal(t, []resource{
		{"node", "002"},
		{"kube", "002"},
		{"node", "001"},
		{"kube", "001"},
	}, out)

	// verify open-ended ascend
	out = slices.Collect(cache.Ascend(Name, "002/kube", ""))
	require.Equal(t, []resource{
		{"kube", "002"},
		{"node", "002"},
	}, out)

	// verify open-ended descend
	out = slices.Collect(cache.Descend(Name, "001/node", ""))
	require.Equal(t, []resource{
		{"node", "001"},
		{"kube", "001"},
	}, out)

	// verify open-start ascend
	out = slices.Collect(cache.Ascend(Name, "", "002/kube"))
	require.Equal(t, []resource{
		{"kube", "001"},
		{"node", "001"},
	}, out)

	// verify open-start descend
	out = slices.Collect(cache.Descend(Name, "", "001/node"))
	require.Equal(t, []resource{
		{"node", "002"},
		{"kube", "002"},
	}, out)
}

// TestOverlap verifies basic expected behavior when multiple resources map to the same
// value on an index.
func TestOverlap(t *testing.T) {
	t.Parallel()

	// set up indexes s.t. resources with different kinds can collide on the name
	// index and resources with different names can collide on the kind index.
	cache := New(Config[resource]{
		Indexes: map[string]func(resource) string{
			Kind: func(r resource) string {
				return r.kind
			},
			Name: func(r resource) string {
				return r.name
			},
		},
	})

	// set up test resources s.t. there is a collision on the name index
	rscs := []resource{
		{"node", "001"},
		{"db", "002"},
		{"kube", "001"},
	}

	var totalEvicted int
	for _, rsc := range rscs {
		totalEvicted += cache.Put(rsc)
	}
	require.Equal(t, 1, totalEvicted)

	// expect one of the three resources that were inserted to be overwritten
	require.Equal(t, 2, cache.Len())

	// verify that the most recently inserted value for our test collision "won", overwriting the
	// previous resource.
	r, ok := cache.Get(Name, "001")
	require.True(t, ok)
	require.Equal(t, "kube", r.kind)

	// verify that the preexisting value that wasn't part of the collision is preserved.
	r, ok = cache.Get(Kind, "db")
	require.True(t, ok)
	require.Equal(t, "002", r.name)

	// inserting a resource of a different kind, but with the same name as an existing resource, should
	// cause the existing resource to be fully deleted, including in the indexes that are not being
	// overwritten by the new value.
	require.Equal(t, 1, cache.Put(resource{"desktop", "001"}))
	_, ok = cache.Get(Kind, "kube")
	require.False(t, ok)

	require.Equal(t, 2, cache.Len())

	// inserting a resource that collides with multiple existing resources should remove all of them.
	require.Equal(t, 2, cache.Put(resource{"db", "001"}))
	require.Equal(t, 1, cache.Len())
}

// TestNextKey asserts the basic behavior of the [NextKey] helper.
func TestNextKey(t *testing.T) {
	t.Parallel()

	tts := []struct {
		key, out string
	}{
		{
			key: "nodes/",
			out: "nodes0",
		},
		{
			key: "a",
			out: "b",
		},
		{
			key: string([]byte{0x00, 0x00, 0x00}),
			out: string([]byte{0x00, 0x00, 0x01}),
		},
		{
			key: string([]byte{0xff, 0xff, 0xff}),
			out: string([]byte{0xff, 0xff, 0xff}),
		},
		{
			key: "",
			out: "",
		},
	}

	for _, tt := range tts {
		require.Equal(t, tt.out, NextKey(tt.key), "NextKey(%q)", tt.key)
	}
}

// BenchmarkSortCache attempts to benchmark reads under moderately high concurrent load.
//
// goos: linux
// goarch: amd64
// pkg: github.com/gravitational/teleport/lib/utils/sortcache
// cpu: Intel(R) Xeon(R) CPU @ 2.80GHz
// BenchmarkSortCache-4   	      12	 250820820 ns/op
func BenchmarkSortCache(b *testing.B) {
	const (
		concurrency      = 100
		resourcesPerKind = 50_000
	)

	// set up a basic cache configuration
	cache := New(Config[resource]{
		Indexes: map[string]func(resource) string{
			Kind: func(r resource) string {
				return r.kind + "/" + r.name
			},
			Name: func(r resource) string {
				return r.name + "/" + r.kind
			},
		},
	})

	// set up some test resources we can use later to
	// inject writes
	r1 := resource{
		kind: "node",
		name: uuid.New().String(),
	}

	r2 := resource{
		kind: "kube",
		name: uuid.New().String(),
	}

	cache.Put(r1)
	cache.Put(r2)

	// seed cache with lots of additional resources to help simulate large reads
	for i := 0; i < resourcesPerKind-1; i++ {
		cache.Put(resource{
			kind: "node",
			name: strconv.Itoa(i),
		})
		cache.Put(resource{
			kind: "kube",
			name: strconv.Itoa(i),
		})
	}

	// set up a background process to inject concurrent writes in a fairly
	// tight loop (should roughly simulate the load generated by a background
	// event stream injecting updates into a replica).
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			cache.Put(r1)
			cache.Put(r2)
		}
	}()

	// set up a bunch of background concurrent read operations to simulate load.
	for i := 0; i < concurrency; i++ {
		go func() {
			var n int
			buf := make([]resource, 0, resourcesPerKind)
			for {
				if ctx.Err() != nil {
					return
				}
				buf = buf[:0]
				n++
				key := "node/"
				if n%2 == 0 {
					key = "kube/"
				}

				for r := range cache.Ascend(Kind, key, NextKey(key)) {
					buf = append(buf, r)
				}

				if len(buf) != resourcesPerKind {
					panic("benchmark is misconfigured")
				}
			}
		}()
	}

	// actual benchmark gets performed against one singular read loop. the goal here being to
	// figure out what a single reader would experience when trying to pull a large block of
	// values from a SortCache that is under high concurrent load.
	var n int
	buf := make([]resource, 0, resourcesPerKind)
	for b.Loop() {
		buf = buf[:0]
		n++
		key := "node/"
		if n%2 == 0 {
			key = "kube/"
		}

		for r := range cache.Ascend(Kind, key, NextKey(key)) {
			buf = append(buf, r)
		}

		if len(buf) != resourcesPerKind {
			panic("benchmark is misconfigured")
		}
	}
}

// TestSortCachePagination validates that iterating items in the
// cache correctly paginates chunks internally.
func TestSortCachePagination(t *testing.T) {
	// set up a basic cache configuration
	cache := New(Config[resource]{
		Indexes: map[string]func(resource) string{
			Name: func(r resource) string {
				return r.name + "/" + r.kind
			},
		},
	})

	const count = 1501
	expected := make([]string, 0, count)

	for i := range count {
		name := strconv.Itoa(i)
		r := resource{
			kind: types.KindNode,
			name: name,
		}
		cache.Put(r)
		expected = append(expected, name)
	}

	slices.Sort(expected)

	t.Run("ascending", func(t *testing.T) {
		i := 0
		for r := range cache.Ascend(Name, "", "") {
			require.Equal(t, expected[i], r.name)
			i++
		}
		require.Equal(t, count, i)
	})

	t.Run("descending", func(t *testing.T) {
		i := count - 1
		for r := range cache.Descend(Name, "", "") {
			require.Equal(t, expected[i], r.name)
			i--
		}
		require.Equal(t, -1, i)
	})
}
