//go:build !windows
// +build !windows

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
package terminal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gravitational/trace"
	"github.com/moby/term"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/utils"
)

// Terminal is used to configure raw input and output modes for an attached
// terminal emulator.
type Terminal struct {
	signalEmitter

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	closer    *utils.CloseBroadcaster
	closeWait *sync.WaitGroup
}

// New creates a new Terminal instance. Callers should call `InitRaw` to
// configure the terminal for raw input or output modes.
//
// Note that the returned Terminal instance must be closed to ensure the
// terminal is properly reset; unexpected exits may leave users' terminals
// unusable.
func New(stdin io.Reader, stdout, stderr io.Writer) (*Terminal, error) {
	if stdin == nil {
		stdin = os.Stdin
	}

	if stdout == nil {
		stdout = os.Stdout
	}

	if stderr == nil {
		stderr = os.Stderr
	}

	term := Terminal{
		stdin:     stdin,
		stdout:    stdout,
		stderr:    stderr,
		closer:    utils.NewCloseBroadcaster(),
		closeWait: &sync.WaitGroup{},
	}

	return &term, nil
}

// addCRFormatter is a formatter which adds carriage return (CR) to the output of a base formatter.
// This is needed in case the logger output is fed into terminal in raw mode.
type addCRFormatter struct {
	BaseFmt logrus.Formatter
}

func (r addCRFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	out, err := r.BaseFmt.Format(entry)
	if err != nil {
		return nil, err
	}

	replaced := bytes.ReplaceAll(out, []byte("\n"), []byte("\r\n"))
	return replaced, nil
}

func newCRFormatter(baseFmt logrus.Formatter) *addCRFormatter {
	return &addCRFormatter{BaseFmt: baseFmt}
}

// InitRaw puts the terminal into raw mode. On Unix, no special input handling
// is required beyond simply reading from stdin, so `input` has no effect.
// Note that some implementations may replace one or more streams (particularly
// stdin).
func (t *Terminal) InitRaw(input bool) error {
	// Put the terminal into raw mode.
	ts, err := term.SetRawTerminal(0)
	fmtNew := newCRFormatter(logrus.StandardLogger().Formatter)
	logrus.StandardLogger().Formatter = fmtNew
	if err != nil {
		log.Warnf("Could not put terminal into raw mode: %v", err)
	} else {
		// Ensure the terminal is reset on exit.
		t.closeWait.Add(1)
		go func() {
			<-t.closer.C
			term.RestoreTerminal(0, ts)
			logrus.StandardLogger().Formatter = fmtNew.BaseFmt
			t.closeWait.Done()
		}()
	}

	// Convert Unix-specific signals to our abstracted events.
	ctrlZSignal := make(chan os.Signal, 1)
	signal.Notify(ctrlZSignal, syscall.SIGTSTP)
	resizeSignal := make(chan os.Signal, 1)
	signal.Notify(resizeSignal, syscall.SIGWINCH)
	go func() {
		for {
			select {
			case <-ctrlZSignal:
				t.writeEvent(StopEvent{})
			case <-resizeSignal:
				t.writeEvent(ResizeEvent{})
			case <-t.closer.C:
				return
			}
		}
	}()

	// NOTE: Unix does not require any special input handling.

	return nil
}

// Size fetches the current terminal size as measured in columns and rows.
func (t *Terminal) Size() (width int16, height int16, err error) {
	size, err := term.GetWinsize(0)
	if err != nil {
		return 0, 0, trace.Errorf("Unable to get window size: %v", err)
	}

	return int16(size.Width), int16(size.Height), nil
}

// IsAttached determines if this terminal is attached to an interactive console
// session.
func (t *Terminal) IsAttached() bool {
	return t.Stdin() == os.Stdin && term.IsTerminal(os.Stdin.Fd())
}

// Resize makes a best-effort attempt to resize the terminal window. Support
// varies between platforms and terminal emulators.
func (t *Terminal) Resize(width, height int16) error {
	_, err := os.Stdout.Write([]byte(fmt.Sprintf("\x1b[8;%d;%dt", height, width)))

	return trace.Wrap(err)
}

func (t *Terminal) Stdin() io.Reader {
	return t.stdin
}

func (t *Terminal) Stdout() io.Writer {
	return t.stdout
}

func (t *Terminal) Stderr() io.Writer {
	return t.stderr
}

// Close closes the Terminal, restoring the console to its original state.
func (t *Terminal) Close() error {
	t.clearSubscribers()
	if err := t.closer.Close(); err != nil {
		return trace.Wrap(err)
	}

	t.closeWait.Wait()
	return nil
}
