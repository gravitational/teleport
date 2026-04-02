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
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readerAt returns an io.ReadCloser that reads from data starting at offset.
func readerAt(data []byte, offset int64) io.ReadCloser {
	return io.NopCloser(bytes.NewReader(data[offset:]))
}

func TestRead(t *testing.T) {
	data := []byte("hello world")

	tests := []struct {
		name     string
		makeFunc func(t *testing.T) func(context.Context, int64) (io.ReadCloser, error)
		wantData []byte
		wantErr  bool
	}{
		{
			name: "success",
			makeFunc: func(_ *testing.T) func(context.Context, int64) (io.ReadCloser, error) {
				return func(_ context.Context, offset int64) (io.ReadCloser, error) {
					return readerAt(data, offset), nil
				}
			},
			wantData: data,
		},
		{
			name: "retry on transient error",
			makeFunc: func(t *testing.T) func(context.Context, int64) (io.ReadCloser, error) {
				calls := 0
				return func(_ context.Context, offset int64) (io.ReadCloser, error) {
					calls++
					if calls == 1 {
						return nil, errors.New("transient error")
					}
					assert.Equal(t, 2, calls)
					return readerAt(data, offset), nil
				}
			},
			wantData: data,
		},
		{
			name: "resume from offset after mid-stream failure",
			makeFunc: func(t *testing.T) func(context.Context, int64) (io.ReadCloser, error) {
				first := true
				return func(_ context.Context, offset int64) (io.ReadCloser, error) {
					if first {
						first = false
						return &partialReader{data: data, failAfter: 5}, nil
					}
					assert.Equal(t, int64(5), offset)
					return readerAt(data, offset), nil
				}
			},
			wantData: data,
		},
		{
			name: "exhausts all attempts",
			makeFunc: func(t *testing.T) func(context.Context, int64) (io.ReadCloser, error) {
				calls := 0
				return func(_ context.Context, _ int64) (io.ReadCloser, error) {
					calls++
					t.Cleanup(func() { assert.Equal(t, maxDownloadAttempts, calls) })
					return nil, errors.New("always fails")
				}
			},
			wantErr: true,
		},
		{
			name: "retry and recover on short EOF",
			makeFunc: func(t *testing.T) func(context.Context, int64) (io.ReadCloser, error) {
				// First attempt returns only the first 5 bytes then EOF.
				// Second attempt resumes from offset 5 and returns the rest.
				first := true
				return func(_ context.Context, offset int64) (io.ReadCloser, error) {
					if first {
						first = false
						assert.Equal(t, int64(0), offset)
						return io.NopCloser(bytes.NewReader(data[:5])), nil
					}
					assert.Equal(t, int64(5), offset)
					return readerAt(data, offset), nil
				}
			},
			wantData: data,
		},
		{
			name: "exhausts all attempts on persistent short EOF",
			makeFunc: func(_ *testing.T) func(context.Context, int64) (io.ReadCloser, error) {
				// Every attempt returns only the first 5 bytes then EOF.
				return func(_ context.Context, offset int64) (io.ReadCloser, error) {
					return io.NopCloser(bytes.NewReader(data[offset:5])), nil
				}
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := New(t.Context(), int64(len(data)), tc.makeFunc(t))
			defer r.Close()

			got, err := io.ReadAll(r)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantData, got)
		})
	}
}

func TestRead_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	r := New(ctx, 1, func(_ context.Context, _ int64) (io.ReadCloser, error) {
		return nil, errors.New("should not matter")
	})
	defer r.Close()

	_, err := r.Read(make([]byte, 1))
	require.ErrorIs(t, err, os.ErrClosed)
}

func TestClose_ConcurrentWithRead(t *testing.T) {
	// Make Read block until we signal it, so we can call Close concurrently.
	readStarted := make(chan struct{})
	unblock := make(chan struct{})

	r := New(t.Context(), 0, func(_ context.Context, _ int64) (io.ReadCloser, error) {
		return &blockingReader{started: readStarted, unblock: unblock}, nil
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 1)
		_, _ = r.Read(buf)
	}()

	// Wait for Read to be inside the blocking reader, then close concurrently.
	<-readStarted
	require.NoError(t, r.Close())
	close(unblock)
	wg.Wait()
}

func TestClose_Idempotent(t *testing.T) {
	data := []byte("hello")
	r := New(t.Context(), int64(len(data)), func(_ context.Context, offset int64) (io.ReadCloser, error) {
		return readerAt(data, offset), nil
	})

	_, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NoError(t, r.Close())
	require.NoError(t, r.Close())
}

// partialReader reads up to failAfter bytes from data then returns an error.
type partialReader struct {
	data      []byte
	failAfter int
	read      int
	closed    bool
}

func (p *partialReader) Read(buf []byte) (int, error) {
	remaining := p.failAfter - p.read
	if remaining <= 0 {
		return 0, errors.New("injected read error")
	}
	n := min(len(buf), remaining)
	copy(buf, p.data[p.read:p.read+n])
	p.read += n
	if p.read >= p.failAfter {
		return n, errors.New("injected read error")
	}
	return n, nil
}

func (p *partialReader) Close() error {
	p.closed = true
	return nil
}

// blockingReader signals readStarted then blocks until unblock is closed.
type blockingReader struct {
	started chan struct{}
	unblock chan struct{}
	once    sync.Once
}

func (b *blockingReader) Read(_ []byte) (int, error) {
	b.once.Do(func() { close(b.started) })
	<-b.unblock
	return 0, errors.New("closed")
}

func (b *blockingReader) Close() error { return nil }
