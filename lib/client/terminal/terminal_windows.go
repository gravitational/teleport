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
	"os"
	"sync"
	"syscall"

	"github.com/Azure/go-ansiterm/winterm"
	"github.com/gravitational/trace"
	"github.com/moby/term"

	"github.com/gravitational/teleport/lib/client/tncon"
	"github.com/gravitational/teleport/lib/utils"
)

// initTerminal configures the terminal for raw, VT compatible output and
// optionally input. The returned function should be called before program
// exit to ensure the terminal is reset, otherwise it will be left in a broken
// state.
func initTerminal(input bool) (func(), error) {
	stdoutFd := int(syscall.Stdout)
	stdinFd := int(syscall.Stdin)

	oldOutMode, err := winterm.GetConsoleMode(uintptr(stdoutFd))
	if err != nil {
		return func() {}, fmt.Errorf("failed to retrieve stdout mode: %w", err)
	}

	oldInMode, err := winterm.GetConsoleMode(uintptr(stdinFd))
	if err != nil {
		return func() {}, fmt.Errorf("failed to retrieve stdout mode: %w", err)
	}

	newOutMode := oldOutMode | winterm.ENABLE_VIRTUAL_TERMINAL_PROCESSING | winterm.DISABLE_NEWLINE_AUTO_RETURN

	err = winterm.SetConsoleMode(uintptr(stdoutFd), newOutMode)
	if err != nil {
		return func() {}, fmt.Errorf("failed to set stdout mode: %w", err)
	}

	if input {
		newInMode := oldInMode
		newInMode &^= winterm.ENABLE_ECHO_INPUT
		newInMode &^= winterm.ENABLE_LINE_INPUT
		newInMode &^= winterm.ENABLE_MOUSE_INPUT
		newInMode &^= winterm.ENABLE_WINDOW_INPUT
		newInMode &^= winterm.ENABLE_PROCESSED_INPUT

		newInMode |= winterm.ENABLE_EXTENDED_FLAGS
		newInMode |= winterm.ENABLE_INSERT_MODE
		newInMode |= winterm.ENABLE_QUICK_EDIT_MODE
		newInMode |= winterm.ENABLE_VIRTUAL_TERMINAL_INPUT

		err = winterm.SetConsoleMode(uintptr(stdinFd), newInMode)
		if err != nil {
			// Attempt to reset the stdout mode before returning.
			err = winterm.SetConsoleMode(uintptr(stdoutFd), oldOutMode)
			if err != nil {
				log.ErrorContext(context.Background(), "Failed to reset terminal output mode",
					"original_output_mode", oldOutMode,
					"error", err,
				)
			}

			return func() {}, fmt.Errorf("failed to set stdin mode: %w", err)
		}
	}

	return func() {
		err := winterm.SetConsoleMode(uintptr(stdoutFd), oldOutMode)
		if err != nil {
			log.ErrorContext(context.Background(), "Failed to reset terminal output mode",
				"original_output_mode", oldOutMode,
				"error", err,
			)
		}

		if input {
			err = winterm.SetConsoleMode(uintptr(stdinFd), oldInMode)
			if err != nil {
				log.ErrorContext(context.Background(), "Failed to reset terminal input mode",
					"original_input_mode", oldInMode,
					"error", err,
				)
			}
		}
	}, nil
}

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

// InitRaw puts the terminal into raw output mode. If `input` is set, it also
// begins capturing raw input events from the Windows API, asynchronously
// writing them to a Pipe emulating a traditional Unix stdin.
// Note that some implementations may replace one or more streams (particularly
// stdin).
func (t *Terminal) InitRaw(input bool) error {
	// Put the terminal into raw mode.
	cleanup, err := initTerminal(input)
	if err != nil {
		return trace.Wrap(err)
	}

	// Begin reading raw input events.
	err = tncon.Start()
	if err != nil {
		return trace.Wrap(err)
	}

	// Make sure to reset the terminal on exit.
	t.closeWait.Add(1)
	go func() {
		defer t.closeWait.Done()

		<-t.closer.C
		tncon.Stop()
		cleanup()
	}()

	// emit resize events
	t.closeWait.Add(1)
	go func() {
		defer t.closeWait.Done()

		ch := tncon.SubcribeResizeEvents()
		for {
			select {
			case <-ch:
				t.writeEvent(ResizeEvent{})
			case <-t.closer.C:
				return
			}
		}
	}()

	t.stdin = tncon.SequenceReader()
	return nil
}

// IsAttached determines if this terminal is attached to an interactive console
// session.
func (t *Terminal) IsAttached() bool {
	return t.Stdin() == os.Stdin && term.IsTerminal(os.Stdin.Fd())
}

// Size fetches the current terminal size as measured in columns and rows.
func (t *Terminal) Size() (width int16, height int16, err error) {
	size, err := term.GetWinsize(uintptr(int(syscall.Stdout)))
	if err != nil {
		return 0, 0, trace.Errorf("Unable to get window size: %v", err)
	}

	return int16(size.Width), int16(size.Height), nil
}

// Resize makes a best-effort attempt to resize the terminal window. Support
// varies between platforms and terminal emulators.
func (t *Terminal) Resize(width, height int16) error {
	if height < 1 || width < 1 {
		return trace.Errorf("cannot shrink terminal below 1x1: rows=%d, cols=%d", height, width)
	}

	stdoutFd := uintptr(int(syscall.Stdout))

	// Hack: the buffer can't be smaller than the window, and the window can't
	// be bigger than the buffer otherwise we'll just get an inscrutible
	// "The parameter is incorrect" error. As a workaround, first resize the
	// window to the minimum possible size:
	err := winterm.SetConsoleWindowInfo(stdoutFd, true, winterm.SMALL_RECT{
		Left:   0,
		Top:    0,
		Right:  1,
		Bottom: 1,
	})
	if err != nil {
		return trace.Errorf("shrinking the console window: %w", err)
	}

	// ... then we can freely set the buffer:
	err = winterm.SetConsoleScreenBufferSize(stdoutFd, winterm.COORD{
		X: width,
		Y: height,
	})
	if err != nil {
		return trace.Errorf("setting screen buffer size: %w", err)
	}

	// ... and finally we can set the window's size to its desired value.
	err = winterm.SetConsoleWindowInfo(stdoutFd, true, winterm.SMALL_RECT{
		Left:   0,
		Top:    0,
		Right:  width - 1,
		Bottom: height - 1,
	})
	if err != nil {
		return trace.Errorf("setting console window info: %w", err)
	}

	return nil
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
// Potentially blocks on cleanup tasks.
func (t *Terminal) Close() error {
	t.clearSubscribers()
	if err := t.closer.Close(); err != nil {
		return trace.Wrap(err)
	}

	t.closeWait.Wait()
	return nil
}
