/*

 Copyright 2022 Gravitational, Inc.

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

package cassandra

import (
	"bytes"
	"crypto/rand"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_memoryBuffer_Read(t *testing.T) {
	buf := make([]byte, 1024*256)
	_, err := rand.Read(buf)
	require.NoError(t, err)

	mb := newMemoryBuffer(bytes.NewBuffer(buf))

	t.Run("Buffer is empty after creation", func(t *testing.T) {
		require.Empty(t, mb.Bytes())
	})

	readAndAssert := func(currentPos, bufSize int) int {
		read1 := make([]byte, bufSize)

		readBytes, err := mb.Read(read1)
		require.NoError(t, err)
		require.Equal(t, bufSize, readBytes)

		endPos := currentPos + bufSize
		require.Equal(t, buf[endPos-len(mb.Bytes()):endPos], mb.Bytes())

		return endPos
	}

	var pos int
	t.Run("Read 4 bytes", func(t *testing.T) {
		pos = readAndAssert(0, 4)
	})

	t.Run("Read another 4 bytes, data should be appended", func(t *testing.T) {
		pos = readAndAssert(pos, 4)
	})

	t.Run("Rest should clear the buffer", func(t *testing.T) {
		mb.Reset()
		require.Empty(t, mb.Bytes())
	})

	t.Run("Internal buffer is resized", func(t *testing.T) {
		pos = readAndAssert(pos, 8192)
	})

	t.Run("Append still works", func(t *testing.T) {
		pos = readAndAssert(pos, 8192)
	})

	t.Run("Read all remaining", func(t *testing.T) {
		read, err := io.Copy(io.Discard, mb)
		require.NoError(t, err)
		require.Equal(t, int64(len(buf)-pos), read)

		require.Equal(t, buf[len(buf)-len(mb.Bytes()):], mb.Bytes())
	})
}
