// Copyright (c) 2020 Leonid Titov. All rights reserved.
// MIT license.
// Version 2020-12-23

package tncon

import (
	"io"
)

// bufferedChannelPipe is a synchronous buffered pipe implemented with a channel. This pipe
// is much more efficient than the standard io.Pipe, and can keep up with real-time
// shell output, which is needed for the lib/client/tncon implementation.
//
// Derived from https://github.com/latitov/milkthisbuffer/blob/main/milkthisbuffer.go
type bufferedChannelPipe struct {
	ch     chan byte
	closed chan struct{}
}

func newBufferedChannelPipe(len int) *bufferedChannelPipe {
	return &bufferedChannelPipe{
		ch:     make(chan byte, len),
		closed: make(chan struct{}),
	}
}

// Write will write all of p to the buffer unless the buffer is closed
func (b *bufferedChannelPipe) Write(p []byte) (n int, err error) {
	for n = 0; n < len(p); n++ {
		select {
		// blocking behavior
		case b.ch <- p[n]:
		case <-b.closed:
			return n, io.EOF
		}
	}
	return n, nil
}

// Read will always read at least one byte from the buffer unless the buffer is closed
func (b *bufferedChannelPipe) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	// blocking behavior
	select {
	case p[0] = <-b.ch:
	case <-b.closed:
		return 0, io.EOF
	}

	for n = 1; n < len(p); n++ {
		select {
		case p[n] = <-b.ch:
		case <-b.closed:
			return n, io.EOF
		default:
			return n, nil
		}
	}
	return n, nil
}

func (b *bufferedChannelPipe) Close() error {
	close(b.closed)
	return nil
}
