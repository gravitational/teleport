//go:build !windows

/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package packaging

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/integration/helpers/archive"
)

// TestPackaging verifies un-archiving of all supported teleport package formats.
func TestPackaging(t *testing.T) {
	script := "#!/bin/sh\necho test"
	sourceDir := t.TempDir()

	// Create test script for packaging in relative path `teleport\bin` to ensure that
	// binaries going to be identified and extracted flatten to `extractDir`.
	binPath := filepath.Join(sourceDir, "teleport", "bin")
	require.NoError(t, os.MkdirAll(binPath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(binPath, "tsh"), []byte(script), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(binPath, "tctl"), []byte(script), 0o755))

	ctx := context.Background()

	t.Run("tar.gz", func(t *testing.T) {
		toolsDir := t.TempDir()
		extractDir, err := os.MkdirTemp(toolsDir, "extract")
		require.NoError(t, err)

		archivePath := filepath.Join(toolsDir, "tsh.tar.gz")
		err = archive.CompressDirToTarGzFile(ctx, sourceDir, archivePath, archive.WithNoCompress())
		require.NoError(t, err)
		require.FileExists(t, archivePath, "archive not created")

		// For the .tar.gz format we extract app by app to check that content discard is not required.
		toolsMap, err := replaceTarGz(archivePath, extractDir, []string{"tsh", "tctl"})
		require.NoError(t, err)
		for tool, path := range toolsMap {
			assert.FileExists(t, filepath.Join(extractDir, path), fmt.Sprintf("script: %q not found", tool))
			data, err := os.ReadFile(filepath.Join(extractDir, path))
			require.NoError(t, err)
			assert.Equal(t, script, string(data))
		}
	})

	t.Run("pkg", func(t *testing.T) {
		if runtime.GOOS != "darwin" {
			t.Skip("unsupported platform")
		}
		toolsDir := t.TempDir()
		extractDir, err := os.MkdirTemp(toolsDir, "extract")
		require.NoError(t, err)

		archivePath := filepath.Join(toolsDir, "tsh.pkg")
		err = archive.CompressDirToPkgFile(ctx, sourceDir, archivePath, "com.example.pkgtest")
		require.NoError(t, err)
		require.FileExists(t, archivePath, "archive not created")

		toolsMap, err := replacePkg(archivePath, filepath.Join(extractDir, "apps"), []string{"tsh", "tctl"})
		require.NoError(t, err)
		for tool, path := range toolsMap {
			assert.FileExists(t, filepath.Join(extractDir, "apps", path), fmt.Sprintf("script: %q not found", tool))
			data, err := os.ReadFile(filepath.Join(extractDir, "apps", path))
			require.NoError(t, err)
			assert.Equal(t, script, string(data))
		}
	})

	t.Run("zip", func(t *testing.T) {
		toolsDir := t.TempDir()
		extractDir, err := os.MkdirTemp(toolsDir, "extract")
		require.NoError(t, err)

		archivePath := filepath.Join(toolsDir, "tsh.zip")
		err = archive.CompressDirToZipFile(ctx, sourceDir, archivePath)
		require.NoError(t, err)
		require.FileExists(t, archivePath, "archive not created")

		toolsMap, err := replaceZip(archivePath, extractDir, []string{"tsh", "tctl"})
		require.NoError(t, err)
		for tool, path := range toolsMap {
			assert.FileExists(t, filepath.Join(extractDir, path), fmt.Sprintf("script: %q not found", tool))
			data, err := os.ReadFile(filepath.Join(extractDir, path))
			require.NoError(t, err)
			assert.Equal(t, script, string(data))
		}
	})
}

// TestRemoveWithSuffix verifies that helper for the cleanup removes directories
func TestRemoveWithSuffix(t *testing.T) {
	testDir := t.TempDir()
	dirForRemove := "test-extract-pkg"

	// Creates directories `test/test-extract-pkg/test-extract-pkg` with exact names
	// to ensure that only root one going to be removed recursively without any error.
	path := filepath.Join(testDir, dirForRemove, dirForRemove)
	require.NoError(t, os.MkdirAll(path, 0o755))
	// Also we create the directory that needs to be skipped, and it matches the remove
	// pattern `test/skip-test-extract-pkg/test-extract-pkg`.
	skipName := "skip-" + dirForRemove
	skipPath := filepath.Join(testDir, skipName)
	dirInSkipPath := filepath.Join(skipPath, dirForRemove)
	require.NoError(t, os.MkdirAll(skipPath, 0o755))

	err := RemoveWithSuffix(testDir, dirForRemove, []string{skipName})
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(testDir, dirForRemove))
	assert.True(t, os.IsNotExist(err))

	filePath, err := os.Stat(skipPath)
	require.NoError(t, err)
	assert.True(t, filePath.IsDir())

	_, err = os.Stat(dirInSkipPath)
	assert.True(t, os.IsNotExist(err))
}
