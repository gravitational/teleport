/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
