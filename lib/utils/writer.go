/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package utils

// CaptureNBytesWriter is an io.Writer thats captures up to first n bytes
// of the incoming data in memory, and then it ignores the rest of the incoming
// data.
type CaptureNBytesWriter struct {
	capture      []byte
	maxRemaining int
}

// NewCaptureNBytesWriter creates a new CaptureNBytesWriter.
func NewCaptureNBytesWriter(max int) *CaptureNBytesWriter {
	return &CaptureNBytesWriter{
		maxRemaining: max,
	}
}

// Write implements io.Writer.
func (w *CaptureNBytesWriter) Write(p []byte) (int, error) {
	if w.maxRemaining > 0 {
		capture := p[:]
		if len(capture) > w.maxRemaining {
			capture = capture[:w.maxRemaining]
		}

		w.capture = append(w.capture, capture...)
		w.maxRemaining -= len(capture)
	}

	// Always pretend to be successful.
	return len(p), nil
}

// Bytes returns all captured bytes.
func (w CaptureNBytesWriter) Bytes() []byte {
	return w.capture
}
