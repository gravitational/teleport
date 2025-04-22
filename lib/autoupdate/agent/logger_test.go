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

package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLineLogger(t *testing.T) {
	t.Parallel()

	out := &bytes.Buffer{}
	ll := lineLogger{
		ctx: context.Background(),
		log: slog.New(slog.NewTextHandler(out,
			&slog.HandlerOptions{ReplaceAttr: msgOnly},
		)),
	}

	for _, e := range []struct {
		v string
		n int
	}{
		{v: "", n: 0},
		{v: "a", n: 1},
		{v: "b\n", n: 2},
		{v: "c\nd", n: 3},
		{v: "e\nf\ng", n: 5},
		{v: "h", n: 1},
		{v: "", n: 0},
		{v: "\n", n: 1},
		{v: "i\n", n: 2},
		{v: "j", n: 1},
	} {
		n, err := ll.Write([]byte(e.v))
		require.NoError(t, err)
		require.Equal(t, e.n, n)
	}
	require.Equal(t, "msg=ab\nmsg=c\nmsg=de\nmsg=f\nmsg=gh\nmsg=i\n", out.String())
	ll.Flush()
	require.Equal(t, "msg=ab\nmsg=c\nmsg=de\nmsg=f\nmsg=gh\nmsg=i\nmsg=j\n", out.String())
}

func msgOnly(_ []string, a slog.Attr) slog.Attr {
	switch a.Key {
	case "time", "level":
		return slog.Attr{}
	}
	return slog.Attr{Key: a.Key, Value: a.Value}
}

func TestProgressLogger(t *testing.T) {
	t.Parallel()

	type write struct {
		n   int
		out string
	}
	for _, tt := range []struct {
		name       string
		max, lines int
		writes     []write
	}{
		{
			name:  "even",
			max:   100,
			lines: 5,
			writes: []write{
				{n: 10},
				{n: 10, out: "20%"},
				{n: 10},
				{n: 10, out: "40%"},
				{n: 10},
				{n: 10, out: "60%"},
				{n: 10},
				{n: 10, out: "80%"},
				{n: 10},
				{n: 10, out: "100%"},
				{n: 10},
				{n: 10, out: "120%"},
			},
		},
		{
			name:  "fast",
			max:   100,
			lines: 5,
			writes: []write{
				{n: 100, out: "100%"},
				{n: 100, out: "200%"},
			},
		},
		{
			name:  "over fast",
			max:   100,
			lines: 5,
			writes: []write{
				{n: 200, out: "200%"},
			},
		},
		{
			name:  "slow down when uneven",
			max:   100,
			lines: 5,
			writes: []write{
				{n: 50, out: "50%"},
				{n: 10, out: "60%"},
				{n: 10, out: "70%"},
				{n: 10, out: "80%"},
				{n: 10},
				{n: 10, out: "100%"},
				{n: 10},
				{n: 10, out: "120%"},
			},
		},
		{
			name:  "slow down when very uneven",
			max:   100,
			lines: 5,
			writes: []write{
				{n: 50, out: "50%"},
				{n: 1, out: "51%"},
				{n: 1},
				{n: 20, out: "72%"},
				{n: 10, out: "82%"},
				{n: 10},
				{n: 10, out: "102%"},
			},
		},
		{
			name:  "close",
			max:   1000,
			lines: 5,
			writes: []write{
				{n: 999, out: "99%"},
				{n: 1, out: "100%"},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			ll := progressLogger{
				ctx: context.Background(),
				log: slog.New(slog.NewTextHandler(out,
					&slog.HandlerOptions{ReplaceAttr: msgOnly},
				)),
				name:  "test",
				max:   tt.max,
				lines: tt.lines,
			}
			for _, e := range tt.writes {
				n, err := ll.Write(make([]byte, e.n))
				require.NoError(t, err)
				require.Equal(t, e.n, n)
				v, err := io.ReadAll(out)
				require.NoError(t, err)
				if len(v) > 0 {
					e.out = fmt.Sprintf(`msg=Downloading file=test progress=%s`+"\n", e.out)
				}
				require.Equal(t, e.out, string(v))
			}
		})
	}
}
