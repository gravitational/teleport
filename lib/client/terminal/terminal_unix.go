//go:build !windows
// +build !windows

/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package terminal

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gravitational/trace"
	"github.com/moby/term"

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

// InitRaw puts the terminal into raw mode. On Unix, no special input handling
// is required beyond simply reading from stdin, so `input` has no effect.
// Note that some implementations may replace one or more streams (particularly
// stdin).
func (t *Terminal) InitRaw(input bool) error {
	// Put the terminal into raw mode.
	ts, err := term.SetRawTerminal(0)

	originalHandler := slog.Default().Handler()
	slog.SetDefault(slog.New(slog.DiscardHandler))
	if err != nil {
		log.WarnContext(context.Background(), "Could not put terminal into raw mode", "error", err)
	} else {
		// Ensure the terminal is reset on exit.
		t.closeWait.Add(1)
		go func() {
			<-t.closer.C
			term.RestoreTerminal(0, ts)
			slog.SetDefault(slog.New(originalHandler))
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
	_, err := fmt.Fprintf(t.stdout, "\x1b[8;%d;%dt", height, width)
	return trace.Wrap(err)
}

const (
	saveCursor    = "7"
	restoreCursor = "8"
)

// SaveCursor saves the current cursor position.
func (t *Terminal) SaveCursor() error {
	_, err := t.stdout.Write([]byte("\x1b" + saveCursor))
	return trace.Wrap(err)
}

// RestoreCursor restores the last saved cursor position.
func (t *Terminal) RestoreCursor() error {
	_, err := t.stdout.Write([]byte("\x1b" + restoreCursor))
	return trace.Wrap(err)
}

func (t *Terminal) Stdin() io.Reader  { return t.stdin }
func (t *Terminal) Stdout() io.Writer { return t.stdout }
func (t *Terminal) Stderr() io.Writer { return t.stderr }

// Close closes the Terminal, restoring the console to its original state.
func (t *Terminal) Close() error {
	t.clearSubscribers()
	if err := t.closer.Close(); err != nil {
		return trace.Wrap(err)
	}

	t.closeWait.Wait()
	return nil
}
