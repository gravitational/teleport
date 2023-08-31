// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestTarWriterProducesValidEmptyTarfile(t *testing.T) {
	var tarball bytes.Buffer
	uut := newTarWriter(&tarball, clockwork.NewFakeClock())
	require.NoError(t, uut.Close())

	reader := tar.NewReader(&tarball)
	_, err := reader.Next()
	require.ErrorIs(t, err, io.EOF)
}

func TestTarWriterStatAlwaysReturnsNotFound(t *testing.T) {
	var tarball bytes.Buffer
	uut := newTarWriter(&tarball, clockwork.NewFakeClock())
	defer uut.Close()

	_, err := uut.Stat("somefile")
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestTarWriterFileWrite(t *testing.T) {
	testContent := []struct {
		filename string
		filemode int64
		content  []byte
	}{
		{
			filename: "alpha",
			filemode: 0600,
			content:  []byte("I am the very model of a modern Major-General"),
		}, {
			filename: "beta",
			filemode: 0777,
			content:  []byte("I've information vegetable, animal, and mineral"),
		}, {
			filename: "gamma",
			filemode: 0666,
			content:  []byte("I know the kings of England, and I quote the fights historical"),
		},
	}

	// The TAR format doesn't support sub-second timestamps, so make sure
	// that the nsec component is 0
	t0 := time.Date(1976, time.July, 29, 2, 30, 01, 0, time.Local)
	fakeClock := clockwork.NewFakeClockAt(t0)

	// GIVEN a tar file created by writing data to a identity file writer
	var tarball bytes.Buffer
	uut := newTarWriter(&tarball, fakeClock)
	for _, f := range testContent {
		require.NoError(t, uut.WriteFile(f.filename, f.content, os.FileMode(f.filemode)))
		fakeClock.Advance(24 * time.Hour)
	}
	require.NoError(t, uut.Close())

	// WHEN I read the file
	i := 0
	reader := tar.NewReader(&tarball)
	expectedTime := t0

	// Expect all the file content and metadata is preserved
	var err error
	for err != io.EOF {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}

		require.Equal(t, testContent[i].filename, header.Name)
		require.Equal(t, testContent[i].filemode, header.Mode)
		require.Equal(t, len(testContent[i].content), int(header.Size))
		require.Equal(t, expectedTime, header.ModTime)

		actualContent := make([]byte, header.Size)
		_, err = reader.Read(actualContent)
		if err != nil && err != io.EOF {
			require.NoError(t, err)
		}
		require.Equal(t, testContent[i].content, actualContent)

		expectedTime = expectedTime.Add(24 * time.Hour)
		i++
	}

	require.Equal(t, i, len(testContent))
}
