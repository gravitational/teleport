// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package discovery

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNumberedBlock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		style   textStyle
		index   int
		details []keyValue
		want    string
	}{
		{
			name:  "single key",
			index: 0,
			details: []keyValue{
				{Key: "HOST", Value: "i-abc123"},
			},
			want: "[1] HOST: i-abc123\n",
		},
		{
			name:  "aligned keys",
			index: 2,
			details: []keyValue{
				{Key: "INSTANCE", Value: "i-abc123"},
				{Key: "RESULT", Value: "Failed"},
				{Key: "RUNS", Value: "5"},
			},
			want: "[3] INSTANCE: i-abc123\n" +
				"    RESULT  : Failed\n" +
				"    RUNS    : 5\n",
		},
		{
			name:  "double digit index widens prefix",
			index: 99,
			details: []keyValue{
				{Key: "HOST", Value: "abc"},
				{Key: "STATE", Value: "ok"},
			},
			want: "[100] HOST : abc\n" +
				"      STATE: ok\n",
		},
		{
			name:    "empty details",
			index:   0,
			details: nil,
			want:    "",
		},
		{
			name:  "with indent via style",
			style: textStyle{indent: "  "},
			index: 0,
			details: []keyValue{
				{Key: "HOST", Value: "abc"},
				{Key: "STATE", Value: "ok"},
			},
			want: "  [1] HOST : abc\n" +
				"      STATE: ok\n",
		},
		{
			name:  "nested style",
			style: textStyle{}.nested(0),
			index: 0,
			details: []keyValue{
				{Key: "TIMESTAMP", Value: "2026-02-13 17:43:33"},
				{Key: "RESULT", Value: "Failed"},
			},
			want: "    [1] TIMESTAMP: 2026-02-13 17:43:33\n" +
				"        RESULT   : Failed\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := tt.style.numberedBlock(&buf, tt.index, tt.details)
			require.NoError(t, err)
			require.Equal(t, tt.want, buf.String())
		})
	}
}

func TestKeyValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		style textStyle
		lines []keyValue
		want  string
	}{
		{
			name:  "indented stats section",
			style: textStyle{}.indented(),
			lines: []keyValue{
				{Key: "TOTAL EVENTS", Value: "15"},
				{Key: "SUCCESSFUL", Value: "12"},
				{Key: "FAILED", Value: "3"},
				{Key: "DISTINCT HOSTS", Value: "5"},
				{Key: "HOSTS WITH FAILURES", Value: "2"},
			},
			want: "  TOTAL EVENTS       : 15\n" +
				"  SUCCESSFUL         : 12\n" +
				"  FAILED             : 3\n" +
				"  DISTINCT HOSTS     : 5\n" +
				"  HOSTS WITH FAILURES: 2\n",
		},
		{
			name: "top-level header",
			lines: []keyValue{
				{Key: "NAME", Value: "my-integration"},
				{Key: "TYPE", Value: "AWS OIDC"},
				{Key: "ROLE ARN", Value: "arn:aws:iam::123:role/R"},
			},
			want: "NAME    : my-integration\n" +
				"TYPE    : AWS OIDC\n" +
				"ROLE ARN: arn:aws:iam::123:role/R\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := tt.style.keyValues(&buf, tt.lines)
			require.NoError(t, err)
			require.Equal(t, tt.want, buf.String())
		})
	}
}

func TestStyleNesting(t *testing.T) {
	t.Parallel()

	t.Run("indented adds two spaces", func(t *testing.T) {
		s := textStyle{}.indented()
		require.Equal(t, "  ", s.indent)
		s = s.indented()
		require.Equal(t, "    ", s.indent)
	})

	t.Run("nested matches bracket width", func(t *testing.T) {
		s := textStyle{}.nested(0) // [1] = 4 chars
		require.Equal(t, "    ", s.indent)
		s = textStyle{}.nested(99) // [100] = 6 chars
		require.Equal(t, "      ", s.indent)
	})

	t.Run("chaining preserves color setting", func(t *testing.T) {
		s := textStyle{enabled: true}
		require.True(t, s.indented().enabled)
		require.True(t, s.nested(0).enabled)
	})
}
