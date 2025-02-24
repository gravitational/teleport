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

package tools

import (
	"fmt"
	"io"
	"strings"

	"github.com/gravitational/trace"
)

type progressWriter struct {
	n        int64
	limit    int64
	size     int
	progress int
}

// newProgressWriter creates progress writer instance and prints empty
// progress bar right after initialisation.
func newProgressWriter(size int) (*progressWriter, func()) {
	pw := &progressWriter{size: size}
	pw.Print(0)
	return pw, func() {
		fmt.Print("\n")
	}
}

// Print prints the update progress bar with `n` bricks.
func (w *progressWriter) Print(n int) {
	bricks := strings.Repeat("▒", n) + strings.Repeat(" ", w.size-n)
	fmt.Print("\rUpdate progress: [" + bricks + "] (Ctrl-C to cancel update)")
}

func (w *progressWriter) Write(p []byte) (int, error) {
	if w.limit == 0 || w.size == 0 {
		return len(p), nil
	}

	w.n += int64(len(p))
	bricks := int((w.n*100)/w.limit) / w.size
	if w.progress != bricks {
		w.Print(bricks)
		w.progress = bricks
	}

	return len(p), nil
}

// CopyLimit sets the limit of writing bytes to the progress writer and initiate copying process.
func (w *progressWriter) CopyLimit(dst io.Writer, src io.Reader, limit int64) (written int64, err error) {
	if limit < 0 {
		n, err := io.Copy(dst, io.TeeReader(src, w))
		w.Print(w.size)
		return n, trace.Wrap(err)
	}

	w.limit = limit
	n, err := io.CopyN(dst, io.TeeReader(src, w), limit)
	return n, trace.Wrap(err)
}
