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

package terminal

import (
	"strconv"
	"strings"

	"github.com/hinshun/vt10x"
)

type Frame struct {
	Ansi   string
	Time   int64
	Lines  [][]vt10x.Glyph
	Cursor *Cursor
	Cols   int
	Rows   int
}

type Cursor struct {
	X int
	Y int
}

type FrameIterator struct {
	iter EventIterator
	vt   vt10x.Terminal
}

func (f *FrameIterator) Next() (*Frame, bool) {
	for {
		event, ok := f.iter.Next()
		if !ok {
			return nil, false
		}

		if event.Type == "resize" {
			parts := strings.Split(event.Data, ":")
			if len(parts) != 2 {
				continue
			}

			width, err := strconv.Atoi(parts[0])
			if err != nil {
				continue
			}

			height, err := strconv.Atoi(parts[1])
			if err != nil {
				continue
			}

			f.vt.Resize(width, height)
		}

		if event.Type != "print" {
			continue
		}

		_, _ = f.vt.Write([]byte(event.Data))

		x, y := f.vt.Cursor().X, f.vt.Cursor().Y
		cursor := &Cursor{X: x, Y: y}

		width, height := f.vt.Size()
		lines := make([][]vt10x.Glyph, height)

		for row := 0; row < height; row++ {
			lines[row] = make([]vt10x.Glyph, width)
			for col := 0; col < width; col++ {
				cell := f.vt.Cell(col, row)
				lines[row][col] = cell
			}
		}

		ansi := dumpToAnsi(width, height, x, y, f.vt.CursorVisible(), lines)

		return &Frame{
			Time:   event.Time,
			Ansi:   ansi,
			Lines:  lines,
			Cols:   width,
			Rows:   height,
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

func Frames(stdout EventIterator) *FrameIterator {
	vt := vt10x.New()

	return &FrameIterator{
		iter: stdout,
		vt:   vt,
	}
}
