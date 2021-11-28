/*
Copyright 2021 Gravitational, Inc.

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
	"sync"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

func safeSend(ch chan bool, value bool) (closed bool) {
	defer func() {
		if recover() != nil {
			closed = true
		}
	}()

	ch <- value
	return false
}

type BreakReader struct {
	remaining []byte
	cond      *sync.Cond
	in        chan []byte
	on        bool
	R         *utils.TrackingReader
}

func NewBreakReader(r *utils.TrackingReader) *BreakReader {
	data := make(chan []byte)

	go func() {
		for {
			buf := make([]byte, 1024)
			n, err := r.Read(buf)
			if err != nil {
				log.Error("BreakReader: failed to read from reader.")
				return
			}

			data <- buf[:n]
		}
	}()

	return &BreakReader{
		cond: sync.NewCond(&sync.Mutex{}),
		in:   data,
		on:   true,
		R:    r,
	}
}

func (r *BreakReader) On() {
	r.cond.L.Lock()
	defer r.cond.L.Unlock()
	r.on = true
	r.cond.Broadcast()
}

func (r *BreakReader) Off() {
	r.cond.L.Lock()
	defer r.cond.L.Unlock()
	r.on = false
	r.cond.Broadcast()
}

func (r *BreakReader) Read(p []byte) (int, error) {
	if len(r.remaining) > 0 {
		n := copy(p, r.remaining)
		r.remaining = r.remaining[n:]
		return n, nil
	}

	c := make(chan bool)
	go func() {
		r.cond.L.Lock()

		for {
			if safeSend(c, r.on) {
				break
			}

			r.cond.Wait()
		}

		r.cond.L.Unlock()
	}()

	on := <-c
	for {
		if !on {
			on = <-c
			continue
		}

		select {
		case on = <-c:
			continue
		case r.remaining = <-r.in:
			close(c)
			n := copy(p, r.remaining)
			r.remaining = r.remaining[n:]
			return n, nil
		}
	}
}

type SwitchWriter struct {
	mu     sync.Mutex
	W      *utils.TrackingWriter
	buffer []byte
	on     bool
}

func NewSwitchWriter(w *utils.TrackingWriter) *SwitchWriter {
	return &SwitchWriter{
		W:  w,
		on: true,
	}
}

func (w *SwitchWriter) On() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.on = true

	for len(w.buffer) > 0 {
		n, err := w.W.Write(w.buffer)
		if err != nil {
			log.Errorf("SwitchWriter: failed to write to underlying writer: %v", err)
			return trace.Wrap(err)
		}

		w.buffer = w.buffer[n:]
	}

	return nil
}

func (w *SwitchWriter) Off() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.on = false
}

func (w *SwitchWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.on {
		return w.W.Write(p)
	}

	w.buffer = append(w.buffer, p...)
	return len(p), nil
}

func (w *SwitchWriter) WriteUnconditional(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.W.Write(p)
}
