/**
 * Copyright (C) 2024 Gravitational, Inc.
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

package ttyplayback

import (
	"github.com/hinshun/vt10x"
)

type Frame struct {
	Time   int64
	Lines  [][]vt10x.Glyph
	Cursor *Cursor
}

type Cursor struct {
	X int
	Y int
}

type FrameIterator struct {
	iter       EventIterator
	vt         vt10x.Terminal
	prevCursor *Cursor
	prevLines  [][]vt10x.Glyph
}

func (f *FrameIterator) Next() (*Frame, bool) {
	for {
		event, ok := f.iter.Next()
		if !ok {
			return nil, false
		}

		if event.Type != "print" {
			continue
		}

		_, _ = f.vt.Write([]byte(event.Data))

		x, y := f.vt.Cursor().X, f.vt.Cursor().Y
		cursor := &Cursor{X: x, Y: y}

		//cursorChanged := f.prevCursor == nil ||
		//	f.prevCursor.X != cursor.X ||
		//	f.prevCursor.Y != cursor.Y

		width, height := f.vt.Size()
		lines := make([][]vt10x.Glyph, height)

		//linesChanged := false
		for row := 0; row < height; row++ {
			lines[row] = make([]vt10x.Glyph, width)
			for col := 0; col < width; col++ {
				cell := f.vt.Cell(col, row)
				lines[row][col] = cell

				if f.prevLines == nil ||
					row >= len(f.prevLines) ||
					col >= len(f.prevLines[row]) ||
					f.prevLines[row][col].Char != lines[row][col].Char {
					//linesChanged = true
				}
			}
		}

		//if linesChanged || cursorChanged {
		f.prevCursor = cursor
		f.prevLines = lines

		return &Frame{
			Time:   event.Time,
			Lines:  lines,
			Cursor: cursor,
		}, true
		//} else {
		//	f.prevCursor = cursor
		//	log.Printf("skipping frame with no visual changes: %q", event.Data)
		//}
	}
}

func (f *FrameIterator) CollectAll() []Frame {
	var frames []Frame
	for {
		frame, ok := f.Next()
		if !ok {
			break
		}
		frames = append(frames, *frame)
	}
	return frames
}

func Frames(stdout EventIterator, width, height int) *FrameIterator {
	vt := vt10x.New(vt10x.WithSize(width, height))

	return &FrameIterator{
		iter:       stdout,
		vt:         vt,
		prevCursor: nil,
		prevLines:  nil,
	}
}
