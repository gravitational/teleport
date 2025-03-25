/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package backend

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestSanitize(t *testing.T) {
	tests := []struct {
		inKey  Key
		assert require.ErrorAssertionFunc
	}{
		{
			inKey:  NewKey("a-b", "c:d", ".e_f", "01"),
			assert: require.NoError,
		},
		{
			inKey:  NewKey("namespaces", "", "params"),
			assert: require.Error,
		},
		{
			inKey:  NewKey("namespaces", ".."),
			assert: require.Error,
		},
		{
			inKey:  NewKey("namespaces", "..", "params"),
			assert: require.Error,
		},
		{
			inKey:  NewKey("namespaces", "..params"),
			assert: require.NoError,
		},
		{
			inKey:  NewKey("namespaces", "."),
			assert: require.Error,
		},
		{
			inKey:  NewKey("namespaces", ".", "params"),
			assert: require.Error,
		},
		{
			inKey:  NewKey("namespaces", ".params"),
			assert: require.NoError,
		},
		{
			inKey:  NewKey(".."),
			assert: require.Error,
		},
		{
			inKey:  NewKey("..params"),
			assert: require.NoError,
		},
		{
			inKey:  NewKey("..", "params"),
			assert: require.Error,
		},
		{
			inKey:  NewKey("."),
			assert: require.Error,
		},
		{
			inKey:  NewKey(".params"),
			assert: require.NoError,
		},
		{
			inKey:  NewKey(".", "params"),
			assert: require.Error,
		},
		{
			inKey:  RangeEnd(NewKey("a-b", "c:d", ".e_f", "01")),
			assert: require.NoError,
		},
		{
			inKey:  RangeEnd(NewKey("")),
			assert: require.NoError,
		},
		{
			inKey:  RangeEnd(NewKey("Malformed \xf0\x90\x28\xbc UTF8")),
			assert: require.Error,
		},
		{
			inKey:  NewKey("test+subaddr@example.com"),
			assert: require.NoError,
		},
		{
			inKey:  NewKey("xyz"),
			assert: require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.inKey.String(), func(t *testing.T) {
			ctx := context.Background()
			safeBackend := NewSanitizer(&nopBackend{})

			_, err := safeBackend.Get(ctx, tt.inKey)
			require.NoError(t, err)

			_, err = safeBackend.Create(ctx, Item{Key: tt.inKey})
			tt.assert(t, err)

			_, err = safeBackend.Put(ctx, Item{Key: tt.inKey})
			tt.assert(t, err)

			_, err = safeBackend.Update(ctx, Item{Key: tt.inKey})
			tt.assert(t, err)

			_, err = safeBackend.CompareAndSwap(ctx, Item{Key: tt.inKey}, Item{Key: tt.inKey})
			tt.assert(t, err)

			err = safeBackend.Delete(ctx, tt.inKey)
			require.NoError(t, err)

			err = safeBackend.DeleteRange(ctx, tt.inKey, tt.inKey)
			require.NoError(t, err)
		})
	}
}

func BenchmarkIsKeySafe(b *testing.B) {
	keys := []Key{
		NewKey("a-b", "c:d", ".e_f", "01"),
		NewKey("namespaces", "", "params"),
		NewKey("namespaces", ".."),
		NewKey("namespaces", "..", "params"),
		NewKey("namespaces", "..params"),
		NewKey("namespaces", "."),
		NewKey("namespaces", ".", "params"),
		NewKey("namespaces", ".params"),
		NewKey(".."),
		NewKey("..params"),
		NewKey("..", "params"),
		NewKey("."),
		NewKey(".params"),
		NewKey(".", "params"),
		RangeEnd(NewKey("a-b", "c:d", ".e_f", "01")),
		RangeEnd(NewKey("")),
		RangeEnd(NewKey("Malformed \xf0\x90\x28\xbc UTF8")),
		NewKey("test+subaddr@example.com"),
		NewKey("xyz"),
		NewKey("@@"),
		NewKey("@_:.-+"),
	}

	for b.Loop() {
		for _, key := range keys {
			IsKeySafe(key)
		}
	}
}

type nopBackend struct{}

func (n *nopBackend) GetName() string {
	return "nop"
}

func (n *nopBackend) Get(_ context.Context, _ Key) (*Item, error) {
	return &Item{}, nil
}

func (n *nopBackend) GetRange(_ context.Context, startKey, endKey Key, limit int) (*GetResult, error) {
	return &GetResult{Items: []Item{
		{Key: NewKey("foo"), Value: []byte("bar")},
	}}, nil
}

func (n *nopBackend) Create(_ context.Context, _ Item) (*Lease, error) {
	return &Lease{}, nil
}

func (n *nopBackend) Put(_ context.Context, _ Item) (*Lease, error) {
	return &Lease{}, nil
}

func (n *nopBackend) Update(_ context.Context, _ Item) (*Lease, error) {
	return &Lease{}, nil
}

func (n *nopBackend) ConditionalUpdate(_ context.Context, _ Item) (*Lease, error) {
	return &Lease{}, nil
}

func (n *nopBackend) CompareAndSwap(_ context.Context, _ Item, _ Item) (*Lease, error) {
	return &Lease{}, nil
}

func (n *nopBackend) Delete(_ context.Context, _ Key) error {
	return nil
}

func (n *nopBackend) ConditionalDelete(ctx context.Context, key Key, revision string) error {
	return nil
}

func (n *nopBackend) DeleteRange(_ context.Context, _ Key, _ Key) error {
	return nil
}

func (n *nopBackend) KeepAlive(_ context.Context, _ Lease, _ time.Time) error {
	return nil
}

func (n *nopBackend) AtomicWrite(_ context.Context, _ []ConditionalAction) (revision string, err error) {
	return "", nil
}

func (n *nopBackend) Close() error {
	return nil
}

func (n *nopBackend) Clock() clockwork.Clock {
	return clockwork.NewFakeClock()
}

// NewWatcher returns a new event watcher
func (n *nopBackend) NewWatcher(ctx context.Context, watch Watch) (Watcher, error) {
	return nil, nil
}

// CloseWatchers closes all the watchers
// without closing the backend
func (n *nopBackend) CloseWatchers() {

}
