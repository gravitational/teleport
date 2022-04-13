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

package botfs

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestReadWrite attempts to test read/write against all possible symlink
// modes.
func TestReadWrite(t *testing.T) {
	dir := t.TempDir()

	secureWriteExpected, err := HasSecureWriteSupport()
	require.NoError(t, err)

	expectedData := []byte{1, 2, 3, 4}


	for _, mode := range []SymlinksMode{SymlinksInsecure, SymlinksTrySecure, SymlinksSecure}, {
		if mode == SymlinksSecure && !secureWriteExpected {
			t.Logf("skipping secure read/write test due to lack of platform support")
			continue
		}

		path := filepath.Join(dir, string(test.mode))

		err := Create(path, false, test.mode)
		require.NoError(t, err)

		err = Write(path, expectedData, test.mode)
		require.NoError(t, err)

		data, err := Read(path, test.mode)
		require.NoError(t, err)

		require.Zero(t, bytes.Compare(data, expectedData), "read bytes must be equal to those written")
	}
}
