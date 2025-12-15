//go:build !windows

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
	"syscall"

	"github.com/google/renameio/v2"
	"github.com/gravitational/trace"
)

// SetRequiredUmask sets the umask to match the systemd umask that the teleport-update service will execute with.
// This ensures consistent file permissions.
// NOTE: This must be run in main.go before any goroutines that create files are started.
func SetRequiredUmask(ctx context.Context, log *slog.Logger) {
	warnUmask(ctx, log, syscall.Umask(requiredUmask))
}

// writeAtomicWithinDir atomically creates a new file with renameio, while ensuring that temporary
// files use the same directory as the target file (with format: .[base][randints]).
// This ensures that SELinux contexts for important files are set correctly.
// writeAtomicWithinDir provider a callback with a writer.
func writeAtomicWithinDir(filename string, perm os.FileMode, fn func(io.Writer) error) error {
	opts := []renameio.Option{
		renameio.WithPermissions(perm),
		renameio.WithExistingPermissions(),
		renameio.WithTempDir(filepath.Dir(filename)),
	}
	f, err := renameio.NewPendingFile(filename, opts...)
	if err != nil {
		return trace.Wrap(err)
	}
	defer f.Cleanup()

	if err := fn(f); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(f.CloseAtomicallyReplace())
}

// writeFileAtomicWithinDir atomically creates a new file with renameio, while ensuring that temporary
// files use the same directory as the target file (with format: .[base][randints]).
// This ensures that SELinux contexts for important files are set correctly.
func writeFileAtomicWithinDir(filename string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(filename)
	err := renameio.WriteFile(filename, data, perm, renameio.WithTempDir(dir))
	return trace.Wrap(err)
}

// atomicSymlink atomically replaces a symlink.
func atomicSymlink(oldpath, newpath string) error {
	return trace.Wrap(renameio.Symlink(oldpath, newpath))
}
