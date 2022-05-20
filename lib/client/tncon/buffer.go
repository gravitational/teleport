// Copyright (c) 2020 Leonid Titov. All rights reserved.
// MIT licence.
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
	ch chan byte
}

func newBufferedChannelPipe(len int) *bufferedChannelPipe {
	return &bufferedChannelPipe{
		ch: make(chan byte, len),
	}
}

// Write will write all of p to the buffer unless the buffer is closed
func (b *bufferedChannelPipe) Write(p []byte) (n int, err error) {
	// Catch write to closed buffer with a recover
	// https://stackoverflow.com/a/34899098/11729048
	defer func() {
		if err2 := recover(); err2 != nil {
			err = io.ErrClosedPipe
		}
	}()

	for n = 0; n < len(p); n++ {
		// blocking behaviour
		b.ch <- p[n]
	}
	return n, nil
}

// Read will always read at least one byte from the buffer unless the buffer is closed
func (b *bufferedChannelPipe) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	// blocking behaviour
	r, ok := <-b.ch
	if !ok {
		return 0, io.EOF
	}
	p[n] = r

	for n = 1; n < len(p); n++ {
		select {
		case r, ok := <-b.ch:
			if !ok {
				return n, io.EOF
			}
			p[n] = r
		default:
			return n, nil
		}
	}
	return n, nil
}

func (b *bufferedChannelPipe) Close() error {
	close(b.ch)
	return nil
}
