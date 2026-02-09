/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package reexec

import (
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStartPreservesExtraFilesAndPlaceholders(t *testing.T) {
	ctx := t.Context()

	cmd := helperCommand(t)

	reexecCmd, err := NewReexecCommand(&Config{})
	require.NoError(t, err)

	extraExisting, err := os.CreateTemp(t.TempDir(), "reexec-existing-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = extraExisting.Close() })

	extraChild, err := os.CreateTemp(t.TempDir(), "reexec-child-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = extraChild.Close() })

	cmd.ExtraFiles = []*os.File{extraExisting}
	reexecCmd.AddChildPipe(nil)
	reexecCmd.AddChildPipe(extraChild)

	require.NoError(t, reexecCmd.Start(ctx))
	t.Cleanup(func() { _ = reexecCmd.Wait() })

	var existingCount int
	var childCount int
	for _, file := range cmd.ExtraFiles {
		require.NotNil(t, file)
		if file == extraExisting {
			existingCount++
		}
		if file == extraChild {
			childCount++
		}
	}

	require.Equal(t, 1, existingCount)
	require.Equal(t, 1, childCount)
}

func helperCommand(t *testing.T) *exec.Cmd {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=TestReexecHelperProcess", "--")
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

func TestReexecHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	cfg := os.NewFile(3, "config")
	if cfg != nil {
		_, _ = io.Copy(io.Discard, cfg)
		_ = cfg.Close()
	}

	os.Exit(0)
}
