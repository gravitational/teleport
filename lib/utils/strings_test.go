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

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrimNonEmpty(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		in        string
		wantValue string
		wantKeep  bool
	}{
		{name: "empty input is dropped", in: "", wantValue: "", wantKeep: false},
		{name: "whitespace-only input is dropped", in: "  \t\n", wantValue: "", wantKeep: false},
		{name: "leading whitespace trimmed and kept", in: "  hello", wantValue: "hello", wantKeep: true},
		{name: "trailing whitespace trimmed and kept", in: "hello  ", wantValue: "hello", wantKeep: true},
		{name: "surrounding whitespace trimmed and kept", in: " \tx\n ", wantValue: "x", wantKeep: true},
		{name: "unicode whitespace trimmed and kept", in: "\u00a0hello\u00a0", wantValue: "hello", wantKeep: true},
		{name: "internal whitespace preserved", in: "a b", wantValue: "a b", wantKeep: true},
		{name: "no whitespace passes through", in: "plain", wantValue: "plain", wantKeep: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, keep := TrimNonEmpty(tt.in)
			assert.Equal(t, tt.wantValue, got)
			assert.Equal(t, tt.wantKeep, keep)
		})
	}
}
