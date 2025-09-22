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

package test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/lib/backend"
)

func RunAtomicWriteComplianceSuite(t *testing.T, newBackend Constructor) {
	t.Run("Move", func(t *testing.T) {
		testAtomicWriteMove(t, newBackend)
	})

	t.Run("Lock", func(t *testing.T) {
		testAtomicWriteLock(t, newBackend)
	})

	t.Run("Max", func(t *testing.T) {
		testAtomicWriteMax(t, newBackend)
	})

	t.Run("Concurrent", func(t *testing.T) {
		testAtomicWriteConcurrent(t, newBackend)
	})

	t.Run("NonConflicting", func(t *testing.T) {
		testAtomicWriteNonConflicting(t, newBackend)
	})

	t.Run("Other", func(t *testing.T) {
		testAtomicWriteOther(t, newBackend)
	})
}

// testAtomicWriteMove verifies the correct behavior of "move" operations.
func testAtomicWriteMove(t *testing.T, newBackend Constructor) {
	bk, _, err := newBackend()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	prefix := MakePrefix()

	fromKey, toKey, val := prefix("/src"), prefix("/dest"), []byte("val")

	lease, err := bk.Put(ctx, backend.Item{
		Key:   fromKey,
		Value: val,
	})
	require.NoError(t, err)

	// perform "move".
	_, err = bk.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			Key:       fromKey,
			Condition: backend.Revision(lease.Revision),
			Action:    backend.Delete(),
		},
		{
			Key:       toKey,
			Condition: backend.Whatever(),
			Action: backend.Put(backend.Item{
				Value: val,
			}),
		},
	})

	require.NoError(t, err)

	_, err = bk.Get(ctx, fromKey)
	require.True(t, trace.IsNotFound(err), "err: %v", err)

	item, err := bk.Get(ctx, toKey)
	require.NoError(t, err)
	require.Equal(t, val, item.Value)

	// re-attempt now outdated "move".
	_, err = bk.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			Key:       fromKey,
			Condition: backend.Revision(lease.Revision),
			Action:    backend.Delete(),
		},
		{
			Key:       toKey,
			Condition: backend.Whatever(),
			Action: backend.Put(backend.Item{
				Value: val,
			}),
		},
	})
	require.ErrorIs(t, err, backend.ErrConditionFailed)
}

// testAtomicWriteLock verifies correct behavior of various "lock" patterns (i.e. where some update on key X is conditional on
// the state of key Y).
func testAtomicWriteLock(t *testing.T, newBackend Constructor) {
	bk, _, err := newBackend()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	prefix := MakePrefix()

	itemKey, lockKey := prefix("/item"), prefix("/lock")

	// successful 'NotExists' condition.
	_, err = bk.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			Key:       lockKey,
			Condition: backend.NotExists(),
			Action:    backend.Nop(),
		},
		{
			Key:       itemKey,
			Condition: backend.Whatever(),
			Action: backend.Put(backend.Item{
				Value: []byte("i1"),
			}),
		},
	})
	require.NoError(t, err)

	firstLockLease, err := bk.Put(ctx, backend.Item{
		Key:   lockKey,
		Value: []byte("l1"),
	})
	require.NoError(t, err)

	// failing 'NotExists' condition.
	_, err = bk.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			Key:       lockKey,
			Condition: backend.NotExists(),
			Action:    backend.Nop(),
		},
		{
			Key:       itemKey,
			Condition: backend.Whatever(),
			Action: backend.Put(backend.Item{
				Value: []byte("i2"),
			}),
		},
	})
	require.ErrorIs(t, err, backend.ErrConditionFailed)

	// verify that item value matches former successful put
	item, err := bk.Get(ctx, itemKey)
	require.NoError(t, err)
	require.Equal(t, []byte("i1"), item.Value)

	// successful 'Revision' condition.
	_, err = bk.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			Key:       lockKey,
			Condition: backend.Revision(firstLockLease.Revision),
			Action:    backend.Nop(),
		},
		{
			Key:       itemKey,
			Condition: backend.Whatever(),
			Action: backend.Put(backend.Item{
				Value: []byte("i3"),
			}),
		},
	})
	require.NoError(t, err)

	// update the lock
	_, err = bk.Put(ctx, backend.Item{
		Key:   lockKey,
		Value: []byte("l2"),
	})
	require.NoError(t, err)

	// unsuccessful 'Revision' condition.
	_, err = bk.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			Key:       lockKey,
			Condition: backend.Revision(firstLockLease.Revision),
			Action:    backend.Nop(),
		},
		{
			Key:       itemKey,
			Condition: backend.Whatever(),
			Action: backend.Put(backend.Item{
				Value: []byte("i4"),
			}),
		},
	})
	require.ErrorIs(t, err, backend.ErrConditionFailed)

	// verify that item value matches former successful put
	item, err = bk.Get(ctx, itemKey)
	require.NoError(t, err)
	require.Equal(t, []byte("i3"), item.Value)

	// delete the lock in prep for NotExists case
	err = bk.Delete(ctx, lockKey)
	require.NoError(t, err)

	// successful 'NotExists' condition.
	_, err = bk.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			Key:       lockKey,
			Condition: backend.NotExists(),
			Action:    backend.Nop(),
		},
		{
			Key:       itemKey,
			Condition: backend.Whatever(),
			Action: backend.Put(backend.Item{
				Value: []byte("i5"),
			}),
		},
	})
	require.NoError(t, err)

	// recreate the lock
	_, err = bk.Put(ctx, backend.Item{
		Key:   lockKey,
		Value: []byte("l3"),
	})
	require.NoError(t, err)

	// unsuccessful 'NotExists' condition.
	_, err = bk.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			Key:       lockKey,
			Condition: backend.NotExists(),
			Action:    backend.Nop(),
		},
		{
			Key:       itemKey,
			Condition: backend.Whatever(),
			Action: backend.Put(backend.Item{
				Value: []byte("i6"),
			}),
		},
	})
	require.ErrorIs(t, err, backend.ErrConditionFailed)

	// verify that item value matches former successful put
	item, err = bk.Get(ctx, itemKey)
	require.NoError(t, err)
	require.Equal(t, []byte("i5"), item.Value)
}

// testAtomicWriteMax verifies correct behavior of very large atomic writes.
func testAtomicWriteMax(t *testing.T, newBackend Constructor) {
	bk, _, err := newBackend()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	prefix := MakePrefix()

	keyOf := func(i int) backend.Key {
		return prefix("key-" + strconv.Itoa(i))
	}

	var condacts []backend.ConditionalAction

	// set up one more conditional actions than should be permitted
	for i := range backend.MaxAtomicWriteSize + 1 {
		condacts = append(condacts, backend.ConditionalAction{
			Key:       keyOf(i),
			Condition: backend.NotExists(),
			Action: backend.Put(backend.Item{
				Value: []byte("v1"),
			}),
		})
	}

	// atomic write should fail
	_, err = bk.AtomicWrite(ctx, condacts)
	require.Error(t, err)

	// truncate to the allowed maximum
	condacts = condacts[:backend.MaxAtomicWriteSize]

	// atomic write should now succeed
	rev1, err := bk.AtomicWrite(ctx, condacts)
	require.NoError(t, err)

	// verify that items were inserted as expected
	for i := range backend.MaxAtomicWriteSize {
		item, err := bk.Get(ctx, keyOf(i))
		require.NoError(t, err, "i=%d", i)
		require.Equal(t, rev1, item.Revision)
		require.Equal(t, []byte("v1"), item.Value)
	}

	// update puts
	for i := range condacts {
		condacts[i].Action = backend.Put(backend.Item{
			Value: []byte("v2"),
		})
	}

	// re-attempt should fail due to conditions no-longer holding
	_, err = bk.AtomicWrite(ctx, condacts)
	require.ErrorIs(t, err, backend.ErrConditionFailed)

	// verify that failed atomic write results in no changes
	for i := range backend.MaxAtomicWriteSize {
		item, err := bk.Get(ctx, keyOf(i))
		require.NoError(t, err, "i=%d", i)
		require.Equal(t, rev1, item.Revision)
		require.Equal(t, []byte("v1"), item.Value)
	}

	// update conditional actions to assert revision
	for i := range condacts {
		condacts[i].Action = backend.Put(backend.Item{
			Value: []byte("v3"),
		})
		condacts[i].Condition = backend.Revision(rev1)
	}

	// conditional actions should now succeed
	rev2, err := bk.AtomicWrite(ctx, condacts)
	require.NoError(t, err)

	// verify that changes occurred as expected
	for i := range backend.MaxAtomicWriteSize {
		item, err := bk.Get(ctx, keyOf(i))
		require.NoError(t, err, "i=%d", i)
		require.Equal(t, rev2, item.Revision)
		require.Equal(t, []byte("v3"), item.Value)
	}
}

// testAtomicWriteConcurrent is a sanity-check intended to verify the correctness of AtomicWrite under high concurrency.
func testAtomicWriteConcurrent(t *testing.T, newBackend Constructor) {
	const (
		increments = 200
		workers    = 20
	)
	bk, _, err := newBackend()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	prefix := MakePrefix()

	counterKey := prefix("/counter")

	_, err = bk.Put(ctx, backend.Item{
		Key:   counterKey,
		Value: []byte("0"),
	})
	require.NoError(t, err)

	var eg errgroup.Group
	for range workers {
		eg.Go(func() error {
			var localIncrements int

			// note that we only attempt exactly 'increments' number of times, because we expect every iteration to
			// succeed for at least one worker. this requirement only holds true if reads are *consistent*, weak reads
			// *would* result in cases where all workers failed to perform an increment because they all observed an
			// outdated state.
			for range increments {
				if localIncrements >= increments/workers {
					return nil
				}

				item, err := bk.Get(ctx, counterKey)
				if err != nil {
					// should never happen unless test is malformed or backend is offline
					return trace.Errorf("unexpected error loading counter: %v", err)
				}

				n, err := strconv.Atoi(string(item.Value))
				if err != nil {
					// should never happen unless test is malformed or backend is offline
					return trace.Errorf("invalid counter value %q: %v", item.Value, err)
				}

				n++

				_, err = bk.AtomicWrite(ctx, []backend.ConditionalAction{
					{
						Key:       counterKey,
						Condition: backend.Revision(item.Revision),
						Action: backend.Put(backend.Item{
							Value: []byte(strconv.Itoa(n)),
						}),
					},
				})

				if err != nil {
					if errors.Is(err, backend.ErrConditionFailed) {
						continue
					}

					// should never happen unless test is malformed or backend is offline
					return trace.Errorf("unexpected error writing counter: %v", err)
				}

				localIncrements++
			}

			if localIncrements < increments/workers {
				// should never happen unless test is malformed or backend is offline
				return trace.Errorf("worker halted with %d/%d local increments (this is a bug)", localIncrements, increments/workers)
			}

			return nil
		})
	}

	require.NoError(t, eg.Wait())

	counterItem, err := bk.Get(ctx, counterKey)
	require.NoError(t, err)

	n, err := strconv.Atoi(string(counterItem.Value))
	require.NoError(t, err)
	require.Equal(t, increments, n)
}

// testAtomicWriteNonConflicting verifies that non-conflicting but overlapping transactions all succeed
// on the first attempt when running concurrently, meaning that backends that treat overlap as conflict (e.g. dynamo)
// handle such conflicts internally.
func testAtomicWriteNonConflicting(t *testing.T, newBackend Constructor) {
	const workers = 60

	bk, _, err := newBackend()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	prefix := MakePrefix()

	results := make(chan error, workers)

	commonKey := prefix("common")

	itemKey := func(i int) backend.Key {
		return prefix("item-" + strconv.Itoa(i))
	}

	_, err = bk.Put(ctx, backend.Item{
		Key:   commonKey,
		Value: []byte("c1"),
	})
	require.NoError(t, err)

	for i := range workers {
		go func(i int) {
			_, err := bk.AtomicWrite(ctx, []backend.ConditionalAction{
				{
					Key:       commonKey,
					Condition: backend.Exists(),
					Action:    backend.Nop(),
				},
				{
					Key:       itemKey(i),
					Condition: backend.Whatever(),
					Action: backend.Put(backend.Item{
						Value: []byte("v1"),
					}),
				},
			})

			results <- err
		}(i)
	}

	timeout := time.After(time.Minute)

	for i := range workers {
		select {
		case err := <-results:
			require.NoError(t, err, trace.DebugReport(err))
		case <-timeout:
			require.FailNow(t, "timeout waiting for workers to finish", "iteration=%d", i)
		}
	}

	for i := range workers {
		item, err := bk.Get(ctx, itemKey(i))
		require.NoError(t, err)
		require.Equal(t, []byte("v1"), item.Value)
	}
}

// testAtomicWriteOther verifies some minor edge-cases that may not be covered by other tests. Specifically,
// it verifies that Item.Key has no effect on writes or subsequent reads, and that ineffectual writes still
// update the value of revision.
func testAtomicWriteOther(t *testing.T, newBackend Constructor) {
	bk, _, err := newBackend()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	prefix := MakePrefix()

	fooKey, barKey, badKey := prefix("foo"), prefix("bar"), prefix("bad")

	fooVal, barVal := []byte("foo"), []byte("bar")

	// set initial values. we include incorrect keys in the items passed to Put in
	// order to verify that those keys are ignored as intended.
	rev1, err := bk.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			Key:       fooKey,
			Condition: backend.Whatever(),
			Action: backend.Put(backend.Item{
				Key:   badKey, // should be ignored
				Value: fooVal,
			}),
		},
		{
			Key:       barKey,
			Condition: backend.Whatever(),
			Action: backend.Put(backend.Item{
				Key:   badKey, // should be ignored
				Value: barVal,
			}),
		},
	})
	require.NoError(t, err)

	fooItem, err := bk.Get(ctx, fooKey)
	require.NoError(t, err)
	require.Equal(t, fooKey, fooItem.Key)
	require.Equal(t, fooVal, fooItem.Value)
	require.Equal(t, rev1, fooItem.Revision)

	barItem, err := bk.Get(ctx, barKey)
	require.NoError(t, err)
	require.Equal(t, barKey, barItem.Key)
	require.Equal(t, barVal, barItem.Value)
	require.Equal(t, rev1, barItem.Revision)

	// ensure that the key passed to item didn't cause anything to be written
	// to that key.
	_, err = bk.Get(ctx, badKey)
	require.True(t, trace.IsNotFound(err), "err: %v", err)

	// re-write the same values again to verify that revision is changed even when values are not.
	rev2, err := bk.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			Key:       fooKey,
			Condition: backend.Whatever(),
			Action: backend.Put(backend.Item{
				Key:   badKey, // should be ignored
				Value: fooVal,
			}),
		},
		{
			Key:       barKey,
			Condition: backend.Whatever(),
			Action: backend.Put(backend.Item{
				Key:   badKey, // should be ignored
				Value: barVal,
			}),
		},
	})
	require.NoError(t, err)

	fooItem, err = bk.Get(ctx, fooKey)
	require.NoError(t, err)
	require.Equal(t, fooVal, fooItem.Value)
	require.Equal(t, rev2, fooItem.Revision)

	barItem, err = bk.Get(ctx, barKey)
	require.NoError(t, err)
	require.Equal(t, barVal, barItem.Value)
	require.Equal(t, rev2, barItem.Revision)

	// perform partially-redundant write to ensure that revision is also changed for all items in that case.
	rev3, err := bk.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			Key:       fooKey,
			Condition: backend.Whatever(),
			Action: backend.Put(backend.Item{
				Value: fooVal,
			}),
		},
		{
			Key:       barKey,
			Condition: backend.Whatever(),
			Action: backend.Put(backend.Item{
				Value: []byte("something-else"),
			}),
		},
	})
	require.NoError(t, err)

	fooItem, err = bk.Get(ctx, fooKey)
	require.NoError(t, err)
	require.Equal(t, fooVal, fooItem.Value)
	require.Equal(t, rev3, fooItem.Revision)

	barItem, err = bk.Get(ctx, barKey)
	require.NoError(t, err)
	require.Equal(t, []byte("something-else"), barItem.Value)
	require.Equal(t, rev3, barItem.Revision)

	// mixed put and delete case
	rev4, err := bk.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			Key:       fooKey,
			Condition: backend.Whatever(),
			Action: backend.Put(backend.Item{
				Value: fooVal,
			}),
		},
		{
			Key:       barKey,
			Condition: backend.Whatever(),
			Action:    backend.Delete(),
		},
	})
	require.NoError(t, err)

	fooItem, err = bk.Get(ctx, fooKey)
	require.NoError(t, err)
	require.Equal(t, fooVal, fooItem.Value)
	require.Equal(t, rev4, fooItem.Revision)

	_, err = bk.Get(ctx, barKey)
	require.True(t, trace.IsNotFound(err), "err: %v", err)

	// mixed put and condition case
	rev5, err := bk.AtomicWrite(ctx, []backend.ConditionalAction{
		{
			Key:       fooKey,
			Condition: backend.Whatever(),
			Action: backend.Put(backend.Item{
				Value: fooVal,
			}),
		},
		{
			Key:       barKey,
			Condition: backend.NotExists(),
			Action:    backend.Nop(),
		},
	})
	require.NoError(t, err)

	fooItem, err = bk.Get(ctx, fooKey)
	require.NoError(t, err)
	require.Equal(t, fooVal, fooItem.Value)
	require.Equal(t, rev5, fooItem.Revision)

	_, err = bk.Get(ctx, barKey)
	require.True(t, trace.IsNotFound(err), "err: %v", err)
}
