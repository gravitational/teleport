/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package prompt

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextReader(t *testing.T) {
	pr, pw := io.Pipe()
	t.Cleanup(func() { pr.Close() })
	t.Cleanup(func() { pw.Close() })

	write := func(t *testing.T, s string) {
		_, err := pw.Write([]byte(s))
		assert.NoError(t, err, "Write failed")
	}

	ctx := context.Background()
	cr := NewContextReader(pr)

	t.Run("simple read", func(t *testing.T) {
		go write(t, "hello")
		buf, err := cr.ReadContext(ctx)
		require.NoError(t, err)
		require.Equal(t, string(buf), "hello")
	})

	t.Run("reclaim abandoned read", func(t *testing.T) {
		done := make(chan struct{})
		cancelCtx, cancel := context.WithCancel(ctx)
		go func() {
			time.Sleep(1 * time.Millisecond) // give ReadContext time to block
			cancel()
			write(t, "after cancel")
			close(done)
		}()
		buf, err := cr.ReadContext(cancelCtx)
		require.ErrorIs(t, err, context.Canceled)
		require.Empty(t, buf)

		<-done // wait for write
		buf, err = cr.ReadContext(ctx)
		require.NoError(t, err)
		require.Equal(t, string(buf), "after cancel")
	})

	t.Run("close ContextReader", func(t *testing.T) {
		go func() {
			time.Sleep(1 * time.Millisecond) // give ReadContext time to block
			assert.NoError(t, cr.Close(), "Close errored")
		}()
		_, err := cr.ReadContext(ctx)
		require.ErrorIs(t, err, ErrReaderClosed)

		// Subsequent reads fail.
		_, err = cr.ReadContext(ctx)
		require.ErrorIs(t, err, ErrReaderClosed)

		// Ongoing read after Close is dropped.
		write(t, "unblock goroutine")
		buf, err := cr.ReadContext(ctx)
		assert.ErrorIs(t, err, ErrReaderClosed)
		assert.Empty(t, buf, "buf not empty")

		// Multiple closes are fine.
		assert.NoError(t, cr.Close(), "2nd Close failed")
	})

	// Re-creating is safe because the tests above leave no "pending" reads.
	cr = NewContextReader(pr)

	t.Run("close underlying reader", func(t *testing.T) {
		go func() {
			write(t, "before close")
			pw.CloseWithError(io.EOF)
		}()

		// Read the last chunk of data successfully.
		buf, err := cr.ReadContext(ctx)
		require.NoError(t, err)
		require.Equal(t, string(buf), "before close")

		// Next read fails because underlying reader is closed.
		buf, err = cr.ReadContext(ctx)
		require.ErrorIs(t, err, io.EOF)
		require.Empty(t, buf)
	})
}
