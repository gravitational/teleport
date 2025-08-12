/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package filesessions

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestPlainFileOpsReservations(t *testing.T) {
	ctx := context.Background()
	rec := NewPlainFileRecorder(logtest.NewLogger(), os.OpenFile)
	base := t.TempDir()
	reservation := filepath.Join(base, "reservation")
	var fileSize int64 = 512

	err := rec.ReservePart(ctx, reservation, fileSize)
	require.NoError(t, err)

	info, err := os.Stat(reservation)
	require.NoError(t, err)
	require.Equal(t, fileSize, info.Size())

	buf := bytes.NewBufferString("testing")
	expectedLen := buf.Len()
	err = rec.WritePart(ctx, reservation, buf)
	require.NoError(t, err)

	info, err = os.Stat(reservation)
	require.NoError(t, err)
	require.Equal(t, int64(expectedLen), info.Size())
}

func TestPlainFileOpsCombineParts(t *testing.T) {
	ctx := context.Background()
	rec := NewPlainFileRecorder(logtest.NewLogger(), os.OpenFile)
	base := t.TempDir()
	parts := []string{"part1", "part2", "part3"}
	partPaths := make([]string, len(parts))
	for idx, part := range parts {
		partPaths[idx] = filepath.Join(base, part)
		f, err := os.Create(partPaths[idx])
		require.NoError(t, err)

		_, err = f.WriteString(part)
		require.NoError(t, err)
	}

	dst := bytes.NewBuffer(nil)
	err := rec.CombineParts(ctx, dst, slices.Values(partPaths))
	require.NoError(t, err)

	output, _ := dst.ReadString(0)

	require.Equal(t, strings.Join(parts, ""), output)
}
