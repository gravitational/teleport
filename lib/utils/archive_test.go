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
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"io/fs"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
)

type mockFileReader struct {
	files map[string]*InMemoryFile
}

func (m mockFileReader) ReadFile(name string) ([]byte, error) {
	f, found := m.files[name]
	if !found {
		return nil, fs.ErrNotExist
	}

	return f.Content(), nil
}

func (m mockFileReader) Open(name string) (fs.File, error) {
	return nil, trace.NotImplemented("Open is not implemented")
}

func (m mockFileReader) Stat(name string) (fs.FileInfo, error) {
	f, found := m.files[name]
	if !found {
		return nil, fs.ErrNotExist
	}

	return f, nil
}

// CompressAsTarGzArchive creates a Tar Gzip archive in memory, reading the files using the provided file reader
func TestCompressAsTarGzArchive(t *testing.T) {
	tests := []struct {
		name       string
		fileNames  []string
		fsContents map[string]*InMemoryFile
		assert     require.ErrorAssertionFunc
	}{
		{
			name:       "File Not Exists bubbles up",
			fileNames:  []string{"not", "found"},
			fsContents: map[string]*InMemoryFile{},
			assert: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.ErrorIs(t, err, fs.ErrNotExist)
			},
		},
		{
			name:      "Archive is created",
			fileNames: []string{"file1", "file2"},
			fsContents: map[string]*InMemoryFile{
				"file1": NewInMemoryFile("file1", teleport.FileMaskOwnerOnly, time.Now(), []byte("contentsfile1")),
				"file2": NewInMemoryFile("file2", teleport.FileMaskOwnerOnly, time.Now(), []byte("contentsfile2")),
			},
			assert: require.NoError,
		},
	}

	for _, tt := range tests {
		fileReader := mockFileReader{
			files: tt.fsContents,
		}
		bs, err := CompressTarGzArchive(tt.fileNames, fileReader)
		tt.assert(t, err)
		if err != nil {
			continue
		}

		gzipReader, err := gzip.NewReader(bs)
		require.NoError(t, err)

		tarContentFileNames := []string{}

		tarReader := tar.NewReader(gzipReader)
		for {
			header, err := tarReader.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			require.NoError(t, err)
			require.Equal(t, byte(tar.TypeReg), header.Typeflag)

			tarContentFileNames = append(tarContentFileNames, header.Name)
			require.Contains(t, tt.fsContents, header.Name)

			gotBytes, err := io.ReadAll(tarReader)
			require.NoError(t, err)

			require.Equal(t, tt.fsContents[header.Name].content, gotBytes)
			require.Equal(t, tt.fsContents[header.Name].mode, fs.FileMode(header.Mode))
		}
		require.ElementsMatch(t, tarContentFileNames, tt.fileNames)
	}
}
