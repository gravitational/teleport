/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

package recordingexport

import (
	"image"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAVIEncoderOutputFiles(t *testing.T) {
	t.Parallel()

	prefix := filepath.Join(t.TempDir(), "session")
	enc := NewAVIEncoder(prefix, 64, 64, 30)

	// No frames written yet, so no files exist.
	require.Empty(t, enc.OutputFiles())

	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	require.NoError(t, enc.EmitFrames(img, 1))
	require.NoError(t, enc.Close())

	// A single file should be reported, and it should actually exist on disk.
	files := enc.OutputFiles()
	require.Equal(t, []string{prefix + ".avi"}, files)
	require.FileExists(t, files[0])
}
