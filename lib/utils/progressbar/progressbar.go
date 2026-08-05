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

// Package progressbar provides a minimal terminal progress bar.
package progressbar

import (
	"fmt"
	"io"
	"strings"
)

// width is the number of cells used to draw the bar.
const width = 10

// Bar is a minimal progress bar that redraws in place using a carriage return.
//
// A Bar can be advanced either by discrete steps (Add) or as a byte counter: it
// implements io.ReadWriter, where Read and Write simply count len(p) and pass
// no data.
//
// All methods are safe to call on a nil *Bar, which disables progress reporting
// so callers can leave the bar unset.
type Bar struct {
	w       io.Writer
	desc    string
	total   int64
	current int64
	// lastPercent is the most recently drawn percentage, used to avoid unnecessary redraws
	lastPercent int
	finished    bool
}

// New returns a progress bar that advances toward total and draws to w, labeled
// with desc. total is the number of steps or bytes expected; pass a value <= 0
// when it is unknown, in which case the bar stays empty until Finish.
func New(total int64, desc string, w io.Writer) *Bar {
	b := &Bar{w: w, desc: desc, total: total, lastPercent: -1}
	b.render()
	return b
}

// Add advances the bar by n steps.
func (b *Bar) Add(n int) {
	if b == nil {
		return
	}
	b.advance(int64(n))
}

// Write implements io.Writer, advancing the bar by len(p) bytes. It counts only;
// the buffer is expected to already hold the transferred data.
func (b *Bar) Write(p []byte) (int, error) {
	if b == nil {
		return len(p), nil
	}
	b.advance(int64(len(p)))
	return len(p), nil
}

// Read implements io.Reader, advancing the bar by len(p) bytes. Like Write, it
// counts only and does not populate p.
func (b *Bar) Read(p []byte) (int, error) {
	if b == nil {
		return len(p), nil
	}
	b.advance(int64(len(p)))
	return len(p), nil
}

// Finish fills the bar to 100% and moves to the next line.
func (b *Bar) Finish() {
	if b == nil {
		return
	}
	b.finished = true
	b.render()
	fmt.Fprintln(b.w)
}

func (b *Bar) advance(n int64) {
	b.current += n

	if b.total > 0 && b.current > b.total {
		b.total = b.current
	}

	b.render()
}

func (b *Bar) percent() int {
	switch {
	case b.finished:
		return 100
	case b.total <= 0:
		return 0
	default:
		return min(int(b.current*100/b.total), 100)
	}
}

func (b *Bar) render() {
	percent := b.percent()
	if percent == b.lastPercent {
		return
	}
	b.lastPercent = percent

	filled := width * percent / 100
	bar := strings.Repeat("█", filled) + strings.Repeat(" ", width-filled)
	fmt.Fprintf(b.w, "\r%s [%s] %3d%%", b.desc, bar, percent)
}
