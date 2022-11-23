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

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCaptureNBytesWriter(t *testing.T) {
	t.Parallel()

	data := []byte("abcdef")
	w := NewCaptureNBytesWriter(10)

	// Write 6 bytes. Captured 6 bytes in total.
	n, err := w.Write(data)
	require.Equal(t, 6, n)
	require.NoError(t, err)
	require.Equal(t, "abcdef", string(w.Bytes()))

	// Write 6 bytes. Captured 10 bytes in total.
	n, err = w.Write(data)
	require.Equal(t, 6, n)
	require.NoError(t, err)
	require.Equal(t, "abcdefabcd", string(w.Bytes()))

	// Write 6 bytes. Captured 10 bytes in total.
	n, err = w.Write(data)
	require.Equal(t, 6, n)
	require.NoError(t, err)
	require.Equal(t, "abcdefabcd", string(w.Bytes()))
}
