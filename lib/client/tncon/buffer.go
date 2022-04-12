//go:build windows && cgo

// Copyright (c) 2020 Leonid Titov. All rights reserved.
// MIT licence.
// Version 2020-12-23

package tncon

import (
	"io"
)

// BufferedChannelPipe is a synchronous buffered pipe implemented with a channel. This pipe
// is much more efficient than the standard io.Pipe, and can keep up with real-time
// shell output, which is needed for the lib/client/tncon implementation.
//
// Derived from https://github.com/latitov/milkthisbuffer/blob/main/milkthisbuffer.go
type bufferedChannelPipe struct {
	ch chan byte
}

func newBufferedChannelPipe(len int) (b *bufferedChannelPipe) {
	b = &bufferedChannelPipe{
		ch: make(chan byte, len),
	}
	return
}

// blocking behaviour
func (b *bufferedChannelPipe) Write(p []byte) (n int, err error) {

	// here's why:
	// https://stackoverflow.com/a/34899098/11729048
	defer func() {
		if err2 := recover(); err2 != nil {
			err = io.ErrClosedPipe
		}
	}()

	for n = 0; n < len(p); n++ {
		b.ch <- p[n]
	}
	return
}

// blocking behaviour until at least one byte read, then behaves non-blockingly
func (b *bufferedChannelPipe) Read(p []byte) (n int, err error) {
L:
	for n = 0; n < len(p); {
		if n == 0 {
			b1, ok := <-b.ch
			if !ok {
				err = io.EOF
				break L
			}
			p[n] = b1
			n++
		} else {
			select {
			case b1, ok := <-b.ch:
				if !ok {
					err = io.EOF
					break L
				}
				p[n] = b1
				n++
			default:
				break L
			}
		}
	}
	return
}

func (b *bufferedChannelPipe) Close() error {
	close(b.ch)
	return nil
}
