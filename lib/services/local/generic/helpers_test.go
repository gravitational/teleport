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

package generic_test

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

func TestCollectPageAndCursor(t *testing.T) {
	tests := []struct {
		name      string
		s         stream.Stream[int]
		limit     int
		wantItems []int
		wantNext  string
		errAssert func(require.TestingT, error, ...any)
	}{
		{
			name:      "empty stream",
			s:         stream.Empty[int](),
			errAssert: require.NoError,
		},
		{
			name:      "default limit",
			s:         stream.Slice([]int{1, 2, 3, 4}),
			wantItems: []int{1, 2, 3, 4},
			errAssert: require.NoError,
		},
		{
			name:      "limit hit",
			s:         stream.Slice([]int{1, 2, 3, 4}),
			limit:     2,
			wantItems: []int{1, 2},
			wantNext:  "3",
			errAssert: require.NoError,
		},
		{
			name:      "error propagated",
			s:         stream.Fail[int](trace.BadParameter("ooops")),
			errAssert: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotNext, gotErr := generic.CollectPageAndCursor(tt.s, tt.limit,
				func(item int) string { return fmt.Sprintf("%d", item) })
			require.Empty(t, cmp.Diff(tt.wantItems, got))
			require.Equal(t, tt.wantNext, gotNext)
			tt.errAssert(t, gotErr)
		})
	}
}
