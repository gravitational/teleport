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

package proxy

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_wrapContentEncoding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		encoding string
		wantErr  string
		check    func(t *testing.T)
	}{
		{
			name:     "unsupported encoding",
			encoding: "br",
			wantErr:  "unsupported Content-Encoding",
		},
		{
			name:     "deflate unsupported",
			encoding: "deflate",
			wantErr:  "unsupported Content-Encoding",
		},
		{
			name:     "empty encoding",
			encoding: "",
			check: func(t *testing.T) {
				r, w, err := wrapContentEncoding(bytes.NewReader(nil), io.Discard, "")
				require.NoError(t, err)
				require.NoError(t, r.Close())
				require.NoError(t, w.Close())
			},
		},
		{
			name:     "identity encoding",
			encoding: "identity",
			check: func(t *testing.T) {
				r, w, err := wrapContentEncoding(bytes.NewReader(nil), io.Discard, "identity")
				require.NoError(t, err)
				require.NoError(t, r.Close())
				require.NoError(t, w.Close())
			},
		},
		{
			name:     "gzip round trip",
			encoding: "gzip",
			check: func(t *testing.T) {
				original := []byte("hello, gzip world!")

				var compressed bytes.Buffer
				gz := gzip.NewWriter(&compressed)
				_, err := gz.Write(original)
				require.NoError(t, err)
				require.NoError(t, gz.Close())

				var recompressed bytes.Buffer
				reader, writer, err := wrapContentEncoding(&compressed, &recompressed, "gzip")
				require.NoError(t, err)

				decompressed, err := io.ReadAll(reader)
				require.NoError(t, err)
				require.NoError(t, reader.Close())
				require.Equal(t, original, decompressed)

				_, err = writer.Write(original)
				require.NoError(t, err)
				require.NoError(t, writer.Close())

				gzr, err := gzip.NewReader(&recompressed)
				require.NoError(t, err)
				result, err := io.ReadAll(gzr)
				require.NoError(t, err)
				require.Equal(t, original, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.wantErr != "" {
				_, _, err := wrapContentEncoding(bytes.NewReader(nil), io.Discard, tt.encoding)
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			tt.check(t)
		})
	}
}
