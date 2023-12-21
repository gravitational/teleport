/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"archive/tar"
	"bytes"
	"io"
	"io/fs"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestTarWriterProducesValidEmptyTarfile(t *testing.T) {
	uut := newTarWriter(clockwork.NewFakeClock())
	var tarball bytes.Buffer
	require.NoError(t, uut.Archive(&tarball))

	reader := tar.NewReader(&tarball)
	_, err := reader.Next()
	require.ErrorIs(t, err, io.EOF)
}

func TestTarWriterStatReturnsNotFoundOnEmptyArchive(t *testing.T) {
	uut := newTarWriter(clockwork.NewFakeClock())
	_, err := uut.Stat("somefile")
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestTarWriter(t *testing.T) {
	// The TAR format doesn't support sub-second timestamps, so make sure
	// that the nsec component is 0
	t0 := time.Date(1976, time.July, 29, 2, 30, 01, 0, time.Local)

	testContent := map[string]struct {
		filemode int64
		content  []byte
	}{
		"alpha": {
			filemode: 0600,
			content:  []byte("I am the very model of a modern Major-General"),
		},
		"beta": {
			filemode: 0777,
			content:  []byte("I've information vegetable, animal, and mineral"),
		},
		"gamma": {
			filemode: 0666,
			content:  []byte("I know the kings of England, and I quote the fights historical"),
		},
		"delta": {
			filemode: 644,
			content:  []byte("From Marathon to Waterloo, in order categorical"),
		},
	}

	filenames := make([]string, 0, len(testContent))
	for fn := range testContent {
		filenames = append(filenames, fn)
	}
	sort.Strings(filenames)

	newPopulatedConfigWriter := func(innerT *testing.T) (*tarWriter, map[string]time.Time) {
		modTimes := map[string]time.Time{}
		fakeClock := clockwork.NewFakeClockAt(t0)
		uut := newTarWriter(fakeClock)
		for fn, f := range testContent {
			err := uut.WriteFile(fn, f.content, os.FileMode(f.filemode))
			require.NoError(innerT, err)

			modTimes[fn] = fakeClock.Now()
			fakeClock.Advance(24 * time.Hour)
		}
		var tarball bytes.Buffer
		require.NoError(innerT, uut.Archive(&tarball))
		return uut, modTimes
	}

	t.Run("simple write", func(t *testing.T) {
		// Given a tarball created by writing content to a ConfigWriter
		var tarball bytes.Buffer
		uut, modTimes := newPopulatedConfigWriter(t)
		err := uut.Archive(&tarball)
		require.NoError(t, err)

		// WHEN I read the file
		i := 0
		reader := tar.NewReader(&tarball)

		// Expect all the file content and metadata is preserved
		for err != io.EOF {
			header, err := reader.Next()
			if err == io.EOF {
				break
			}

			// Expect that the files will come out of the TAR ordered by
			// filename, and that their metadata is preserved
			filename := filenames[i]
			expected := testContent[filename]
			require.Equal(t, filename, header.Name)
			require.Equal(t, expected.filemode, header.Mode)
			require.Len(t, expected.content, int(header.Size))
			require.Equal(t, modTimes[filename], header.ModTime)

			actualContent := make([]byte, header.Size)
			_, err = reader.Read(actualContent)
			if err != nil && err != io.EOF {
				require.NoError(t, err)
			}
			require.Equal(t, expected.content, actualContent)
			i++
		}

		require.Len(t, testContent, i)
	})

	t.Run("delete file", func(t *testing.T) {
		// Given populated ConfigWriter
		uut, _ := newPopulatedConfigWriter(t)

		// When I delete a file from the config bundle, expect that the
		// operation succeeds
		require.NoError(t, uut.Remove("gamma"))

		// When I create an archive from the modified ConfigWriter, expect that
		// the operation succeeds
		var tarball bytes.Buffer
		err := uut.Archive(&tarball)
		require.NoError(t, err)

		// When I list all of the files in the resulting archive...
		reader := tar.NewReader(&tarball)
		archivedFiles := []string{}
		for err != io.EOF {
			header, err := reader.Next()
			if err == io.EOF {
				break
			}
			archivedFiles = append(archivedFiles, header.Name)

			// Skip over the actual content data
			io.CopyN(io.Discard, reader, header.Size)
		}

		// Expect that the name of the deleted file does not appear in the
		// archive listing, but all the other expected names do.
		require.Len(t, archivedFiles, len(filenames)-1)
		require.NotContains(t, archivedFiles, "gamma")
		for _, fn := range filenames {
			if fn != "gamma" {
				require.Contains(t, archivedFiles, fn)
			}
		}
	})

	t.Run("delete missing file", func(t *testing.T) {
		// Given populated ConfigWriter
		uut, _ := newPopulatedConfigWriter(t)

		// When I delete file that doesn't exist, expect the operation to
		// succeed; missing files are ot considered an error
		err := uut.Remove("omega")
		require.NoError(t, err)
	})

	t.Run("stat file", func(t *testing.T) {
		// Given populated ConfigWriter
		uut, modTimes := newPopulatedConfigWriter(t)

		// When I stat file that does exist, expect the operation to succeed
		info, err := uut.Stat("beta")
		require.NoError(t, err)

		// Expect also that the file info is correct
		expected := testContent["beta"]
		require.Equal(t, "beta", info.Name())
		require.Equal(t, fs.FileMode(expected.filemode), info.Mode())
		require.Len(t, expected.content, int(info.Size()))
		require.Equal(t, modTimes["beta"], info.ModTime())
	})

	t.Run("stat missing file", func(t *testing.T) {
		// Given populated ConfigWriter
		uut, _ := newPopulatedConfigWriter(t)

		// When I stat file that doesn't exist, expect the operation to
		// return not found
		_, err := uut.Stat("omega")
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})
}
