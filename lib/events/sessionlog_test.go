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

package events

import (
	"bytes"
	stdgzip "compress/gzip"
	"fmt"
	"io"
	"testing"

	kgzip "github.com/klauspost/compress/gzip"
	"github.com/stretchr/testify/require"
)

// These tests lock in Teleport's contract around the gzip library (the wrapper
// plumbing, backward compatibility with existing recordings, and the
// Multistream(false) padding workaround). They intentionally do not sweep
// payload shapes or otherwise re-verify that gzip compresses correctly — that is
// the gzip library's responsibility, exercised by its own tests.

// nopWriteCloser adapts a bytes.Buffer to io.WriteCloser for the gzipWriter.
type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

// compressGzip compresses payload into a standard gzip member. kind selects the
// implementation: "klauspost" exercises the production writer (newGzipWriter);
// "stdlib" simulates a recording written by an older Teleport version.
func compressGzip(t *testing.T, kind string, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	switch kind {
	case "klauspost":
		w := newGzipWriter(nopWriteCloser{&buf})
		_, err := w.Write(payload)
		require.NoError(t, err)
		require.NoError(t, w.Close())
	case "stdlib":
		w, err := stdgzip.NewWriterLevel(&buf, stdgzip.BestSpeed)
		require.NoError(t, err)
		_, err = w.Write(payload)
		require.NoError(t, err)
		require.NoError(t, w.Close())
	default:
		t.Fatalf("unknown writer kind %q", kind)
	}
	return buf.Bytes()
}

// decodeGzip decodes data through the production reader path (newGzipReader).
func decodeGzip(t *testing.T, data []byte) ([]byte, error) {
	t.Helper()
	r, err := newGzipReader(io.NopCloser(bytes.NewReader(data)))
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()
	return io.ReadAll(r)
}

// TestGzipWriterReaderRoundTrip checks the Teleport plumbing around the gzip
// library: a recording written by the production writer decodes through the
// production reader, and a writer survives the writerPool Get/Reset/Put cycle
// (the loop, since each Close returns the writer to the pool).
func TestGzipWriterReaderRoundTrip(t *testing.T) {
	payload := bytes.Repeat([]byte("session recording event 0123456789\n"), 4096)
	for i := range 3 {
		data := compressGzip(t, "klauspost", payload)
		got, err := decodeGzip(t, data)
		require.NoError(t, err, "iteration %d", i)
		require.Equal(t, payload, got)
	}
}

// TestGzipReaderReadsLegacyRecordings is the backward-compatibility guard for the
// gzip swap: recordings written by older Teleport versions (standard library
// compress/gzip) must still decode through the current reader.
func TestGzipReaderReadsLegacyRecordings(t *testing.T) {
	payload := bytes.Repeat([]byte("session recording event 0123456789\n"), 4096)
	data := compressGzip(t, "stdlib", payload)
	got, err := decodeGzip(t, data)
	require.NoError(t, err)
	require.Equal(t, payload, got)
}

// TestGzipReaderToleratesTrailingPadding guards the one reader behavior Teleport
// configures itself, Multistream(false): older versions sometimes wrote stray
// bytes after the gzip member, and the reader must decode the recording rather
// than fail on them. The stdlib case is the historically relevant one (the bug
// shipped in stdlib-writer versions); the klauspost case guards new recordings.
func TestGzipReaderToleratesTrailingPadding(t *testing.T) {
	payload := []byte("session recording payload")
	for _, kind := range []string{"klauspost", "stdlib"} {
		t.Run(kind, func(t *testing.T) {
			data := compressGzip(t, kind, payload)
			padded := append(bytes.Clone(data), 0, 0, 0, 0) // stray bytes after the member
			got, err := decodeGzip(t, padded)
			require.NoError(t, err, "reader must tolerate trailing padding")
			require.Equal(t, payload, got)
		})
	}
}

// BenchmarkNewGzipWriter compares per-writer allocation between the klauspost and
// stdlib gzip writers at BestSpeed (the memory justification for the writer).
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

// BenchmarkGzipReader compares decode time and allocations between the klauspost
// and stdlib gzip readers (the performance justification for the reader).
// Run: go test -run=^$ -bench=BenchmarkGzipReader -benchmem ./lib/events/
func BenchmarkGzipReader(b *testing.B) {
	// A few MB of varied, recording-like text. Varied (not a single repeated
	// string, which compresses unrealistically well) so decode does real work
	// and the stdlib reader's per-block allocation cost shows.
	var raw bytes.Buffer
	for i := 0; raw.Len() < 2<<20; i++ {
		fmt.Fprintf(&raw, "2026-06-16T12:%02d:%02d user=admin action=exec cmd=\"ls -la /var/log/app-%d\" status=ok bytes=%d\n", i%60, (i*7)%60, i, i*131)
	}
	var gz bytes.Buffer
	w, _ := stdgzip.NewWriterLevel(&gz, stdgzip.BestSpeed)
	_, _ = w.Write(raw.Bytes())
	_ = w.Close()
	data := gz.Bytes()

	uncompressed := int64(raw.Len()) // for MB/s decode throughput (bytes produced per op)
	b.Run("Klauspost", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(uncompressed)
		for b.Loop() {
			r, _ := kgzip.NewReader(bytes.NewReader(data))
			r.Multistream(false)
			_, _ = io.Copy(io.Discard, r)
			_ = r.Close()
		}
	})
	b.Run("Stdlib", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(uncompressed)
		for b.Loop() {
			r, _ := stdgzip.NewReader(bytes.NewReader(data))
			r.Multistream(false)
			_, _ = io.Copy(io.Discard, r)
			_ = r.Close()
		}
	})
}
