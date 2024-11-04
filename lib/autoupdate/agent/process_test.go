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
