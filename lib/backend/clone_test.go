// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package backend_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

func TestClone(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	src, err := memory.New(memory.Config{})
	require.NoError(t, err)
	defer src.Close()

	dst, err := memory.New(memory.Config{})
	require.NoError(t, err)
	defer dst.Close()

	const itemCount = 11111
	items := make([]backend.Item, itemCount)

	for i := 0; i < itemCount; i++ {
		item := backend.Item{
			Key:   backend.NewKey(fmt.Sprintf("key-%05d", i)),
			Value: []byte(fmt.Sprintf("value-%d", i)),
		}
		_, err := src.Put(ctx, item)
		require.NoError(t, err)
		items[i] = item
	}

	err = backend.Clone(ctx, src, dst, 10, false)
	require.NoError(t, err)

	start := backend.NewKey("")
	result, err := stream.Collect(dst.Items(ctx, backend.ItemsParams{StartKey: start, EndKey: backend.RangeEnd(start)}))
	require.NoError(t, err)

	assert.Len(t, result, itemCount)
	diff := cmp.Diff(items, result, cmpopts.IgnoreFields(backend.Item{}, "Revision"), cmp.AllowUnexported(backend.Key{}))
	require.Empty(t, diff)
}

type failingBackendConfig struct {
	failedItemsAttempts     int
	failAfterItemNumber     int
	failAfterItemNumberOnce bool
	failedPutAttempts       map[string]*atomic.Int32
	b                       *memory.Memory
}

type failingBackend struct {
	failedItemsAttempts     int32
	failAfterItemNumber     int
	failAfterItemNumberOnce bool
	itemsAttempts           atomic.Int32
	failedPutAttempts       map[string]*atomic.Int32
	*memory.Memory
}

func newFailingBackend(cfg failingBackendConfig) (*failingBackend, error) {
	bk := cfg.b
	if cfg.b == nil {
		mem, err := memory.New(memory.Config{})
		if err != nil {
			return nil, err
		}
		bk = mem
	}

	return &failingBackend{
		failedItemsAttempts:     int32(cfg.failedItemsAttempts),
		failAfterItemNumber:     cfg.failAfterItemNumber,
		failAfterItemNumberOnce: cfg.failAfterItemNumberOnce,
		failedPutAttempts:       cfg.failedPutAttempts,
		Memory:                  bk,
	}, nil
}

func (f *failingBackend) Put(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	counter, ok := f.failedPutAttempts[i.Key.String()]
	if ok && counter.Add(-1) >= 0 {
		return nil, fmt.Errorf("failed to put item %s", i.Key)
	}

	return f.Memory.Put(ctx, i)
}

func (f *failingBackend) Items(ctx context.Context, params backend.ItemsParams) iter.Seq2[backend.Item, error] {
	return func(yield func(backend.Item, error) bool) {
		attempts := f.itemsAttempts.Add(1)
		if f.failedItemsAttempts > 0 && attempts <= f.failedItemsAttempts {
			yield(backend.Item{}, errors.New("failed to get items"))
			return
		}

		count := f.failAfterItemNumber
		for i, err := range f.Memory.Items(ctx, params) {
			if err != nil {
				yield(backend.Item{}, err)
				return
			}

			if f.failAfterItemNumber > 0 && count == 0 {
				if !f.failAfterItemNumberOnce || (f.failAfterItemNumberOnce && attempts == 1) {
					yield(backend.Item{}, fmt.Errorf("failed to get item %s", i.Key))
					return
				}
			}

			if !yield(i, nil) {
				return
			}
			count--
		}
	}
}

func TestCloneResiliency(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	const itemCount = 11111

	newAtomic := func(value int32) *atomic.Int32 {
		i := &atomic.Int32{}
		i.Add(value)
		return i
	}

	populatedBackend, err := memory.New(memory.Config{})
	require.NoError(t, err)
	defer populatedBackend.Close()

	items := make([]backend.Item, itemCount)

	for i := range itemCount {
		item := backend.Item{
			Key:   backend.NewKey(fmt.Sprintf("key-%05d", i)),
			Value: fmt.Appendf(nil, "value-%d", i),
		}
		_, err := populatedBackend.Put(ctx, item)
		require.NoError(t, err)
		items[i] = item
	}

	tests := []struct {
		name      string
		dstConfig failingBackendConfig
		srcConfig failingBackendConfig
		assertion func(t *testing.T, cloneError error, sourceItems, destinationItems []backend.Item)
	}{
		{
			name: "cloning completes with transient destination errors",
			srcConfig: failingBackendConfig{
				b: populatedBackend,
			},
			dstConfig: failingBackendConfig{
				failedItemsAttempts: 1,
				failAfterItemNumber: 133,
				failedPutAttempts: map[string]*atomic.Int32{
					"/key-00000": newAtomic(2),
				},
			},
			assertion: func(t *testing.T, cloneError error, sourceItems, destinationItems []backend.Item) {
				require.NoError(t, cloneError)
				assert.Len(t, destinationItems, itemCount)
				diff := cmp.Diff(sourceItems, destinationItems, cmpopts.IgnoreFields(backend.Item{}, "Revision"), cmp.AllowUnexported(backend.Key{}))
				require.Empty(t, diff)
			},
		},
		{
			name: "cloning fails with persistent destination errors",
			srcConfig: failingBackendConfig{
				b: populatedBackend,
			},
			dstConfig: failingBackendConfig{
				failedPutAttempts: map[string]*atomic.Int32{
					"/key-00000": newAtomic(3),
				},
			},
			assertion: func(t *testing.T, cloneError error, sourceItems, destinationItems []backend.Item) {
				assert.Error(t, cloneError)
				assert.True(t, trace.IsLimitExceeded(cloneError))
				require.Len(t, destinationItems, itemCount-1)
				diff := cmp.Diff(sourceItems[1:], destinationItems, cmpopts.IgnoreFields(backend.Item{}, "Revision"), cmp.AllowUnexported(backend.Key{}))
				require.Empty(t, diff)
			},
		},
		{
			name: "cloning completes with transient source errors",
			srcConfig: failingBackendConfig{
				failAfterItemNumber: itemCount / 2,
				b:                   populatedBackend,
			},
			assertion: func(t *testing.T, cloneError error, sourceItems, destinationItems []backend.Item) {
				require.NoError(t, cloneError)
				assert.Len(t, destinationItems, itemCount)
				diff := cmp.Diff(sourceItems, destinationItems, cmpopts.IgnoreFields(backend.Item{}, "Revision"), cmp.AllowUnexported(backend.Key{}))
				require.Empty(t, diff)
			},
		},
		{
			name: "cloning completes with transient source and destination errors",
			dstConfig: failingBackendConfig{
				failedPutAttempts: map[string]*atomic.Int32{
					"/key-00000": newAtomic(2),
				},
			},
			srcConfig: failingBackendConfig{
				failedItemsAttempts: 2,
				b:                   populatedBackend,
			},
			assertion: func(t *testing.T, cloneError error, sourceItems, destinationItems []backend.Item) {
				require.NoError(t, cloneError)
				assert.Len(t, destinationItems, itemCount)
				diff := cmp.Diff(sourceItems, destinationItems, cmpopts.IgnoreFields(backend.Item{}, "Revision"), cmp.AllowUnexported(backend.Key{}))
				require.Empty(t, diff)
			},
		},
		{
			name: "cloning fails with persistent source errors",
			srcConfig: failingBackendConfig{
				failAfterItemNumber: 10,
				failedItemsAttempts: 2,
				b:                   populatedBackend,
			},
			assertion: func(t *testing.T, cloneError error, sourceItems, destinationItems []backend.Item) {
				assert.Error(t, cloneError)
				assert.ErrorContains(t, cloneError, "failed to get item /key-00010")
				require.Len(t, destinationItems, 10)
				diff := cmp.Diff(sourceItems[:10], destinationItems, cmpopts.IgnoreFields(backend.Item{}, "Revision"), cmp.AllowUnexported(backend.Key{}))
				require.Empty(t, diff)
			},
		},
		{
			name: "cloning fails with unavailable source",
			srcConfig: failingBackendConfig{
				failedItemsAttempts: 10,
				b:                   populatedBackend,
			},
			assertion: func(t *testing.T, cloneError error, sourceItems, destinationItems []backend.Item) {
				assert.Error(t, cloneError)
				assert.ErrorContains(t, cloneError, "failed to get items")
				assert.Empty(t, destinationItems)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dst, err := newFailingBackend(test.dstConfig)
			require.NoError(t, err)

			defer dst.Close()

			src, err := newFailingBackend(test.srcConfig)
			require.NoError(t, err)

			cloneError := backend.Clone(ctx, src, dst, 1, false)

			start := backend.NewKey("")
			result, err := dst.GetRange(ctx, start, backend.RangeEnd(start), 0)
			require.NoError(t, err)

			test.assertion(t, cloneError, items, result.Items)
		})
	}
}

func TestCloneForce(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	src, err := memory.New(memory.Config{})
	require.NoError(t, err)
	defer src.Close()

	dst, err := memory.New(memory.Config{})
	require.NoError(t, err)
	defer dst.Close()

	const itemCount = 100
	items := make([]backend.Item, itemCount)

	for i := 0; i < itemCount; i++ {
		item := backend.Item{
			Key:   backend.NewKey(fmt.Sprintf("key-%05d", i)),
			Value: []byte(fmt.Sprintf("value-%d", i)),
		}
		_, err := src.Put(ctx, item)
		require.NoError(t, err)
		items[i] = item
	}

	_, err = dst.Put(ctx, items[0])
	require.NoError(t, err)

	err = backend.Clone(ctx, src, dst, 10, false)
	require.Error(t, err)

	err = backend.Clone(ctx, src, dst, 10, true)
	require.NoError(t, err)

	start := backend.NewKey("")
	result, err := stream.Collect(dst.Items(ctx, backend.ItemsParams{StartKey: start, EndKey: backend.RangeEnd(start)}))
	require.NoError(t, err)

	assert.Len(t, result, itemCount)
	diff := cmp.Diff(items, result, cmpopts.IgnoreFields(backend.Item{}, "Revision"), cmp.AllowUnexported(backend.Key{}))
	require.Empty(t, diff)
}

func BenchmarkClone(b *testing.B) {
	slog.SetDefault(slog.New(slog.DiscardHandler))

	ctx := b.Context()
	src, err := memory.New(memory.Config{})
	require.NoError(b, err)
	defer src.Close()

	itemCount := 11111
	items := make([]backend.Item, itemCount)

	for i := range itemCount {
		item := backend.Item{
			Key:   backend.NewKey("key-" + strconv.Itoa(i)),
			Value: bytes.Repeat([]byte{1}, 100),
		}
		_, err := src.Put(ctx, item)
		require.NoError(b, err)
		items[i] = item
	}

	benchmarkClone := func() {
		dst, err := memory.New(memory.Config{})
		require.NoError(b, err)
		defer dst.Close()

		err = backend.Clone(ctx, src, dst, 500, false)
		require.NoError(b, err)

		start := backend.NewKey("")
		var clonedItems int
		for _, err := range dst.Items(ctx, backend.ItemsParams{StartKey: start, EndKey: backend.RangeEnd(start)}) {
			require.NoError(b, err)
			clonedItems++
		}
		require.Equal(b, itemCount, clonedItems)
	}

	for b.Loop() {
		benchmarkClone()
	}
}

func TestCloneReadResumption(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	populated, err := memory.New(memory.Config{})
	require.NoError(t, err)
	defer populated.Close()

	const itemCount = 4
	items := make([]backend.Item, 0, itemCount)

	fooBar1 := backend.Item{
		Key:   backend.NewKey("foo", "bar-"),
		Value: []byte{0, 1, 2, 3},
	}
	_, err = populated.Put(ctx, fooBar1)
	require.NoError(t, err)
	items = append(items, fooBar1)

	fooBar2 := backend.Item{
		Key:   backend.NewKey("foo", "bar--"),
		Value: []byte{0, 1, 2, 3},
	}
	_, err = populated.Put(ctx, fooBar2)
	require.NoError(t, err)
	items = append(items, fooBar2)

	fooBar3 := backend.Item{
		Key:   backend.NewKey("foo", "bar-0"),
		Value: []byte{0, 1, 2, 3},
	}
	_, err = populated.Put(ctx, fooBar3)
	require.NoError(t, err)
	items = append(items, fooBar3)

	fooBar4 := backend.Item{
		Key:   backend.NewKey("foo", "bar."),
		Value: []byte{0, 1, 2, 3},
	}
	_, err = populated.Put(ctx, fooBar4)
	require.NoError(t, err)
	items = append(items, fooBar4)

	src, err := newFailingBackend(failingBackendConfig{
		failAfterItemNumber:     1,
		failAfterItemNumberOnce: true,
		b:                       populated,
	})
	require.NoError(t, err)

	dst, err := memory.New(memory.Config{})
	require.NoError(t, err)
	defer dst.Close()

	err = backend.Clone(ctx, src, dst, 10, false)
	require.NoError(t, err)

	start := backend.NewKey("")
	result, err := stream.Collect(dst.Items(ctx, backend.ItemsParams{StartKey: start, EndKey: backend.RangeEnd(start)}))
	require.NoError(t, err)

	assert.Len(t, result, itemCount)
	diff := cmp.Diff(items, result, cmpopts.IgnoreFields(backend.Item{}, "Revision"), cmp.AllowUnexported(backend.Key{}))
	require.Empty(t, diff)
}
