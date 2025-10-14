// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

func TestGenericListerPageClipEnd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		page     []string
		next     string
		end      string
		want     []string
		wantNext string
	}{
		{
			name:     "end unspecified, return as is",
			page:     []string{"a", "b"},
			next:     "c",
			want:     []string{"a", "b"},
			wantNext: "c",
		},
		{
			name:     "end token found in page",
			page:     []string{"a", "b", "c"},
			end:      "b",
			next:     "d",
			want:     []string{"a"},
			wantNext: "",
		},
		{
			name:     "next token unaffected when no end found",
			page:     []string{"a", "b"},
			end:      "c",
			next:     "c",
			want:     []string{"a", "b"},
			wantNext: "c",
		},
		{
			name:     "end was before page",
			page:     []string{"b", "c", "d"},
			end:      "a",
			next:     "e",
			want:     []string{},
			wantNext: "",
		},
	}
	for _, tt := range tests {
		g := genericLister[string, string]{
			nextToken: func(s string) string { return s },
		}

		t.Run(tt.name, func(t *testing.T) {
			page, next := g.clipEnd(tt.page, tt.next, tt.end)
			assert.Empty(t, cmp.Diff(tt.want, page))
			assert.Equal(t, tt.wantNext, next)
		})
	}
}

func TestGenericListerRangeWithFallback(t *testing.T) {
	t.Parallel()

	p, err := newPack(t.TempDir(), ForProxy)
	require.NoError(t, err)
	t.Cleanup(p.Close)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.eventsC:
				// Discard events to avoid blocking the test.
			}
		}
	}()

	newResource := func(name string) (types.Application, error) {
		return types.NewAppV3(types.Metadata{
			Name: name,
		}, types.AppSpecV3{
			URI: "localhost",
		})
	}

	var expected []types.Application
	for i := range 9 {
		name := fmt.Sprintf("app-%d", i)
		r, err := newResource(name)
		require.NoError(t, err)
		require.NoError(t, p.apps.CreateApp(t.Context(), r))
		expected = append(expected, r)
	}

	// Wait for cache
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		items, _ := p.cache.GetApps(ctx)
		assert.Len(t, items, len(expected))
	}, 15*time.Second, 100*time.Millisecond)

	tests := []struct {
		name            string
		start           string
		end             string
		cacheOk         bool
		upstreamList    func(context.Context, int, string) ([]types.Application, string, error)
		defaultPageSize int
		fallbackGetter  func(context.Context) ([]types.Application, error)
		want            []types.Application
		wantErr         string
	}{
		{
			name:            "base case with small pages to force depagination",
			upstreamList:    p.apps.ListApps,
			fallbackGetter:  p.apps.GetApps,
			defaultPageSize: 2,
			cacheOk:         true,
			want:            expected,
		},
		{
			name:            "upstream lister",
			upstreamList:    p.apps.ListApps,
			fallbackGetter:  p.apps.GetApps,
			defaultPageSize: 2,
			cacheOk:         false,
			want:            expected,
		},
		{
			name:            "upstream large page respects ends with large page",
			upstreamList:    p.apps.ListApps,
			fallbackGetter:  p.apps.GetApps,
			end:             "app-9",
			defaultPageSize: 100,
			cacheOk:         false,
			want:            expected[:9],
		},
		{
			name:            "upstream lister with bounds",
			upstreamList:    p.apps.ListApps,
			fallbackGetter:  p.apps.GetApps,
			defaultPageSize: 5,
			start:           "app-1",
			end:             "app-9",
			cacheOk:         false,
			want:            expected[1:9],
		},
		{
			name:            "upstream lister with start",
			upstreamList:    p.apps.ListApps,
			fallbackGetter:  p.apps.GetApps,
			defaultPageSize: 5,
			start:           "app-1",
			cacheOk:         false,
			want:            expected[1:],
		},
		{
			name:            "upstream lister with end",
			upstreamList:    p.apps.ListApps,
			fallbackGetter:  p.apps.GetApps,
			defaultPageSize: 5,
			end:             "app-9",
			cacheOk:         false,
			want:            expected[:9],
		},
		{
			name: "upstream fallback",
			upstreamList: func(ctx context.Context, i int, s string) ([]types.Application, string, error) {
				return nil, "", trace.NotImplemented("")
			},
			fallbackGetter:  p.apps.GetApps,
			defaultPageSize: 2,
			cacheOk:         false,
			want:            expected,
		},
		{
			name: "upstream fallback not supported when start and end given",
			upstreamList: func(ctx context.Context, i int, s string) ([]types.Application, string, error) {
				return nil, "", trace.NotImplemented("not implemented")
			},
			fallbackGetter: p.apps.GetApps,
			start:          "app-1",
			end:            "app-9",
			cacheOk:        false,
			wantErr:        "not implemented",
		},
		{
			name: "upstream fallback not supported when start given",
			upstreamList: func(ctx context.Context, i int, s string) ([]types.Application, string, error) {
				return nil, "", trace.NotImplemented("not implemented")
			},
			fallbackGetter: p.apps.GetApps,
			start:          "app-1",
			cacheOk:        false,
			wantErr:        "not implemented",
		},
		{
			name: "upstream fallback not supported when end given",
			upstreamList: func(ctx context.Context, i int, s string) ([]types.Application, string, error) {
				return nil, "", trace.NotImplemented("not implemented")
			},
			fallbackGetter: p.apps.GetApps,
			end:            "app-9",
			cacheOk:        false,
			wantErr:        "not implemented",
		},
		{
			name: "fallback fail",
			upstreamList: func(ctx context.Context, i int, s string) ([]types.Application, string, error) {
				return nil, "", trace.NotImplemented("")
			},
			fallbackGetter:  func(ctx context.Context) ([]types.Application, error) { return nil, trace.BadParameter("oops") },
			defaultPageSize: 2,
			cacheOk:         false,
			wantErr:         "oops",
		},
		{
			name: "upstream list other fail",
			upstreamList: func(ctx context.Context, i int, s string) ([]types.Application, string, error) {
				return nil, "", trace.BadParameter("upstream failed")
			},
			fallbackGetter: p.apps.GetApps,
			cacheOk:        false,
			wantErr:        "upstream failed",
		},
		{
			name: "no fallback set, forward error",
			upstreamList: func(ctx context.Context, i int, s string) ([]types.Application, string, error) {
				return nil, "", trace.NotImplemented("not implemented")
			},
			cacheOk: false,
			wantErr: "not implemented",
		},
		{
			name: "fallback disabled after first item yields",
			upstreamList: func(ctx context.Context, i int, token string) ([]types.Application, string, error) {
				switch token {
				case "":
					return p.apps.ListApps(ctx, i, token)
				default:
					return nil, "", trace.NotImplemented("not implemented")
				}
			},
			defaultPageSize: 2,
			fallbackGetter:  p.apps.GetApps,
			cacheOk:         false,
			want:            expected[:2], // Collect yields first 2 items
			wantErr:         "not implemented",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := genericLister[types.Application, appIndex]{
				cache:      p.cache,
				collection: p.cache.collections.apps,
				index:      appNameIndex,
				nextToken:  types.Application.GetName,

				defaultPageSize: tt.defaultPageSize,
				upstreamList:    tt.upstreamList,
				fallbackGetter:  tt.fallbackGetter,
			}

			p.cache.ok = tt.cacheOk
			got, err := stream.Collect(l.RangeWithFallback(context.Background(), tt.start, tt.end))

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			require.Empty(t, cmp.Diff(tt.want, got,
				cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
				cmpopts.IgnoreFields(header.Metadata{}, "Revision")))
		})
	}
}
