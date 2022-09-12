/*
Copyright 2015 Gravitational, Inc.

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

package web

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestLocalFS(t *testing.T) {
	t.Parallel()

	fs, err := NewDebugFileSystem("../../webassets/teleport")
	require.NoError(t, err)
	require.NotNil(t, fs)

	f, err := fs.Open("/index.html")
	require.NoError(t, err)
	bytes, err := io.ReadAll(f)
	require.NoError(t, err)

	html := string(bytes[:])
	require.NoError(t, f.Close())
	require.Equal(t, strings.Contains(html, `<script src="/web/config.js"></script>`), true)
	require.Equal(t, strings.Contains(html, `content="{{ .XCSRF }}"`), true)
}

func TestZipFS(t *testing.T) {
	t.Parallel()

	fs, err := readZipArchiveAt("../../fixtures/assets.zip")
	require.NoError(t, err)
	require.NotNil(t, fs)

	// test simple full read:
	f, err := fs.Open("/index.html")
	require.NoError(t, err)
	bytes, err := io.ReadAll(f)
	require.NoError(t, err)
	require.Equal(t, len(bytes), 813)
	require.NoError(t, f.Close())

	// seek + read
	f, err = fs.Open("/index.html")
	require.NoError(t, err)
	defer f.Close()

	n, err := f.Seek(10, io.SeekStart)
	require.NoError(t, err)
	require.Equal(t, n, int64(10))

	bytes, err = io.ReadAll(f)
	require.NoError(t, err)
	require.Equal(t, len(bytes), 803)

	n, err = f.Seek(-50, io.SeekEnd)
	require.NoError(t, err)
	require.Equal(t, n, int64(763))
	bytes, err = io.ReadAll(f)
	require.NoError(t, err)
	require.Equal(t, len(bytes), 50)

	_, err = f.Seek(-50, io.SeekEnd)
	require.NoError(t, err)
	n, err = f.Seek(-50, io.SeekCurrent)
	require.NoError(t, err)
	require.Equal(t, n, int64(713))
	bytes, err = io.ReadAll(f)
	require.NoError(t, err)
	require.Equal(t, len(bytes), 100)
}

func readZipArchiveAt(path string) (ResourceMap, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// file needs to stay open for http.FileSystem reads to work
	//
	// feed the binary into the zip reader and enumerate all files
	// found in the attached zip file:
	info, err := file.Stat()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return readZipArchive(file, info.Size())
}
