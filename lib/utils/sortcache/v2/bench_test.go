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

package sortcache

import (
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	v1 "github.com/gravitational/teleport/lib/utils/sortcache"
)

func seed(n int) []*server {
	out := make([]*server, n)
	hosts := max(n/8, 1)
	for i := range out {
		out[i] = &server{
			name: fmt.Sprintf("%08x", i),
			host: fmt.Sprintf("host-%06d", i%hosts),
		}
	}
	return out
}

func newV1() *v1.SortCache[*server, string] {
	return v1.New(v1.Config[*server, string]{Indexes: map[string]func(*server) string{
		"name": func(s *server) string { return s.name },
		"host": func(s *server) string { return s.host + "/" + s.name },
	}})
}

var benchSizes = []int{1_000, 100_000}

// BenchmarkPut measures single-item update churn: the event-stream write
// path. v2 pays btree COW node copies per commit; v1 takes a write lock and
// mutates in place.
func BenchmarkPut(b *testing.B) {
	for _, n := range benchSizes {
		items := seed(n)
		b.Run(fmt.Sprintf("v1/n=%d", n), func(b *testing.B) {
			c := newV1()
			for _, s := range items {
				c.Put(s)
			}
			b.ReportAllocs()
			i := 0
			for b.Loop() {
				c.Put(items[i%n])
				i++
			}
		})
		b.Run(fmt.Sprintf("v2/n=%d", n), func(b *testing.B) {
			c, _, _ := newCache()
			c.Replace(slices.Values(items))
			b.ReportAllocs()
			i := 0
			for b.Loop() {
				c.Put(items[i%n])
				i++
			}
		})
	}
}

// BenchmarkPutBatch measures the amortization of per-commit overhead when
// event batches are applied as one commit (ns/op is per batch of 100).
func BenchmarkPutBatch100(b *testing.B) {
	const n = 100_000
	const batch = 100
	items := seed(n)
	b.Run("v1", func(b *testing.B) {
		c := newV1()
		for _, s := range items {
			c.Put(s)
		}
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			for range batch {
				c.Put(items[i%n])
				i++
			}
		}
	})
	b.Run("v2", func(b *testing.B) {
		c, _, _ := newCache()
		c.Replace(slices.Values(items))
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			c.Put(items[i%n : i%n+batch]...)
			i = (i + batch) % (n - batch)
		}
	})
}

// BenchmarkGet measures single-key point reads.
func BenchmarkGet(b *testing.B) {
	for _, n := range benchSizes {
		items := seed(n)
		b.Run(fmt.Sprintf("v1/n=%d", n), func(b *testing.B) {
			c := newV1()
			for _, s := range items {
				c.Put(s)
			}
			b.ReportAllocs()
			i := 0
			for b.Loop() {
				c.Get("name", items[i%n].name)
				i++
			}
		})
		b.Run(fmt.Sprintf("v2/n=%d", n), func(b *testing.B) {
			c, nameIdx, _ := newCache()
			c.Replace(slices.Values(items))
			b.ReportAllocs()
			i := 0
			for b.Loop() {
				nameIdx.Get(c.Snapshot(), items[i%n].name)
				i++
			}
		})
	}
}

// BenchmarkAscendPage measures reading one 1000-item page.
func BenchmarkAscendPage(b *testing.B) {
	const n = 100_000
	const page = 1000
	items := seed(n)
	b.Run("v1", func(b *testing.B) {
		c := newV1()
		for _, s := range items {
			c.Put(s)
		}
		b.ReportAllocs()
		for b.Loop() {
			cnt := 0
			for range c.Ascend("name", "", "") {
				cnt++
				if cnt == page {
					break
				}
			}
			if cnt != page {
				b.Fatalf("expected %d items, got %d", page, cnt)
			}
		}
	})
	b.Run("v2", func(b *testing.B) {
		c, nameIdx, _ := newCache()
		c.Replace(slices.Values(items))
		b.ReportAllocs()
		for b.Loop() {
			cnt := 0
			for range nameIdx.Ascend(c.Snapshot(), Open[string](), Open[string]()) {
				cnt++
				if cnt == page {
					break
				}
			}
			if cnt != page {
				b.Fatalf("expected %d items, got %d", page, cnt)
			}
		}
	})
}

// BenchmarkAscendFull measures a full iteration over 100k items.
func BenchmarkAscendFull(b *testing.B) {
	const n = 100_000
	items := seed(n)
	b.Run("v1", func(b *testing.B) {
		c := newV1()
		for _, s := range items {
			c.Put(s)
		}
		b.ReportAllocs()
		for b.Loop() {
			cnt := 0
			for range c.Ascend("name", "", "") {
				cnt++
			}
			if cnt != n {
				b.Fatalf("expected %d items, got %d", n, cnt)
			}
		}
	})
	b.Run("v2", func(b *testing.B) {
		c, nameIdx, _ := newCache()
		c.Replace(slices.Values(items))
		b.ReportAllocs()
		for b.Loop() {
			cnt := 0
			for range nameIdx.Ascend(c.Snapshot(), Open[string](), Open[string]()) {
				cnt++
			}
			if cnt != n {
				b.Fatalf("expected %d items, got %d", n, cnt)
			}
		}
	})
}

// BenchmarkBulkLoad measures initializing a cache with 100k items: v1 has no
// bulk path (per-item Put), v2 builds aside and swaps once via Replace.
func BenchmarkBulkLoad(b *testing.B) {
	const n = 100_000
	items := seed(n)
	b.Run("v1", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			c := newV1()
			for _, s := range items {
				c.Put(s)
			}
			if c.Len() != n {
				b.Fatalf("expected %d items, got %d", n, c.Len())
			}
		}
	})
	b.Run("v2", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			c, _, _ := newCache()
			c.Replace(slices.Values(items))
			if c.Len() != n {
				b.Fatalf("expected %d items, got %d", n, c.Len())
			}
		}
	})
}

// BenchmarkReadUnderWriteLoad measures page-read latency while a paced
// background writer (~20k single-item commits/sec) churns the cache. This is
// the scenario the snapshot design targets: v1 readers contend with the
// writer on the cache RWMutex, v2 readers are wait-free.
func BenchmarkReadUnderWriteLoad(b *testing.B) {
	const n = 100_000
	const page = 1000
	items := seed(n)

	runWriter := func(put func(i int)) (stop func()) {
		done := make(chan struct{})
		var wg sync.WaitGroup
		wg.Go(func() {
			i := 0
			for {
				select {
				case <-done:
					return
				default:
				}
				put(i)
				i++
				time.Sleep(50 * time.Microsecond)
			}
		})
		return func() {
			close(done)
			wg.Wait()
		}
	}

	b.Run("v1", func(b *testing.B) {
		c := newV1()
		for _, s := range items {
			c.Put(s)
		}
		stop := runWriter(func(i int) { c.Put(items[i%n]) })
		defer stop()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				cnt := 0
				for range c.Ascend("name", "", "") {
					cnt++
					if cnt == page {
						break
					}
				}
			}
		})
	})
	b.Run("v2", func(b *testing.B) {
		c, nameIdx, _ := newCache()
		c.Replace(slices.Values(items))
		stop := runWriter(func(i int) { c.Put(items[i%n]) })
		defer stop()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				cnt := 0
				for range nameIdx.Ascend(c.Snapshot(), Open[string](), Open[string]()) {
					cnt++
					if cnt == page {
						break
					}
				}
			}
		})
	})
}

// BenchmarkWriteUnderReadLoad measures single-item commit latency while four
// background readers continuously iterate full pages.
func BenchmarkWriteUnderReadLoad(b *testing.B) {
	const n = 100_000
	const page = 1000
	items := seed(n)

	b.Run("v1", func(b *testing.B) {
		c := newV1()
		for _, s := range items {
			c.Put(s)
		}
		done := make(chan struct{})
		var wg sync.WaitGroup
		for range 4 {
			wg.Go(func() {
				for {
					select {
					case <-done:
						return
					default:
					}
					cnt := 0
					for range c.Ascend("name", "", "") {
						cnt++
						if cnt == page {
							break
						}
					}
				}
			})
		}
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			c.Put(items[i%n])
			i++
		}
		close(done)
		wg.Wait()
	})
	b.Run("v2", func(b *testing.B) {
		c, nameIdx, _ := newCache()
		c.Replace(slices.Values(items))
		done := make(chan struct{})
		var wg sync.WaitGroup
		for range 4 {
			wg.Go(func() {
				for {
					select {
					case <-done:
						return
					default:
					}
					cnt := 0
					for range nameIdx.Ascend(c.Snapshot(), Open[string](), Open[string]()) {
						cnt++
						if cnt == page {
							break
						}
					}
				}
			})
		}
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			c.Put(items[i%n])
			i++
		}
		close(done)
		wg.Wait()
	})
}

// BenchmarkPutMovedKey measures the update flow where the write moves a key
// on a secondary index — a heartbeat extending an expiry against an
// expiry-ordered index. "stable" re-puts with an unchanged expiry (no key
// movement, v2's fast path); "moved" advances the expiry every put, forcing
// the delete+reinsert on the expiry index that stable updates avoid. v1 pays
// full delete+reinsert on every index in both modes; its expiry index also
// pays string key formatting, which is the honest cost of v1's string-only
// keys.
func BenchmarkPutMovedKey(b *testing.B) {
	const n = 100_000
	items := seed(n)
	for i, s := range items {
		s.expiry = int64(i)
	}

	newV1Expiry := func() *v1.SortCache[*server, string] {
		c := v1.New(v1.Config[*server, string]{Indexes: map[string]func(*server) string{
			"name":   func(s *server) string { return s.name },
			"expiry": func(s *server) string { return fmt.Sprintf("%020d/%s", s.expiry, s.name) },
		}})
		for _, s := range items {
			c.Put(s)
		}
		return c
	}
	newV2Expiry := func() *SortCache[*server] {
		c, _, _ := newExpiryCache()
		c.Replace(slices.Values(items))
		return c
	}

	b.Run("v1/stable", func(b *testing.B) {
		c := newV1Expiry()
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			it := items[i%n]
			c.Put(&server{name: it.name, expiry: it.expiry})
			i++
		}
	})
	b.Run("v2/stable", func(b *testing.B) {
		c := newV2Expiry()
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			it := items[i%n]
			c.Put(&server{name: it.name, expiry: it.expiry})
			i++
		}
	})
	b.Run("v1/moved", func(b *testing.B) {
		c := newV1Expiry()
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			it := items[i%n]
			c.Put(&server{name: it.name, expiry: int64(n + i)})
			i++
		}
	})
	b.Run("v2/moved", func(b *testing.B) {
		c := newV2Expiry()
		b.ReportAllocs()
		i := 0
		for b.Loop() {
			it := items[i%n]
			c.Put(&server{name: it.name, expiry: int64(n + i)})
			i++
		}
	})
	b.Run("v2/stable-batch100", func(b *testing.B) {
		c := newV2Expiry()
		b.ReportAllocs()
		i := 0
		batch := make([]*server, 100)
		for b.Loop() {
			for j := range batch {
				it := items[(i+j)%n]
				batch[j] = &server{name: it.name, expiry: it.expiry}
			}
			c.Put(batch...)
			i += 100
		}
	})
	b.Run("v2/moved-batch100", func(b *testing.B) {
		c := newV2Expiry()
		b.ReportAllocs()
		i := 0
		batch := make([]*server, 100)
		for b.Loop() {
			for j := range batch {
				it := items[(i+j)%n]
				batch[j] = &server{name: it.name, expiry: int64(n + i + j)}
			}
			c.Put(batch...)
			i += 100
		}
	})
}
