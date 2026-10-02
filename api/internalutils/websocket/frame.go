/*
Copyright 2026 Gravitational, Inc.

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

// Package websocket provides internal helpers for streaming WebSocket frames.
package websocket

import (
	"io"

	"github.com/gobwas/ws"
)

// FrameReadState tracks progress while a WebSocket frame payload is streamed
// across reads without allocating a buffer at its declared length. Its zero
// value is ready to begin a frame.
type FrameReadState struct {
	reader     io.Reader
	header     ws.Header
	remaining  int64
	maskOffset int
}

// Begin binds a frame to reader so FrameReadState can stream its payload as an io.Reader.
func (s *FrameReadState) Begin(header ws.Header, reader io.Reader) {
	*s = FrameReadState{
		reader:    reader,
		header:    header,
		remaining: header.Length,
	}
}

// OpCode returns the current frame's operation code.
func (s *FrameReadState) OpCode() ws.OpCode {
	return s.header.OpCode
}

// DeclaredPayloadLength returns the payload length declared in the current
// frame's header.
func (s *FrameReadState) DeclaredPayloadLength() int64 {
	return s.header.Length
}

// Remaining returns the unread payload length.
func (s *FrameReadState) Remaining() int64 {
	return s.remaining
}

// Read reads up to len(b) bytes from the current frame payload.
func (s *FrameReadState) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	if s.remaining == 0 {
		return 0, io.EOF
	}

	want := min(int64(len(b)), s.remaining)
	n, err := s.reader.Read(b[:int(want)])
	if n > 0 {
		if s.header.Masked {
			ws.Cipher(b[:n], s.header.Mask, s.maskOffset)
		}
		// Only the position within the four-byte mask matters. Keeping it
		// normalized avoids overflow even for frames with very large declared
		// lengths.
		s.maskOffset = (s.maskOffset + n) % len(s.header.Mask)
		s.remaining -= int64(n)
	}
	return n, err
}

// Discard drains the current frame payload using scratch. The contents written
// to scratch are intentionally ignored.
func (s *FrameReadState) Discard(scratch []byte) error {
	for s.remaining > 0 {
		n, err := s.Read(scratch)
		if s.remaining == 0 {
			return nil
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrNoProgress
		}
	}
	return nil
}
