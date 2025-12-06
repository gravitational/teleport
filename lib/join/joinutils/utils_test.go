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

package joinutils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGlobMatchAllowEmptyPattern(t *testing.T) {
	tests := []struct {
		name string

		rule  string
		claim string

		want bool
	}{
		{
			name:  "no rule",
			rule:  "",
			claim: "foo",
			want:  true,
		},
		{
			name:  "non-globby rule matches",
			rule:  "foo",
			claim: "foo",
			want:  true,
		},
		{
			name:  "globby rule matches",
			rule:  "?est-*-foo",
			claim: "test-string-foo",
			want:  true,
		},
		{
			name:  "globby rule mismatch",
			rule:  "?est-*-foo",
			claim: "ttest-bar-ffoo",
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GlobMatchAllowEmptyPattern(tt.rule, tt.claim)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
