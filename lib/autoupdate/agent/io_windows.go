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

	"github.com/gravitational/trace"
)

// SetRequiredUmask does nothing on Windows, which does not support umask.
func SetRequiredUmask(ctx context.Context, log *slog.Logger) {
	return
}

func writeAtomicWithinDir(filename string, perm os.FileMode, fn func(io.Writer) error) error {
	return trace.NotImplemented("atomic write not supported on Windows")
}

func writeFileAtomicWithinDir(filename string, data []byte, perm os.FileMode) error {
	return trace.NotImplemented("atomic write not supported on Windows")
}

func atomicSymlink(oldpath, newpath string) error {
	return trace.NotImplemented("atomic symlink not supported on Windows")
}
