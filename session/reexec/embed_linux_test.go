// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package reexec

import (
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/klauspost/compress/gzip"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestLoadEmbeddedReexec(t *testing.T) {
	t.Parallel()

	truePath, err := exec.LookPath("true")
	require.NoError(t, err)

	trueBin, err := os.ReadFile(truePath)
	require.NoError(t, err)

	b := new(strings.Builder)
	gzwriter := gzip.NewWriter(b)
	_, err = gzwriter.Write(trueBin)
	require.NoError(t, err)
	require.NoError(t, gzwriter.Close())

	trueGZ := b.String()

	mf, err := loadEmbeddedReexec("true", trueGZ)
	require.NoError(t, err)
	defer mf.Close()

	r, err := unix.FcntlInt(mf.Fd(), unix.F_GET_SEALS, 0)
	require.NoError(t, err)
	require.NotZero(t, r&unix.F_SEAL_SEAL)
	require.NotZero(t, r&unix.F_SEAL_SHRINK)
	require.NotZero(t, r&unix.F_SEAL_GROW)
	require.NotZero(t, r&unix.F_SEAL_WRITE)

	if r&unix.F_SEAL_EXEC == 0 {
		t.Log("didn't get F_SEAL_EXEC on memfd, ignoring since the kernel might be old")
	}

	fi, err := mf.Stat()
	require.NoError(t, err)
	require.Equal(t, fs.FileMode(0o555), fi.Mode())

	// loadEmbeddedReexec should've already tested it, but let's be thorough
	err = (&exec.Cmd{
		Path: mf.Name(),
		Args: []string{"true"},
	}).Run()
	require.NoError(t, err)
}
