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

package agent

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"
)

// progressLogger logs progress of any data written as it approaches max.
// max(lines, call_count(Write)) lines are written for each multiple of max.
// progressLogger uses the variability of chunk size as a proxy for speed, and avoids
// logging extraneous lines that do not improve UX for waiting humans.
type progressLogger struct {
	ctx   context.Context
	log   *slog.Logger
	level slog.Level
	name  string
	max   int
	lines int

	l int
	n int
}

func (w *progressLogger) Write(p []byte) (n int, err error) {
	w.n += len(p)
	if w.n >= w.max*(w.l+1)/w.lines {
		w.log.Log(w.ctx, w.level, "Downloading",
			"file", w.name,
			"progress", fmt.Sprintf("%d%%", w.n*100/w.max),
		)
		w.l++
	}
	return len(p), nil
}

// lineLogger logs each line written to it.
type lineLogger struct {
	ctx    context.Context
	log    *slog.Logger
	level  slog.Level
	prefix string

	last bytes.Buffer
}

func (w *lineLogger) out(s string) {
	if !w.log.Handler().Enabled(w.ctx, w.level) {
		return
	}

	//nolint:sloglint // msg is appended with prefix
	w.log.Log(w.ctx, w.level, w.prefix+s)
}

func (w *lineLogger) Write(p []byte) (n int, err error) {
	lines := bytes.Split(p, []byte("\n"))
	// Finish writing line
	if len(lines) > 0 {
		n, err = w.last.Write(lines[0])
		lines = lines[1:]
	}
	// Quit if no newline
	if len(lines) == 0 || err != nil {
		return n, trace.Wrap(err)
	}

	// Newline found, log line
	w.out(w.last.String())
	n += 1
	w.last.Reset()

	// Log lines that are already newline-terminated
	for _, line := range lines[:len(lines)-1] {
		w.out(string(line))
		n += len(line) + 1
	}

	// Store remaining line non-newline-terminated line.
	n2, err := w.last.Write(lines[len(lines)-1])
	n += n2
	return n, trace.Wrap(err)
}

// Flush logs any trailing bytes that were never terminated with a newline.
func (w *lineLogger) Flush() {
	if w.last.Len() == 0 {
		return
	}
	w.out(w.last.String())
	w.last.Reset()
}
