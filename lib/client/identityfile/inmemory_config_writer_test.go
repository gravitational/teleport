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

package identityfile

import (
	"bytes"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestInMemory(t *testing.T) {
	virtualFS := NewInMemoryConfigWriter()

	content := bytes.Repeat([]byte("A"), 4000)
	filename := "test1"
	fileMode := os.FileMode(0644)
	fileSize := int64(len(content))

	err := virtualFS.WriteFile(filename, content, fileMode)
	require.NoError(t, err)

	bs, err := virtualFS.Read(filename)
	require.NoError(t, err)
	require.Equal(t, bs, content)

	fileStat, err := virtualFS.Stat(filename)
	require.NoError(t, err)
	require.Equal(t, fileStat.Name(), filename)
	require.Equal(t, fileStat.Mode(), fileMode)
	require.Equal(t, fileStat.Size(), fileSize)
	require.False(t, fileStat.IsDir())
	require.WithinDuration(t, fileStat.ModTime(), time.Now(), time.Second)

	err = virtualFS.Remove(filename)
	require.NoError(t, err)

	_, err = virtualFS.Read(filename)
	require.ErrorIs(t, err, fs.ErrNotExist)

	_, err = virtualFS.Stat(filename)
	require.ErrorIs(t, err, fs.ErrNotExist)

}
