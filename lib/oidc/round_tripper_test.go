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

package oidc

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

// countingReader returns zeroes forever, counting the number of Read() calls
type countingReader struct {
	reads int
}

func (c *countingReader) Read(p []byte) (int, error) {
	c.reads++
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

// TestLimitReadCloserErrorsWhenExceeded ensures the limitReadCloser actually
// returns an error when the limit is exceeded instead of trying to discard
// everything.
func TestLimitReadCloserErrorsWhenExceeded(t *testing.T) {
	t.Parallel()

	reader := &countingReader{}
	lrc := newLimitReadCloser(reader, io.NopCloser(nil))

	// Make a buffer guaranteed to be larger than maxDataSize.
	buf := make([]byte, maxDataSize+1)

	// The first read should succeed...
	n, err := lrc.Read(buf)
	require.NoError(t, err)
	require.Equal(t, maxDataSize+1, n)
	require.Equal(t, 1, reader.reads)

	// ...but the next read should error immediately without hitting the
	// underlying reader
	_, err = lrc.Read(buf)
	require.ErrorContains(t, err, "response exceeds maximum size of")
	require.Equal(t, 1, reader.reads)
}
