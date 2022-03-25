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

package prompt

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/term"

	log "github.com/sirupsen/logrus"
)

// ErrReaderClosed is returned from ContextReader.ReadContext after it is
// closed.
var ErrReaderClosed = errors.New("ContextReader has been closed")

// ErrNotTerminal is returned by password reads attempted in non-terminal
// readers.
var ErrNotTerminal = errors.New("underlying reader is not a terminal")

type readOutcome struct {
	value []byte
	err   error
}

type readerState int

const (
	readerStateIdle readerState = iota
	readerStateClean
	readerStatePassword
	readerStateClosed
)

// xTermI aggregates methods from /x/term for easy mocking.
type xTermI interface {
	GetState(fd int) (*term.State, error)
	IsTerminal(fd int) bool
	ReadPassword(fd int) ([]byte, error)
	Restore(fd int, oldState *term.State) error
}

type xTerm struct{}

func (xTerm) GetState(fd int) (*term.State, error) {
	return term.GetState(fd)
}

func (xTerm) IsTerminal(fd int) bool {
	return term.IsTerminal(fd)
}

func (xTerm) ReadPassword(fd int) ([]byte, error) {
	return term.ReadPassword(fd)
}

func (xTerm) Restore(fd int, oldState *term.State) error {
	return term.Restore(fd, oldState)
}

// ContextReader is a wrapper around an underlying io.Reader or terminal that
// allows reads to be abandoned. An abandoned read may be reclaimed by future
// callers.
// ContextReader instances are not safe for concurrent use, callers may block
// indefinitely and reads may be lost.
type ContextReader struct {
	xTerm xTermI

	// reader is used for clean reads.
	reader io.Reader
	// fd is used for password reads.
	// Only present if the underlying reader is a terminal, otherwise set to -1.
	fd int

	closed chan struct{}
	reads  chan readOutcome

	mu                *sync.Mutex
	cond              *sync.Cond
	previousTermState *term.State
	state             readerState
}

// NewContextReader creates a new ContextReader wrapping rd.
// Callers should avoid reading from rd after the ContextReader is used, as
// abandoned calls may be in progress. It is safe to read from rd if one can
// guarantee that no calls where abandoned.
// Calling ContextReader.Close attempts to release resources, but note that
// ongoing reads cannot be interrupted.
func NewContextReader(rd io.Reader) *ContextReader {
	xt := xTerm{}

	fd := -1
	if f, ok := rd.(*os.File); ok {
		val := int(f.Fd())
		if xt.IsTerminal(val) {
			fd = val
		}
	}

	mu := &sync.Mutex{}
	cond := sync.NewCond(mu)
	cr := &ContextReader{
		xTerm:  xt,
		reader: bufio.NewReader(rd),
		fd:     fd,
		closed: make(chan struct{}),
		reads:  make(chan readOutcome), // unbuffered
		mu:     mu,
		cond:   cond,
	}
	go cr.processReads()
	return cr
}

func (cr *ContextReader) processReads() {
	defer close(cr.reads)

	for {
		cr.mu.Lock()
		for cr.state == readerStateIdle {
			cr.cond.Wait()
		}
		// Stop the reading loop? Once closed, forever closed.
		if cr.state == readerStateClosed {
			cr.mu.Unlock()
			return
		}
		// React to the state that took us out of idleness.
		// We can't hold the lock during the entire read, so we obey the last state
		// observed.
		state := cr.state
		cr.mu.Unlock()

		var value []byte
		var err error
		switch state {
		case readerStateClean:
			const bufferSize = 4096
			value = make([]byte, bufferSize)
			var n int
			n, err = cr.reader.Read(value)
			value = value[:n]
		case readerStatePassword:
			value, err = cr.xTerm.ReadPassword(cr.fd)
		}
		cr.mu.Lock()
		cr.previousTermState = nil // A finalized read resets the terminal.
		switch cr.state {
		case readerStateClosed: // Don't transition from closed.
		default:
			cr.state = readerStateIdle
		}
		cr.mu.Unlock()

		select {
		case <-cr.closed:
			log.Warnf("ContextReader closed during ongoing read, dropping %v bytes", len(value))
			return
		case cr.reads <- readOutcome{value: value, err: err}:
		}
	}
}

// ReadContext returns the next chunk of output from the reader.
// If ctx is canceled before the read completes, the current read is abandoned
// and may be reclaimed by future callers.
// It is not safe to read from the underlying reader after a read is abandoned,
// nor is it safe to concurrently call ReadContext.
func (cr *ContextReader) ReadContext(ctx context.Context) ([]byte, error) {
	if err := cr.fireCleanRead(); err != nil {
		return nil, trace.Wrap(err)
	}

	return cr.waitForRead(ctx)
}

func (cr *ContextReader) fireCleanRead() error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	switch cr.state {
	case readerStateIdle: // OK, transition and broadcast.
		cr.state = readerStateClean
		cr.cond.Broadcast()
	case readerStateClean: // OK, ongoing read.
	case readerStatePassword: // OK, ongoing read.
		// Attempt to reset terminal state to non-password.
		if cr.previousTermState != nil {
			state := cr.previousTermState
			cr.previousTermState = nil
			if err := cr.xTerm.Restore(cr.fd, state); err != nil {
				return trace.Wrap(err)
			}
		}
	case readerStateClosed:
		return ErrReaderClosed
	}
	return nil
}

func (cr *ContextReader) waitForRead(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, trace.Wrap(ctx.Err())
	case <-cr.closed:
		return nil, ErrReaderClosed
	case read := <-cr.reads:
		return read.value, read.err
	}
}

// ReadPassword reads a password from the underlying reader, provided that the
// reader is a terminal.
// It follows the semantics of ReadContext.
func (cr *ContextReader) ReadPassword(ctx context.Context) ([]byte, error) {
	if cr.fd == -1 {
		return nil, ErrNotTerminal
	}
	if err := cr.firePasswordRead(); err != nil {
		return nil, trace.Wrap(err)
	}

	return cr.waitForRead(ctx)
}

func (cr *ContextReader) firePasswordRead() error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	switch cr.state {
	case readerStateIdle: // OK, transition and broadcast.
		// Save present terminal state, so it may be restored in case the read goes
		// from password to clean.
		state, err := cr.xTerm.GetState(cr.fd)
		if err != nil {
			return trace.Wrap(err)
		}
		cr.previousTermState = state
		cr.state = readerStatePassword
		cr.cond.Broadcast()
	case readerStateClean: // OK, ongoing clean read.
		// TODO(codingllama): Transition the terminal to password read?
		log.Warn("prompt: Clean read reused by password read")
	case readerStatePassword: // OK, ongoing password read.
	case readerStateClosed:
		return ErrReaderClosed
	}
	return nil
}

// Close closes the context reader, attempting to release resources and aborting
// ongoing and future ReadContext calls.
// Background reads that are already blocked cannot be interrupted, thus Close
// doesn't guarantee a release of all resources.
func (cr *ContextReader) Close() error {
	cr.mu.Lock()
	switch cr.state {
	case readerStateClosed: // OK, already closed.
	default:
		cr.state = readerStateClosed
		close(cr.closed) // interrupt blocked sends.
		cr.cond.Broadcast()
	}
	cr.mu.Unlock()
	return nil
}

// PasswordReader is a ContextReader that reads passwords from the underlying
// terminal.
type PasswordReader ContextReader

// Password returns a PasswordReader from a ContextReader.
// The returned PasswordReader is only functional if the underlying reader is a
// terminal.
func (cr *ContextReader) Password() *PasswordReader {
	return (*PasswordReader)(cr)
}

// ReadContext reads a password from the underlying reader, provided that the
// reader is a terminal. It is equivalent to ContextReader.ReadPassword.
// It follows the semantics of ReadContext.
func (pr *PasswordReader) ReadContext(ctx context.Context) ([]byte, error) {
	cr := (*ContextReader)(pr)
	return cr.ReadPassword(ctx)
}
