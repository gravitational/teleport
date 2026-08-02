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

package reconcile

import (
	"context"
	"iter"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

// monitorHarness drives a Monitor over types.Database values,
// with the desired set provided as a slice of views and the current set held
// in a plain map standing in for an agent's registration ledger.
type monitorHarness struct {
	current map[string]types.Database
	desired []types.Database

	materialized int
	onCreates    []string
	onUpdates    []string
	onDeletes    []string
}

func (h *monitorHarness) reconciler(t *testing.T, matcher func(types.Database) bool) *Monitor[string, types.Database, types.Database] {
	t.Helper()
	r, err := New(Config[string, types.Database, types.Database]{
		Matcher: matcher,
		GetCurrent: func(name string) (types.Database, bool) {
			db, ok := h.current[name]
			return db, ok
		},
		RangeCurrent: func() iter.Seq2[string, types.Database] {
			return func(yield func(string, types.Database) bool) {
				for name, db := range h.current {
					if !yield(name, db) {
						return
					}
				}
			}
		},
		RangeDesired: func() iter.Seq2[types.Database, error] {
			return func(yield func(types.Database, error) bool) {
				for _, db := range h.desired {
					if !yield(db, nil) {
						return
					}
				}
			}
		},
		KeyOf: types.Database.GetName,
		Materialize: func(v types.Database) types.Database {
			h.materialized++
			return v.Copy()
		},
		CompareWithCurrent: func(current, view types.Database) int {
			if current.IsEqual(view) {
				return Equal
			}
			return Different
		},
		OnCreate: func(_ context.Context, db types.Database) error {
			h.onCreates = append(h.onCreates, db.GetName())
			h.current[db.GetName()] = db
			return nil
		},
		OnUpdate: func(_ context.Context, db, _ types.Database) error {
			h.onUpdates = append(h.onUpdates, db.GetName())
			h.current[db.GetName()] = db
			return nil
		},
		OnDelete: func(_ context.Context, db types.Database) error {
			h.onDeletes = append(h.onDeletes, db.GetName())
			delete(h.current, db.GetName())
			return nil
		},
	})
	require.NoError(t, err)
	return r
}

func (h *monitorHarness) resetCalls() {
	h.materialized = 0
	h.onCreates = nil
	h.onUpdates = nil
	h.onDeletes = nil
}

func monitorTestDB(t *testing.T, name, uri string, labels map[string]string) *types.DatabaseV3 {
	t.Helper()
	db, err := types.NewDatabaseV3(types.Metadata{
		Name:   name,
		Labels: labels,
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      uri,
	})
	require.NoError(t, err)
	return db
}

func TestMonitor(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	matchAll := func(types.Database) bool { return true }
	matchLabel := func(db types.Database) bool { return db.GetAllLabels()["env"] == "prod" }

	t.Run("create, no-op, update, delete", func(t *testing.T) {
		h := &monitorHarness{current: make(map[string]types.Database)}
		r := h.reconciler(t, matchAll)

		// Create: a new matching resource is materialized exactly once.
		h.desired = []types.Database{monitorTestDB(t, "db1", "localhost:5432", nil)}
		require.NoError(t, r.Reconcile(ctx))
		require.Equal(t, []string{"db1"}, h.onCreates)
		require.Equal(t, 1, h.materialized)

		// No-op: an unchanged resource is neither materialized nor updated.
		h.resetCalls()
		require.NoError(t, r.Reconcile(ctx))
		require.Empty(t, h.onCreates)
		require.Empty(t, h.onUpdates)
		require.Empty(t, h.onDeletes)
		require.Zero(t, h.materialized)

		// Update: a changed resource is materialized and updated.
		h.resetCalls()
		h.desired = []types.Database{monitorTestDB(t, "db1", "otherhost:5432", nil)}
		require.NoError(t, r.Reconcile(ctx))
		require.Equal(t, []string{"db1"}, h.onUpdates)
		require.Equal(t, 1, h.materialized)

		// Delete: a resource absent from the desired set is unregistered.
		h.resetCalls()
		h.desired = nil
		require.NoError(t, r.Reconcile(ctx))
		require.Equal(t, []string{"db1"}, h.onDeletes)
		require.Zero(t, h.materialized)
		require.Empty(t, h.current)
	})

	t.Run("matcher gates creation", func(t *testing.T) {
		h := &monitorHarness{current: make(map[string]types.Database)}
		r := h.reconciler(t, matchLabel)

		h.desired = []types.Database{
			monitorTestDB(t, "prod-db", "prod:5432", map[string]string{"env": "prod"}),
			monitorTestDB(t, "dev-db", "dev:5432", map[string]string{"env": "dev"}),
		}
		require.NoError(t, r.Reconcile(ctx))
		require.Equal(t, []string{"prod-db"}, h.onCreates)
	})

	t.Run("registered resource that stops matching is deleted", func(t *testing.T) {
		h := &monitorHarness{current: make(map[string]types.Database)}
		r := h.reconciler(t, matchLabel)

		h.desired = []types.Database{monitorTestDB(t, "db1", "x:5432", map[string]string{"env": "prod"})}
		require.NoError(t, r.Reconcile(ctx))
		require.Equal(t, []string{"db1"}, h.onCreates)

		// The resource changes AND stops matching: it must be deleted.
		h.resetCalls()
		h.desired = []types.Database{monitorTestDB(t, "db1", "y:5432", map[string]string{"env": "dev"})}
		require.NoError(t, r.Reconcile(ctx))
		require.Empty(t, h.onUpdates)
		require.Equal(t, []string{"db1"}, h.onDeletes)
	})

	t.Run("first occurrence of a key wins", func(t *testing.T) {
		h := &monitorHarness{current: make(map[string]types.Database)}
		r := h.reconciler(t, matchAll)

		h.desired = []types.Database{
			monitorTestDB(t, "db1", "precedence-winner:5432", nil),
			monitorTestDB(t, "db1", "precedence-loser:5432", nil),
		}
		require.NoError(t, r.Reconcile(ctx))
		require.Equal(t, []string{"db1"}, h.onCreates)
		require.Equal(t, "precedence-winner:5432", h.current["db1"].GetURI())
	})

	t.Run("origin changes are not applied by default", func(t *testing.T) {
		h := &monitorHarness{current: make(map[string]types.Database)}
		r := h.reconciler(t, matchAll)

		registered := monitorTestDB(t, "db1", "x:5432", map[string]string{types.OriginLabel: types.OriginConfigFile})
		h.current["db1"] = registered

		h.desired = []types.Database{monitorTestDB(t, "db1", "y:5432", map[string]string{types.OriginLabel: types.OriginDynamic})}
		require.NoError(t, r.Reconcile(ctx))
		require.Empty(t, h.onUpdates)
		require.Empty(t, h.onDeletes)
		require.Same(t, registered, h.current["db1"].(*types.DatabaseV3))
	})

	t.Run("run loop", func(t *testing.T) {
		// current is written by reconcile callbacks on the Run goroutine and
		// asserted from the test goroutine; the mutex provides the ordering.
		var mu sync.Mutex
		current := make(map[string]types.Database)
		var desired []types.Database
		setDesired := func(dbs ...types.Database) {
			mu.Lock()
			defer mu.Unlock()
			desired = dbs
		}
		currentHas := func(name string) bool {
			mu.Lock()
			defer mu.Unlock()
			_, ok := current[name]
			return ok
		}

		// pulse mimics a collection change pulse: armed on demand, fired by
		// closing; each fire requires re-arming.
		var armed chan struct{}
		arm := func() <-chan struct{} {
			mu.Lock()
			defer mu.Unlock()
			if armed == nil {
				armed = make(chan struct{})
			}
			return armed
		}
		fire := func() {
			mu.Lock()
			defer mu.Unlock()
			if armed != nil {
				close(armed)
				armed = nil
			}
		}

		// Two-phase handshake: at the start of every cycle (inside
		// RangeDesired, after capturing the desired set) Run announces on
		// cycleStarted and then blocks on cycleRelease. Between the two the
		// test has a deterministic window: the previous cycle is fully
		// complete, no callbacks are running, and no wake can be consumed.
		cycleStarted := make(chan struct{})
		cycleRelease := make(chan struct{})

		r, err := New(Config[string, types.Database, types.Database]{
			Matcher: matchAll,
			GetCurrent: func(name string) (types.Database, bool) {
				mu.Lock()
				defer mu.Unlock()
				db, ok := current[name]
				return db, ok
			},
			RangeCurrent: func() iter.Seq2[string, types.Database] {
				return func(yield func(string, types.Database) bool) {
					mu.Lock()
					defer mu.Unlock()
					for name, db := range current {
						if !yield(name, db) {
							return
						}
					}
				}
			},
			RangeDesired: func() iter.Seq2[types.Database, error] {
				return func(yield func(types.Database, error) bool) {
					mu.Lock()
					dbs := desired
					mu.Unlock()
					cycleStarted <- struct{}{}
					<-cycleRelease
					for _, db := range dbs {
						if !yield(db, nil) {
							return
						}
					}
				}
			},
			KeyOf:       types.Database.GetName,
			Materialize: func(v types.Database) types.Database { return v.Copy() },
			CompareWithCurrent: func(current, view types.Database) int {
				if current.IsEqual(view) {
					return Equal
				}
				return Different
			},
			OnCreate: func(_ context.Context, db types.Database) error {
				mu.Lock()
				defer mu.Unlock()
				current[db.GetName()] = db
				return nil
			},
			OnUpdate: func(_ context.Context, db, _ types.Database) error {
				mu.Lock()
				defer mu.Unlock()
				current[db.GetName()] = db
				return nil
			},
			OnDelete: func(_ context.Context, db types.Database) error {
				mu.Lock()
				defer mu.Unlock()
				delete(current, db.GetName())
				return nil
			},
			Changes: arm,
		})
		require.NoError(t, err)

		runCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		done := make(chan struct{})
		go func() {
			defer close(done)
			r.Run(runCtx)
		}()

		waitCycleStart := func(what string) {
			t.Helper()
			select {
			case <-cycleStarted:
			case <-time.After(10 * time.Second):
				t.Fatalf("timed out waiting for %s to start", what)
			}
		}
		release := func() {
			t.Helper()
			select {
			case cycleRelease <- struct{}{}:
			case <-time.After(10 * time.Second):
				t.Fatal("timed out releasing a parked cycle")
			}
		}

		// The first cycle starts immediately, without any wake; its
		// completion closes Initialized.
		waitCycleStart("cycle 1")
		release()
		select {
		case <-r.Initialized():
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for Initialized")
		}

		// A pulse fire wakes a follow-up cycle observing the new desired
		// state.
		setDesired(monitorTestDB(t, "db1", "x:5432", nil))
		fire()
		waitCycleStart("cycle 2")
		release() // cycle 2 registers db1

		// Wake cycle 3 with an emptied desired set, but keep it parked:
		// inside the window, cycle 2 is provably complete and Triggers
		// cannot be consumed one-by-one.
		setDesired()
		fire()
		waitCycleStart("cycle 3")
		require.True(t, currentHas("db1"), "cycle 2 must have registered db1")
		r.Trigger()
		r.Trigger()
		r.Trigger() // coalesce into exactly one pending wake
		release()   // cycle 3 deletes db1

		// The pending trigger drives exactly one further no-op cycle.
		waitCycleStart("cycle 4 (trigger-driven)")
		require.False(t, currentHas("db1"), "cycle 3 must have deleted db1")
		release()

		// No fifth cycle: the trigger burst coalesced.
		select {
		case <-cycleStarted:
			t.Fatal("coalesced triggers must produce at most one extra cycle")
		case <-time.After(100 * time.Millisecond):
		}

		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for Run to exit")
		}
	})

	t.Run("desired iterator error aborts the cycle", func(t *testing.T) {
		h := &monitorHarness{current: make(map[string]types.Database)}
		r, err := New(Config[string, types.Database, types.Database]{
			Matcher:    matchAll,
			GetCurrent: func(name string) (types.Database, bool) { db, ok := h.current[name]; return db, ok },
			RangeCurrent: func() iter.Seq2[string, types.Database] {
				return func(yield func(string, types.Database) bool) {
					for name, db := range h.current {
						if !yield(name, db) {
							return
						}
					}
				}
			},
			RangeDesired: func() iter.Seq2[types.Database, error] {
				return func(yield func(types.Database, error) bool) {
					yield(nil, trace.Errorf("upstream unavailable"))
				}
			},
			KeyOf:              types.Database.GetName,
			Materialize:        func(v types.Database) types.Database { return v.Copy() },
			CompareWithCurrent: func(current, view types.Database) int { return Different },
			OnCreate:           func(context.Context, types.Database) error { return nil },
			OnUpdate:           func(_ context.Context, _, _ types.Database) error { return nil },
			OnDelete:           func(context.Context, types.Database) error { return nil },
		})
		require.NoError(t, err)

		// The existing registration must NOT be deleted when the desired set
		// could not be read.
		h.current["db1"] = monitorTestDB(t, "db1", "x:5432", nil)
		require.Error(t, r.Reconcile(ctx))
		require.Contains(t, h.current, "db1")
	})
}
