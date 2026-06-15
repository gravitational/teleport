/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package events

import (
	"bytes"
	stdgzip "compress/gzip"
	"io"
	"testing"

	kgzip "github.com/klauspost/compress/gzip"
	"github.com/stretchr/testify/require"
)

// nopWriteCloser adapts a bytes.Buffer to io.WriteCloser for the gzipWriter.
type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

// TestGzipWriterRoundTrip ensures recordings written by the (klauspost) writer
// are still readable by the stdlib-based reader, including the multistream
// padding workaround. This guards the compress/gzip -> klauspost writer swap.
func TestGzipWriterRoundTrip(t *testing.T) {
	payload := bytes.Repeat([]byte("session recording chunk payload 0123456789\n"), 4096)

	var compressed bytes.Buffer
	w := newGzipWriter(nopWriteCloser{&compressed})
	_, err := w.Write(payload)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	// The reader path (stdlib gzip + Multistream(false)) must decode it.
	r, err := newGzipReader(io.NopCloser(bytes.NewReader(compressed.Bytes())))
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })

	got, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, payload, got, "decoded recording must match original")
	require.Less(t, compressed.Len(), len(payload), "payload should actually be compressed")
}

// TestGzipReaderToleratesTrailingPadding guards the Teleport-specific reason the
// reader disables multistream: older versions sometimes wrote stray padding bytes
// after the gzip member, and the reader must still decode the recording instead of
// failing when it encounters them. This is our contract, not the gzip libraries'.
func TestGzipReaderToleratesTrailingPadding(t *testing.T) {
	payload := []byte("session recording payload")

	var buf bytes.Buffer
	w := newGzipWriter(nopWriteCloser{&buf})
	_, err := w.Write(payload)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	// Simulate the stray bytes appended after the gzip member by old versions.
	buf.Write([]byte{0, 0, 0, 0})

	r, err := newGzipReader(io.NopCloser(bytes.NewReader(buf.Bytes())))
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })

	got, err := io.ReadAll(r)
	require.NoError(t, err, "reader must not choke on trailing padding")
	require.Equal(t, payload, got)
}

// TestGzipWriterPoolReuse exercises the Get/Reset/Put pool cycle that the
// recording hot path relies on, ensuring a recycled writer still round-trips.
func TestGzipWriterPoolReuse(t *testing.T) {
	for i := range 3 {
		var buf bytes.Buffer
		w := newGzipWriter(nopWriteCloser{&buf})
		msg := []byte("reuse iteration payload")
		_, err := w.Write(msg)
		require.NoError(t, err)
		require.NoError(t, w.Close()) // returns the writer to writerPool

		r, err := newGzipReader(io.NopCloser(bytes.NewReader(buf.Bytes())))
		require.NoError(t, err)
		got, err := io.ReadAll(r)
		require.NoError(t, err, "iteration %d", i)
		require.NoError(t, r.Close())
		require.Equal(t, msg, got)
	}
}

// BenchmarkNewGzipWriter compares per-writer allocation between the klauspost
// and stdlib gzip writers at BestSpeed. The Stdlib sub-benchmark is the baseline
// for comparison.
// Run: go test -run=^$ -bench=BenchmarkNewGzipWriter -benchmem ./lib/events/
func BenchmarkNewGzipWriter(b *testing.B) {
	// Write+Flush forces the underlying flate compressor to materialize, since
	// both libraries allocate it lazily on first write rather than at
	// construction. This is what the real recording path does.
	payload := []byte("session recording event payload")
	b.Run("Klauspost", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			w, _ := kgzip.NewWriterLevel(io.Discard, kgzip.BestSpeed)
			_, _ = w.Write(payload)
			_ = w.Flush()
		}
	})
	b.Run("Stdlib", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			w, _ := stdgzip.NewWriterLevel(io.Discard, stdgzip.BestSpeed)
			_, _ = w.Write(payload)
			_ = w.Flush()
		}
	})
}
