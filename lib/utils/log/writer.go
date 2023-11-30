// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import (
	"io"
	"sync"
)

// SharedWriter is an [io.Writer] implementation that protects
// writes with a mutex. This allows a single [io.Writer] to be shared
// by both logrus and slog without their output clobbering each other.
type SharedWriter struct {
	mu sync.Mutex
	io.Writer
}

func (s *SharedWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.Writer.Write(p)
}

// NewSharedWriter wraps the provided [io.Writer] in a writer that
// is thread safe.
func NewSharedWriter(w io.Writer) *SharedWriter {
	return &SharedWriter{Writer: w}
}
