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

package objects

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestGetObjectFetcher(t *testing.T) {
	type dummyObjectFetcher struct{ ObjectFetcher }

	tests := []struct {
		name       string
		getFetcher ObjectFetcherFn
		expectFunc func(t *testing.T, fetcher ObjectFetcher, err error)
	}{
		{
			name: "valid configuration",
			getFetcher: func(ctx context.Context, db types.Database, cfg ObjectFetcherConfig) (ObjectFetcher, error) {
				return &dummyObjectFetcher{}, nil
			},
			expectFunc: func(t *testing.T, fetcher ObjectFetcher, err error) {
				require.NoError(t, err)
				rulesFetcher, ok := fetcher.(*applyRulesFetcher)
				require.True(t, ok)
				require.IsType(t, &dummyObjectFetcher{}, rulesFetcher.innerFetcher)
			},
		},
		{
			name: "error returned from constructor",
			getFetcher: func(ctx context.Context, db types.Database, cfg ObjectFetcherConfig) (ObjectFetcher, error) {
				return nil, trace.BadParameter("having a bad day, sorry")
			},
			expectFunc: func(t *testing.T, fetcher ObjectFetcher, err error) {
				require.ErrorContains(t, err, "having a bad day, sorry")
			},
		},
		{
			name: "unsupported protocol",
			expectFunc: func(t *testing.T, fetcher ObjectFetcher, err error) {
				require.ErrorContains(t, err, "fetcher not implemented for protocol")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			fakeProto := "fakeProto-" + uuid.New().String()
			if tt.getFetcher != nil {
				RegisterObjectFetcher(tt.getFetcher, fakeProto)
				t.Cleanup(func() { unregisterObjectFetcher(fakeProto) })
			}

			db := &types.DatabaseV3{}
			db.SetName("dummy")
			db.Spec.Protocol = fakeProto

			fetcher, err := GetObjectFetcher(ctx, db, ObjectFetcherConfig{})
			tt.expectFunc(t, fetcher, err)
		})
	}
}

// unregisterObjectFetcher is reverse of RegisterObjectFetcher, but only used in tests.
func unregisterObjectFetcher(names ...string) {
	objectFetchersMutex.Lock()
	defer objectFetchersMutex.Unlock()
	for _, name := range names {
		delete(objectFetchers, name)
	}
}
