//go:build bpf && !386

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

package bpf

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConvertArgs(t *testing.T) {
	tests := []struct {
		name      string
		rawArgs   []byte
		truncated bool
		expected  []string
	}{
		{
			name: "no args",
		},
		{
			name:     "only null byte",
			rawArgs:  []byte("\x00"),
			expected: []string{""},
		},
		{
			name:     "only null bytes",
			rawArgs:  []byte("\x00\x00\x00"),
			expected: []string{"", "", ""},
		},
		{
			name:     "1 arg",
			rawArgs:  []byte("foo\x00"),
			expected: []string{"foo"},
		},
		{
			name:     "multiple args",
			rawArgs:  []byte("foo\x00bar\x00baz\x00"),
			expected: []string{"foo", "bar", "baz"},
		},
		{
			name:      "truncated",
			rawArgs:   []byte("foo\x00"),
			truncated: true,
			expected:  []string{"foo", TruncatedArg},
		},
		{
			name:     "consecutive null bytes",
			rawArgs:  []byte("foo\x00\x00bar\x00\x00baz\x00"),
			expected: []string{"foo", "", "bar", "", "baz"},
		},
		{
			name:      "consecutive null bytes truncated",
			rawArgs:   []byte("foo\x00\x00bar\x00\x00baz\x00"),
			truncated: true,
			expected:  []string{"foo", "", "bar", "", "baz", TruncatedArg},
		},
		{
			name:     "no trailing null byte",
			rawArgs:  []byte("foo\x00bar\x00baz"),
			expected: []string{"foo", "bar", "baz"},
		},
		{
			name:      "no trailing null byte truncated",
			rawArgs:   []byte("foo\x00bar\x00baz"),
			truncated: true,
			expected:  []string{"foo", "bar", "baz", TruncatedArg},
		},
		{
			name:     "leading null byte",
			rawArgs:  []byte("\x00foo\x00bar\x00"),
			expected: []string{"", "foo", "bar"},
		},
		{
			name:      "leading null byte truncated",
			rawArgs:   []byte("\x00foo\x00bar\x00"),
			expected:  []string{"", "foo", "bar", TruncatedArg},
			truncated: true,
		},
		{
			name:     "utf-8 args",
			rawArgs:  []byte("résumé\x00日本語\x00"),
			expected: []string{"résumé", "日本語"},
		},
		{
			name:      "utf-8 args truncated",
			rawArgs:   []byte("résumé\x00日本語\x00"),
			expected:  []string{"résumé", "日本語", TruncatedArg},
			truncated: true,
		},
		{
			name:     "full command",
			rawArgs:  []byte("/usr/bin/grep\x00-r\x00pattern\x00/var/log/\x00"),
			expected: []string{"/usr/bin/grep", "-r", "pattern", "/var/log/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertArgs(tt.rawArgs, tt.truncated)
			require.Equal(t, tt.expected, got)
		})
	}
}
