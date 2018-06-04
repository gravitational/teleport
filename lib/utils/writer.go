/*
Copyright 2018 Gravitational, Inc.

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

import (
	"io"
)

// NewBroadcastWriter returns new broadcast writer
func NewBroadcastWriter(writers ...io.Writer) *BroadcastWriter {
	return &BroadcastWriter{
		writers: writers,
	}
}

// BroadcastWriter broadcasts all writes to all writers
type BroadcastWriter struct {
	writers []io.Writer
}

// Write multiplexes the input to multiple sub-writers. If any of the write
// fails, it won't attempt to write to other writers
func (w *BroadcastWriter) Write(p []byte) (n int, err error) {
	for _, writer := range w.writers {
		n, err = writer.Write(p)
		if err != nil {
			return
		}
		if n != len(p) {
			err = io.ErrShortWrite
			return
		}
	}
	return len(p), nil
}
