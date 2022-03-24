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

	"github.com/stretchr/testify/require"
)

func TestContextReader(t *testing.T) {
	pr, pw := io.Pipe()
	t.Cleanup(func() { pr.Close() })
	t.Cleanup(func() { pw.Close() })

	write := func(t *testing.T, s string) {
		_, err := pw.Write([]byte(s))
		require.NoError(t, err)
	}
	ctx := context.Background()

	r := NewContextReader(pr)

	t.Run("simple read", func(t *testing.T) {
		go write(t, "hello")
		buf, err := r.ReadContext(ctx)
		require.NoError(t, err)
		require.Equal(t, string(buf), "hello")
	})

	t.Run("cancelled read", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		go cancel()
		buf, err := r.ReadContext(cancelCtx)
		require.ErrorIs(t, err, context.Canceled)
		require.Empty(t, buf)

		go write(t, "after cancel")
		buf, err = r.ReadContext(ctx)
		require.NoError(t, err)
		require.Equal(t, string(buf), "after cancel")
	})

	t.Run("close underlying reader", func(t *testing.T) {
		go func() {
			write(t, "before close")
			pw.CloseWithError(io.EOF)
		}()

		// Read the last chunk of data successfully.
		buf, err := r.ReadContext(ctx)
		require.NoError(t, err)
		require.Equal(t, string(buf), "before close")

		// Next read fails because underlying reader is closed.
		buf, err = r.ReadContext(ctx)
		require.ErrorIs(t, err, io.EOF)
		require.Empty(t, buf)
	})
}
