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

func TestInput(t *testing.T) {
	t.Parallel()

	out, in := io.Pipe()
	t.Cleanup(func() { out.Close() })
	write := func(t *testing.T, s string) {
		_, err := in.Write([]byte(s))
		require.NoError(t, err)
	}

	r := NewContextReader(out)
	ctx := context.Background()

	t.Run("no whitespace", func(t *testing.T) {
		go write(t, "hi")
		got, err := Input(ctx, io.Discard, r, "")
		require.NoError(t, err)
		require.Equal(t, "hi", got)
	})

	t.Run("with whitespace", func(t *testing.T) {
		go write(t, "hey\n")
		got, err := Input(ctx, io.Discard, r, "")
		require.NoError(t, err)
		require.Equal(t, "hey", got)
	})

	t.Run("closed input", func(t *testing.T) {
		require.NoError(t, in.Close())
		got, err := Input(ctx, io.Discard, r, "")
		require.ErrorIs(t, err, io.EOF)
		require.Empty(t, got)
	})
}
