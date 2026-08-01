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
	"math/rand"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	v1 "github.com/gravitational/teleport/lib/utils/sortcache"
)

// server is the test resource. Pointer-typed T, like all production uses.
type server struct {
	name   string
	host   string
	expiry int64
	v      int
}

// newExpiryCache builds a cache with a name primary index and an
// expiry-ordered secondary index — the "moved key" shape: every update that
// extends an item's expiry moves its key on the secondary index.
func newExpiryCache() (*SortCache[*server], *Index[*server, string], *Index[*server, Tuple2[int64, string]]) {
	nameIdx := NewIndex("name", func(s *server) string { return s.name })
	expiryIdx := NewTupleIndex2("expiry", func(s *server) (int64, string) { return s.expiry, s.name })
	c := New(Config[*server]{Indexes: []AnyIndex[*server]{nameIdx, expiryIdx}})
	return c, nameIdx, expiryIdx
}

func newCache() (*SortCache[*server], *Index[*server, string], *Index[*server, Tuple2[string, string]]) {
	nameIdx := NewIndex("name", func(s *server) string { return s.name })
	hostIdx := NewTupleIndex2("host", func(s *server) (string, string) { return s.host, s.name })
	c := New(Config[*server]{Indexes: []AnyIndex[*server]{nameIdx, hostIdx}})
	return c, nameIdx, hostIdx
}

func collect[T any](seq func(func(T) bool)) []T {
	var out []T
	seq(func(t T) bool {
		out = append(out, t)
		return true
	})
	return out
}

func TestBasics(t *testing.T) {
	t.Parallel()

	c, nameIdx, hostIdx := newCache()

	rscs := []*server{
		{name: "001", host: "node"},
		{name: "002", host: "node"},
		{name: "003", host: "kube"},
		{name: "004", host: "kube"},
	}
	for _, r := range rscs {
		require.Empty(t, c.Put(r))
	}
	require.Equal(t, 4, c.Len())

	snap := c.Snapshot()

	r, ok := nameIdx.Get(snap, "002")
	require.True(t, ok)
	require.Same(t, rscs[1], r)

	r, ok = hostIdx.Get(snap, T2("kube", "003"))
	require.True(t, ok)
	require.Same(t, rscs[2], r)

	_, ok = nameIdx.Get(snap, "005")
	require.False(t, ok)

	// update: replaces the value on all indexes, evicting the old version.
	updated := &server{name: "002", host: "node", v: 1}
	evicted := c.Put(updated)
	require.Len(t, evicted, 1)
	require.Same(t, rscs[1], evicted[0])
	require.Equal(t, 4, c.Len())

	r, ok = nameIdx.Get(c.Snapshot(), "002")
	require.True(t, ok)
	require.Same(t, updated, r)

	// the old snapshot still sees the old value.
	r, ok = nameIdx.Get(snap, "002")
	require.True(t, ok)
	require.Same(t, rscs[1], r)

	// delete removes from all indexes.
	deleted := c.Delete(&server{name: "003"})
	require.Len(t, deleted, 1)
	require.Same(t, rscs[2], deleted[0])
	require.Equal(t, 3, c.Len())
	_, ok = hostIdx.Get(c.Snapshot(), T2("kube", "003"))
	require.False(t, ok)

	// deleting a nonexistent value is a no-op.
	require.Empty(t, c.Delete(&server{name: "nope"}))
	require.Equal(t, 3, c.Len())

	c.Clear()
	require.Equal(t, 0, c.Len())
	require.Equal(t, 4, snap.Len(), "old snapshot must be unaffected by later writes and Clear")
}

func TestCrossValueEviction(t *testing.T) {
	t.Parallel()

	// an index that deliberately violates the unique-suffix discipline.
	nameIdx := NewIndex("name", func(s *server) string { return s.name })
	vIdx := NewIndex("v", func(s *server) int { return s.v })
	c := New(Config[*server]{Indexes: []AnyIndex[*server]{nameIdx, vIdx}})

	a := &server{name: "a", v: 7}
	b := &server{name: "b", v: 7} // collides with a on vIdx only
	require.Empty(t, c.Put(a))

	evicted := c.Put(b)
	require.Len(t, evicted, 1)
	require.Same(t, a, evicted[0])
	require.Equal(t, 1, c.Len())

	// a must be fully gone from all indexes.
	_, ok := nameIdx.Get(c.Snapshot(), "a")
	require.False(t, ok)
}

func TestRangesAndBounds(t *testing.T) {
	t.Parallel()

	c, nameIdx, hostIdx := newCache()
	for i := range 100 {
		c.Put(&server{name: fmt.Sprintf("%03d", i), host: fmt.Sprintf("h%d", i%4)})
	}
	snap := c.Snapshot()

	names := func(seq func(func(*server) bool)) []string {
		var out []string
		for _, s := range collect(seq) {
			out = append(out, s.name)
		}
		return out
	}

	// full ascending iteration is sorted.
	all := names(nameIdx.Ascend(snap, Open[string](), Open[string]()))
	require.Len(t, all, 100)
	require.True(t, slices.IsSorted(all))

	// start-inclusive / stop-exclusive matches teleport range conventions.
	got := names(nameIdx.Ascend(snap, Inclusive("010"), Exclusive("013")))
	require.Equal(t, []string{"010", "011", "012"}, got)

	// exclusive start, inclusive stop.
	got = names(nameIdx.Ascend(snap, Exclusive("010"), Inclusive("013")))
	require.Equal(t, []string{"011", "012", "013"}, got)

	// descending: start is the upper end.
	got = names(nameIdx.Descend(snap, Inclusive("013"), Exclusive("010")))
	require.Equal(t, []string{"013", "012", "011"}, got)

	got = names(nameIdx.Descend(snap, Open[string](), Open[string]()))
	require.Len(t, got, 100)
	require.True(t, slices.IsSortedFunc(got, compareDesc))

	// typed prefix range: everything on host h2, no NextKey/separator games.
	start, stop := Prefix2[string, string]("h2")
	h2 := collect(hostIdx.Ascend(snap, start, stop))
	require.Len(t, h2, 25)
	for _, s := range h2 {
		require.Equal(t, "h2", s.host)
	}

	// prefix bounds do not bleed into neighboring prefixes even when one
	// prefix is a string-prefix of another.
	c.Put(&server{name: "x1", host: "h"})
	c.Put(&server{name: "x2", host: "h22"})
	snap = c.Snapshot()
	start, stop = Prefix2[string, string]("h2")
	h2 = collect(hostIdx.Ascend(snap, start, stop))
	require.Len(t, h2, 25)
	for _, s := range h2 {
		require.Equal(t, "h2", s.host)
	}

	require.Equal(t, 25, hostIdx.Count(snap, start, stop))
}

func compareDesc(a, b string) int {
	switch {
	case a > b:
		return -1
	case a < b:
		return 1
	default:
		return 0
	}
}

// TestMovedKeyUpdate covers updates that change a value's key on a secondary
// index (e.g. a heartbeat extending an expiry): the old key must vanish, the
// new key must appear, eviction must report exactly the old version, and
// prior snapshots must retain the old ordering.
func TestMovedKeyUpdate(t *testing.T) {
	t.Parallel()

	c, nameIdx, expiryIdx := newExpiryCache()

	for i := range 10 {
		c.Put(&server{name: fmt.Sprintf("s%02d", i), expiry: int64(100 + i)})
	}
	before := c.Snapshot()

	// heartbeat: s03 extends its expiry from 103 to 500, moving its key to
	// the end of the expiry index.
	old, _ := nameIdx.Get(before, "s03")
	updated := &server{name: "s03", expiry: 500}
	evicted := c.Put(updated)
	require.Len(t, evicted, 1)
	require.Same(t, old, evicted[0])
	require.Equal(t, 10, c.Len())

	snap := c.Snapshot()

	// old expiry key is gone, new one is present.
	_, ok := expiryIdx.Get(snap, T2(int64(103), "s03"))
	require.False(t, ok)
	got, ok := expiryIdx.Get(snap, T2(int64(500), "s03"))
	require.True(t, ok)
	require.Same(t, updated, got)

	// expiry ordering reflects the move: s03 is now last.
	var order []string
	for s := range expiryIdx.Ascend(snap, Open[Tuple2[int64, string]](), Open[Tuple2[int64, string]]()) {
		order = append(order, s.name)
	}
	require.Len(t, order, 10)
	require.Equal(t, "s03", order[9])
	require.Equal(t, "s04", order[3]) // s04 fills the gap where s03 was

	// the pre-update snapshot still has the old ordering and the old value.
	got, ok = expiryIdx.Get(before, T2(int64(103), "s03"))
	require.True(t, ok)
	require.Same(t, old, got)
	_, ok = expiryIdx.Get(before, T2(int64(500), "s03"))
	require.False(t, ok)

	// a range scan of "expired before 105" sees exactly the still-old items.
	expired := collect(expiryIdx.Ascend(snap, Open[Tuple2[int64, string]](), Exclusive(Tuple2Min[int64, string](105))))
	var expiredNames []string
	for _, s := range expired {
		expiredNames = append(expiredNames, s.name)
	}
	require.Equal(t, []string{"s00", "s01", "s02", "s04"}, expiredNames)
}

func TestReplace(t *testing.T) {
	t.Parallel()

	c, nameIdx, _ := newCache()
	c.Put(&server{name: "old", host: "h"})
	snap := c.Snapshot()

	fresh := make([]*server, 50)
	for i := range fresh {
		fresh[i] = &server{name: fmt.Sprintf("new-%02d", i), host: "h"}
	}
	c.Replace(slices.Values(fresh))

	require.Equal(t, 50, c.Len())
	_, ok := nameIdx.Get(c.Snapshot(), "old")
	require.False(t, ok)
	_, ok = nameIdx.Get(c.Snapshot(), "new-07")
	require.True(t, ok)

	// pre-replace snapshot is untouched.
	require.Equal(t, 1, snap.Len())
	_, ok = nameIdx.Get(snap, "old")
	require.True(t, ok)
}

// TestDifferentialVsV1 runs a randomized op sequence against both the v1
// implementation and the prototype and requires identical contents and
// iteration order on every index. Host/name charsets are restricted to
// alphanumerics so v1's "host/name" concatenated ordering agrees with the
// tuple ordering.
func TestDifferentialVsV1(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(42))

	c1 := v1.New(v1.Config[*server, string]{Indexes: map[string]func(*server) string{
		"name": func(s *server) string { return s.name },
		"host": func(s *server) string { return s.host + "/" + s.name },
	}})
	c2, nameIdx, hostIdx := newCache()

	names := make([]string, 200)
	for i := range names {
		names[i] = fmt.Sprintf("n%03d", i)
	}
	hosts := []string{"alpha", "beta", "gamma", "delta"}

	compare := func() {
		require.Equal(t, c1.Len(), c2.Len())
		snap := c2.Snapshot()

		v1Names := collect(c1.Ascend("name", "", ""))
		v2Names := collect(nameIdx.Ascend(snap, Open[string](), Open[string]()))
		require.Equal(t, v1Names, v2Names)

		v1Hosts := collect(c1.Ascend("host", "", ""))
		v2Hosts := collect(hostIdx.Ascend(snap, Open[Tuple2[string, string]](), Open[Tuple2[string, string]]()))
		require.Equal(t, v1Hosts, v2Hosts)

		v1Desc := collect(c1.Descend("name", "", ""))
		v2Desc := collect(nameIdx.Descend(snap, Open[string](), Open[string]()))
		require.Equal(t, v1Desc, v2Desc)
	}

	for step := range 5000 {
		name := names[rng.Intn(len(names))]
		switch rng.Intn(10) {
		case 0, 1, 2, 3, 4, 5, 6: // put (mix of inserts and updates)
			s := &server{name: name, host: hosts[rng.Intn(len(hosts))], v: step}
			c1.Put(s)
			c2.Put(s)
		default: // delete (often nonexistent)
			c1.Delete("name", name)
			c2.Delete(&server{name: name})
		}
		if step%500 == 0 {
			compare()
		}
	}
	compare()
}

// TestSnapshotConsistencyUnderChurn is the property v1 cannot provide: a
// snapshot taken at any moment yields an internally consistent view — a full
// iteration over any index returns exactly Snapshot.Len() values with no
// misses or duplicates — while a writer concurrently churns puts and deletes.
// Run with -race.
func TestSnapshotConsistencyUnderChurn(t *testing.T) {
	t.Parallel()

	c, nameIdx, hostIdx := newCache()
	items := make([]*server, 2000)
	for i := range items {
		items[i] = &server{name: fmt.Sprintf("s%05d", i), host: fmt.Sprintf("h%03d", i%37)}
	}
	c.Replace(slices.Values(items))

	stop := make(chan struct{})
	var wg sync.WaitGroup

	wg.Go(func() {
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
			}
			// churn: update one, occasionally delete another and re-add it
			// later via the update path.
			c.Put(&server{name: items[i%len(items)].name, host: items[(i*7)%len(items)].host, v: i})
			if i%5 == 0 {
				c.Delete(items[(i*3)%len(items)])
			}
			i++
		}
	})

	for range 4 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				snap := c.Snapshot()
				want := snap.Len()

				got := 0
				seen := make(map[string]struct{}, want)
				for s := range nameIdx.Ascend(snap, Open[string](), Open[string]()) {
					if _, dup := seen[s.name]; dup {
						t.Errorf("duplicate value %q in snapshot iteration", s.name)
						return
					}
					seen[s.name] = struct{}{}
					got++
				}
				if got != want {
					t.Errorf("name index iteration returned %d values, snapshot Len() is %d", got, want)
					return
				}

				// secondary index must agree with the primary within one snapshot.
				got = 0
				for range hostIdx.Ascend(snap, Open[Tuple2[string, string]](), Open[Tuple2[string, string]]()) {
					got++
				}
				if got != want {
					t.Errorf("host index iteration returned %d values, snapshot Len() is %d", got, want)
					return
				}
			}
		})
	}

	time.Sleep(500 * time.Millisecond)
	close(stop)
	wg.Wait()
}
