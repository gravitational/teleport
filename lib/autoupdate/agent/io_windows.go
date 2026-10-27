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

package agent

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
)

// SetRequiredUmask does nothing on Windows, which does not support umask.
func SetRequiredUmask(ctx context.Context, log *slog.Logger) {
	return
}

func writeAtomicWithinDir(filename string, perm os.FileMode, fn func(io.Writer) error) error {
	tmp, err := os.CreateTemp(filepath.Dir(filename), filepath.Base(filename))
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	if err = fn(tmp); err != nil {
		return trace.ConvertSystemError(err)
	}
	// Flush the contents before the move, so that a crash can't leave filename
	// pointing at a file whose data never reached the disk.
	if err = tmp.Sync(); err != nil {
		return trace.ConvertSystemError(err)
	}
	if err = tmp.Close(); err != nil {
		return trace.ConvertSystemError(err)
	}
	// There's no way to do atomic rename in Windows, we do best effort here.
	if err = os.Rename(tmp.Name(), filename); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

func writeFileAtomicWithinDir(filename string, data []byte, perm os.FileMode) error {
	return writeAtomicWithinDir(filename, perm, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

func atomicSymlink(oldpath, newpath string) error {
	return trace.NotImplemented("atomic symlink not supported on Windows")
}
