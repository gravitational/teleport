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
	"context"
	"io"
	"iter"
	"log/slog"
	"os"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

const (
	reservationFilePerm = 0600
	combinedFilePerm    = reservationFilePerm
)

// PlainFileRecorder interacts with the file system to write unencrypted recording parts to disk.
type PlainFileRecorder struct {
	log      *slog.Logger
	openFile utils.OpenFileWithFlagsFunc
}

var _ sessionFileRecorder = (*PlainFileRecorder)(nil)

// NewPlainFileRecorder returns a plaintext implementation of the SessionFileRecorder interface.
func NewPlainFileRecorder(log *slog.Logger, openFile utils.OpenFileWithFlagsFunc) *PlainFileRecorder {
	return &PlainFileRecorder{
		log:      log,
		openFile: openFile,
	}
}

// ReservePart creates a new file for recording session data with a reserved file size.
func (p *PlainFileRecorder) ReservePart(ctx context.Context, name string, size int64) (err error) {
	log := p.log.With("file", name, "size", size)

	f, err := p.openFile(name, os.O_WRONLY|os.O_CREATE, reservationFilePerm)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	defer func() {
		closeErr := f.Close()
		if closeErr != nil {
			log.ErrorContext(ctx, "failed to close reservation file", "error", closeErr)
		}

		if err = trace.NewAggregate(err, closeErr); err != nil {
			if rmErr := os.Remove(name); rmErr != nil {
				log.WarnContext(ctx, "failed to remove part file", "file", name, "error", rmErr)
			}
		}
	}()

	if err := f.Truncate(size); err != nil {
		return trace.ConvertSystemError(err)
	}

	return nil
}

// WritePart writes session data to an already Reserved file and truncates it to the new size.
func (p *PlainFileRecorder) WritePart(ctx context.Context, name string, data io.Reader) (err error) {
	log := p.log.With("file", name)

	// O_CREATE kepr for backwards compatibility only
	const flag = os.O_WRONLY | os.O_CREATE

	f, err := p.openFile(name, flag, reservationFilePerm)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	defer func() {
		closeErr := f.Close()
		if closeErr != nil {
			log.ErrorContext(ctx, "failed to close file part file after write", "error", closeErr)
		}

		if err = trace.NewAggregate(err, closeErr); err != nil {
			if rmErr := os.Remove(name); rmErr != nil {
				log.WarnContext(ctx, "failed to remove part file", "error", rmErr)
			}
		}
	}()

	n, err := io.Copy(f, data)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	if err := f.Truncate(n); err != nil {
		return trace.ConvertSystemError(err)
	}

	return nil
}

// CombineParts collects part files of a single session recording and combines them into a single io.Writer.
func (p *PlainFileRecorder) CombineParts(ctx context.Context, dst io.Writer, parts iter.Seq[string]) error {
	for part := range parts {
		partFile, err := p.openFile(part, os.O_RDONLY, 0)
		if err != nil {
			return trace.ConvertSystemError(err)
		}

		defer func() {
			if err := partFile.Close(); err != nil {
				p.log.ErrorContext(ctx, "failed to close part file during combine", "file", part, "error", err)
			}
		}()

		if _, err = io.Copy(dst, partFile); err != nil {
			return trace.ConvertSystemError(err)
		}
	}

	return nil
}
