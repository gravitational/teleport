// Copyright 2022 Gravitational, Inc
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

package prompt

import (
	"context"
	"io"
	"os"

	"github.com/gravitational/trace"
)

// StdinSync returns a synchronous reader to os.Stdin.
// It is safe for use mixed with other methods that read Stdin, albeit not
// concurrently, but doesn't respect context cancelletion.
func StdinSync() *SyncReader {
	return &SyncReader{Reader: os.Stdin}
}

// SyncReader is a synchronous version of ContextReader.
// Its main advantage is that it allows multiple sources to read from the same
// underlying reader, albeit not concurrently.
type SyncReader struct {
	Reader io.Reader
}

// ReadContext reads a chunk from the underlying reader.
// It does not respect context cancellation after the read starts.
func (c *SyncReader) ReadContext(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	buf := make([]byte, 4*1024) // 4kB, matches Linux page size.
	n, err := c.Reader.Read(buf)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	buf = buf[:n]
	return buf, nil
}
