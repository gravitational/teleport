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

	bs, err := virtualFS.ReadFile(filename)
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

	_, err = virtualFS.ReadFile(filename)
	require.ErrorIs(t, err, fs.ErrNotExist)

	_, err = virtualFS.Stat(filename)
	require.ErrorIs(t, err, fs.ErrNotExist)

}
