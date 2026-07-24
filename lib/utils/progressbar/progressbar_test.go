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

package progressbar

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// lastLine returns the final in-place frame rendered to the bar (the text after
// the last carriage return), trimming any trailing newline.
func lastLine(s string) string {
	frames := strings.Split(s, "\r")
	return strings.TrimSuffix(frames[len(frames)-1], "\n")
}

func TestAddAndFinish(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	bar := New(4, "test", &buf)
	for range 4 {
		bar.Add(1)
	}
	bar.Finish()

	last := lastLine(buf.String())
	require.Contains(t, last, "100%")
	require.Equal(t, width, strings.Count(last, "█"))
	require.True(t, strings.HasSuffix(buf.String(), "\n"))
}

func TestUnboundedGrowth(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	bar := New(2, "test", &buf)
	for range 10 {
		bar.Add(1)
	}
	bar.Finish()

	require.Contains(t, lastLine(buf.String()), "100%")
}

func TestByteCounter(t *testing.T) {
	t.Parallel()

	const data = "hello world"

	var buf bytes.Buffer
	bar := New(int64(len(data)), "file", &buf)
	var _ io.ReadWriter = bar

	n, err := io.Copy(bar, bytes.NewBufferString(data))
	require.NoError(t, err)
	require.Equal(t, int64(len(data)), n)
	bar.Finish()

	require.Contains(t, lastLine(buf.String()), "100%")
}

func TestUnknownTotal(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	bar := New(-1, "download", &buf)
	bar.Add(100)
	require.NotContains(t, buf.String(), "100%")

	bar.Finish()
	require.Contains(t, lastLine(buf.String()), "100%")
}

func TestNilBar(t *testing.T) {
	t.Parallel()

	var bar *Bar
	require.NotPanics(t, func() {
		bar.Add(1)
		bar.Finish()
		n, err := bar.Write([]byte("data"))
		require.NoError(t, err)
		require.Equal(t, 4, n)
		n, err = bar.Read(make([]byte, 4))
		require.NoError(t, err)
		require.Equal(t, 4, n)
	})
}
