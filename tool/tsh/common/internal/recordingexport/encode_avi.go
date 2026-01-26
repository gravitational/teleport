/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

package recordingexport

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"

	"github.com/gravitational/trace"
	"github.com/icza/mjpeg"
)

// AVIEncoder encodes an AVI video file using the motion JPEG codec.
// It is slow, produces large files, and often needs to split a single
// recording into multiple files, but is implemented in pure Go
// and available on all systems.
type AVIEncoder struct {
	currentWriter mjpeg.AviWriter
	buf           *bytes.Buffer

	maxWidth, maxHeight int32
	framesPerSecond     int32

	currentFileCount int
	outputPrefix     string
}

// NewAVIEncoder produces AVI files with names based on the specified output prefix.
func NewAVIEncoder(prefix string, maxWidth, maxHeight, fps int32) *AVIEncoder {
	return &AVIEncoder{
		outputPrefix:    prefix,
		buf:             new(bytes.Buffer),
		maxWidth:        maxWidth,
		maxHeight:       maxHeight,
		framesPerSecond: fps,
	}
}

func (a *AVIEncoder) EmitFrames(img image.Image, count int) error {
	a.buf.Reset()
	if err := jpeg.Encode(a.buf, img, nil /* options */); err != nil {
		return trace.Wrap(err)
	}

	if a.currentWriter == nil {
		filename := makeAVIFileName(a.outputPrefix, a.currentFileCount)

		var err error
		a.currentWriter, err = mjpeg.New(filename, a.maxWidth, a.maxHeight, a.framesPerSecond)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	for range count {
		err := a.currentWriter.AddFrame(a.buf.Bytes())
		if errors.Is(err, mjpeg.ErrTooLarge) {
			// file too large - open a new one
			if err := a.currentWriter.Close(); err != nil {
				return trace.WrapWithMessage(err, "failed to write partial recording")
			}
			// TODO: indicate that a file was written
			a.currentFileCount++

			filename := makeAVIFileName(a.outputPrefix, a.currentFileCount)
			a.currentWriter, err = mjpeg.New(filename, a.maxWidth, a.maxHeight, a.framesPerSecond)
			if err != nil {
				return trace.Wrap(err)
			}

			// re-emit the frame to the new file
			if err := a.currentWriter.AddFrame(a.buf.Bytes()); err != nil {
				return trace.Wrap(err)
			}
		} else if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (a *AVIEncoder) Close() error {
	if a.currentWriter == nil {
		return nil
	}
	if err := a.currentWriter.Close(); err != nil {
		return trace.Wrap(err)
	}
	// TODO indicate that file was written
	return nil
}

func makeAVIFileName(prefix string, currentFile int) string {
	if currentFile == 0 {
		return prefix + ".avi"
	}

	return fmt.Sprintf("%v-%d.avi", prefix, currentFile)
}
