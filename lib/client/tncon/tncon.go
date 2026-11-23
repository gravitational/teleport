//go:build windows

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

package tncon

import (
	"fmt"
	"io"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
)

// A buffer of 100 should provide ample buffer to hold several VT
// sequences (which are 5 bytes each max) and output them to the
// terminal in real time.
const sequenceBufferSize = 100

var (
	sequenceBuffer *bufferedChannelPipe

	resizeEventSubscribers      []chan struct{}
	resizeEventSubscribersMutex sync.Mutex

	running           bool = false
	runningMutex      sync.Mutex
	runningQuitHandle windows.Handle
)

func SequenceReader() io.Reader {
	return sequenceBuffer
}

func writeSequence(bytes []byte) {
	sequenceBuffer.Write(bytes)
}

// SubcribeResizeEvents creates a new channel from which to receive console input events.
func SubcribeResizeEvents() chan struct{} {
	resizeEventSubscribersMutex.Lock()
	defer resizeEventSubscribersMutex.Unlock()

	ch := make(chan struct{})
	resizeEventSubscribers = append(resizeEventSubscribers, ch)

	return ch
}

func notifyResizeEvent() {
	resizeEventSubscribersMutex.Lock()
	defer resizeEventSubscribersMutex.Unlock()

	for _, sub := range resizeEventSubscribers {
		sub <- struct{}{}
	}
}

// readInputContinuous is a blocking call that continuously reads console
// input events. Events will be emitted via channels to subscribers. This
// function returns when stdin is closed, or the quit event is triggered.
func readInputContinuous(quitHandle windows.Handle) error {
	readConsoleEvents(quitHandle)

	// Close the sequenceBuffer (terminal stdin)
	if err := sequenceBuffer.Close(); err != nil {
		return trace.Wrap(err)
	}

	// Once finished, close all existing subscriber channels to notify them
	// of the close (they can resubscribe if it's ever restarted).
	resizeEventSubscribersMutex.Lock()
	defer resizeEventSubscribersMutex.Unlock()

	for _, ch := range resizeEventSubscribers {
		close(ch)
	}
	resizeEventSubscribers = resizeEventSubscribers[:0]

	runningMutex.Lock()
	defer runningMutex.Unlock()
	running = false

	// Close the quit event handle.
	if runningQuitHandle != 0 {
		windows.CloseHandle(runningQuitHandle)
		runningQuitHandle = 0
	}

	return nil
}

// IsRunning determines if a tncon session is currently active.
func IsRunning() bool {
	runningMutex.Lock()
	defer runningMutex.Unlock()

	return running
}

// Start begins a new tncon session, capturing raw input events and emitting
// them as events. Only one session may be active at a time, but sessions can
// be stopped
func Start() error {
	runningMutex.Lock()
	defer runningMutex.Unlock()

	if running {
		return fmt.Errorf("a tncon session is already active")
	}

	quitHandle, err := windows.CreateEvent(nil, 1 /* manual reset */, 0 /* not signaled */, nil)
	if err != nil {
		return trace.Wrap(err, "creating tncon quit event")
	}

	running = true
	runningQuitHandle = quitHandle

	// Adding a buffer increases the speed of reads by a great amount,
	// since waiting on channel sends is the main chokepoint. Without
	// a sufficient buffer, the individual keystrokes won't be transmitted
	// quickly enough for them to be grouped as a VT sequence by Windows.
	sequenceBuffer = newBufferedChannelPipe(sequenceBufferSize)

	go readInputContinuous(runningQuitHandle)

	return nil
}

// Stop sets the stop event, requesting that the input reader quits. Subscriber
// channels will close shortly after calling, and the subscriber list will be
// cleared.
func Stop() {
	runningMutex.Lock()
	defer runningMutex.Unlock()

	if running && runningQuitHandle != 0 {
		windows.SetEvent(runningQuitHandle)
	}
}

// readConsoleEvents reads console input events until stdin closes or the quit
// event is set. Key events are written to the sequence buffer and window
// resize events are sent to subscribers.
func readConsoleEvents(quitHandle windows.Handle) {
	input, err := windows.GetStdHandle(windows.STD_INPUT_HANDLE)
	if err != nil {
		return
	}

	var (
		decoder keyDecoder
		records [16]inputRecord
	)
	for {
		event, err := windows.WaitForMultipleObjects([]windows.Handle{input, quitHandle}, false, windows.INFINITE)
		if err != nil || event != windows.WAIT_OBJECT_0 {
			// The quit event was set or the wait failed.
			return
		}

		var count uint32
		if err := readConsoleInput(input, &records[0], uint32(len(records)), &count); err != nil {
			return
		}

		for _, record := range records[:count] {
			switch record.EventType {
			case windows.WINDOW_BUFFER_SIZE_EVENT:
				notifyResizeEvent()
			case windows.KEY_EVENT:
				key := record.KeyEvent
				if bytes := decoder.decode(key.KeyDown != 0, key.VirtualKeyCode, key.UnicodeChar); bytes != nil {
					writeSequence(bytes)
				}
			}
		}
	}
}
