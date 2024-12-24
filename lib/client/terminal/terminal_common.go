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
	"fmt"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentClient)

// ResizeEvent is emitted when a terminal window is resized.
type ResizeEvent struct{}

// StopEvent is emitted when the user sends a SIGSTOP
type StopEvent struct{}

type signalEmitter struct {
	subscribers      []chan interface{}
	subscribersMutex sync.Mutex
}

// Subscribe creates a channel that will receive terminal events.
func (e *signalEmitter) Subscribe() chan interface{} {
	e.subscribersMutex.Lock()
	defer e.subscribersMutex.Unlock()

	ch := make(chan interface{})
	e.subscribers = append(e.subscribers, ch)

	return ch
}

func (e *signalEmitter) writeEvent(event interface{}) {
	e.subscribersMutex.Lock()
	defer e.subscribersMutex.Unlock()

	for _, sub := range e.subscribers {
		sub <- event
	}
}

func (e *signalEmitter) clearSubscribers() {
	e.subscribersMutex.Lock()
	defer e.subscribersMutex.Unlock()

	for _, ch := range e.subscribers {
		close(ch)
	}
	e.subscribers = e.subscribers[:0]
}

// SetCursorPos sets the cursor position to the given x, y coordinates.
// Coordinates are 1-indexed. (1, 1) represents the top left corner.
func (t *Terminal) SetCursorPos(x, y int) error {
	_, err := fmt.Fprintf(t.stdout, "\x1b[%d;%dH", y, x)
	return trace.Wrap(err)
}

// SetWindowTitle sets the terminal window's title.
func (t *Terminal) SetWindowTitle(s string) error {
	_, err := fmt.Fprintf(t.stdout, "\x1b]0;%s\a", s)
	return trace.Wrap(err)
}

// Clear clears the terminal, including scrollback.
func (t *Terminal) Clear() error {
	// \x1b[3J - clears scrollback (it is needed at least for the Mac terminal) -
	// https://newbedev.com/how-do-i-reset-the-scrollback-in-the-terminal-via-a-shell-command
	// \x1b\x63 - clears current screen - same as '\0033\0143' from https://superuser.com/a/123007
	const resetPattern = "\x1b[3J\x1b\x63\n"
	if _, err := t.Stdout().Write([]byte(resetPattern)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
