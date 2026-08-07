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

package downloadretrier

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"

	"github.com/gravitational/trace"
)

const maxDownloadAttempts = 3

// DownloadFunc starts a download from a specific byte offset and returns
// a ReadCloser for the remaining content. The caller is responsible for
// closing the returned ReadCloser.
// The Retrier will call DownloadFunc up to maxDownloadAttempts times if it
// encounters errors, resuming from the last successfully read offset.
type DownloadFunc func(ctx context.Context, offset int64) (io.ReadCloser, error)

// Retrier is an io.ReadCloser that transparently retries a failed download
// up to maxDownloadAttempts times, resuming from the last successfully read
// byte offset.
type Retrier struct {
	download      DownloadFunc
	ctx           context.Context
	cancel        context.CancelFunc
	lastErr       error
	offset        int64
	contentLength int64
	attempts      int

	// mu protects access to reader and attempts, which are modified by Read and Close.
	mu     sync.Mutex
	reader io.ReadCloser
}

// New creates a Retrier. The provided ctx can be used to cancel the download;
// Close also cancels it.
// contentLength is the expected total number of bytes; an io.EOF before
// contentLength bytes have been read is treated as a retryable short-read
// (io.ErrUnexpectedEOF) rather than a successful end-of-stream.
// Before returning a Retrier, please ensure that the file exists to avoid
// unnecessary retries on a non-existent file.
func New(ctx context.Context, contentLength int64, download DownloadFunc) *Retrier {
	ctx, cancel := context.WithCancel(ctx)
	return &Retrier{
		download:      download,
		ctx:           ctx,
		cancel:        cancel,
		contentLength: contentLength,
	}
}

// Read implements io.Reader. On a read error it closes the current reader and
// opens a fresh one from the last committed offset, up to maxDownloadAttempts
// total. If partial data was read before the error, those bytes are returned
// to the caller without an error; the retry happens on the next Read call.
func (r *Retrier) Read(p []byte) (_ int, err error) {
	for {
		r.mu.Lock()
		reader := r.reader
		if r.ctx.Err() != nil {
			r.mu.Unlock()
			return 0, os.ErrClosed
		}
		if reader == nil {
			if r.attempts >= maxDownloadAttempts {
				r.mu.Unlock()
				return 0, trace.Wrap(r.lastErr, "download failed after %d attempts", maxDownloadAttempts)
			}

			r.attempts++

			reader, err = r.download(r.ctx, r.offset)
			if err != nil {
				r.mu.Unlock()
				if trace.IsAccessDenied(err) {
					return 0, trace.Wrap(err)
				} else if r.ctx.Err() != nil {
					return 0, os.ErrClosed
				}
				r.lastErr = err
				// Count this as a failed attempt and try again.
				continue
			}

			r.reader = reader
		}
		r.mu.Unlock()

		// Snapshot the reader and release the lock before blocking.
		n, err := reader.Read(p)

		r.offset += int64(n)
		if err == nil {
			return n, nil
		}
		if errors.Is(err, io.EOF) {
			// If we received a clean EOF but haven't read all expected bytes,
			// treat it as a retryable short-read rather than a successful
			// end-of-stream. This guards against truncated ranged responses
			// returned by intermediaries.
			if r.offset < r.contentLength {
				err = io.ErrUnexpectedEOF
			} else {
				return n, io.EOF
			}
		}
		r.lastErr = err
		// Non-EOF read error: tear down the current reader so the next
		// iteration reopens it from r.offset. Guard against a concurrent
		// Close having already swapped out r.reader.
		r.mu.Lock()
		if r.reader != nil {
			r.reader.Close()
			r.reader = nil
		}
		r.mu.Unlock()

		if r.ctx.Err() != nil {
			return n, os.ErrClosed
		}

		// Return any bytes that were already read; the retry happens on the
		// next Read call so the caller's buffer is not wasted.
		if n > 0 {
			return n, nil
		}
	}
}

// Close cancels the underlying context and closes the current reader, if any.
// It is safe to call Close concurrently with or after Read.
func (r *Retrier) Close() error {
	r.cancel()

	r.mu.Lock()
	defer r.mu.Unlock()
	reader := r.reader
	r.reader = nil

	if reader != nil {
		return reader.Close()
	}
	return nil
}
