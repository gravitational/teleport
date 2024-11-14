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

package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_splitCommaSeparatedSlice(t *testing.T) {

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "empty",
		},
		{
			name: "single dir",
			args: []string{"dir1"},
			want: []string{"dir1"},
		},
		{
			name: "multi dir",
			args: []string{"dir1", "dir2"},
			want: []string{"dir1", "dir2"},
		},
		{
			name: "multi dir comma separated",
			args: []string{"dir1,dir2"},
			want: []string{"dir1", "dir2"},
		},
		{
			name: "multi dir comma separated spaces",
			args: []string{"dir1, dir2"},
			want: []string{"dir1", "dir2"},
		},
		{
			name: "multi dir comma separated spaces",
			args: []string{"dir1, dir2", "dir3"},
			want: []string{"dir1", "dir2", "dir3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitCommaSeparatedSlice(tt.args)
			require.Equal(t, tt.want, got)
		})
	}
}
