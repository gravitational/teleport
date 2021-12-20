//go:build windows && cgo
// +build windows,cgo

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

package tncon

/*
#include <windows.h>
#include <stdlib.h>
#include <synchapi.h>
#include "tncon.h"
*/
import "C"

import (
	"fmt"
	"sync"
	"unsafe"
)

var (
	subscribers      []chan interface{}
	subscribersMutex sync.Mutex

	running           bool = false
	runningMutex      sync.Mutex
	runningQuitHandle C.HANDLE
)

// SequenceEvent is emitted when one or more key sequences are generated. This
// implementation generally produces many 1-byte events rather than one event
// per keystroke unless VT sequence translation is enabled.
type SequenceEvent struct {
	Sequence []byte
}

// ResizeEvent is emitted when the window size has been modified. The semantics
// of this event may vary depending on the current terminal and its flags:
//  - `cmd.exe` tends not to emit vertical resize events, and horizontal events
//    have nonsensical height (`Y`) values.
//  - `powershell.exe` emits events reliably, but height values are still
//    insane.
//  - The new Windows Terminal app emits sane events for both horizontal and
//    vertical resize inputs.
type ResizeEvent struct {
	X int16

	// y is the resized height. Note that depending on console mode, this
	// number may not be sensible and events may not trigger on vertical
	// resize.
	Y int16
}

// writeEvent dispatches an event to all listeners.
func writeEvent(event interface{}) {
	subscribersMutex.Lock()
	defer subscribersMutex.Unlock()

	for _, sub := range subscribers {
		sub <- event
	}
}

//export writeSequenceEvent
func writeSequenceEvent(addr *C.char, len C.int) {
	bytes := C.GoBytes(unsafe.Pointer(addr), len)
	writeEvent(SequenceEvent{
		Sequence: bytes,
	})
}

//export writeResizeEvent
func writeResizeEvent(size C.COORD) {
	writeEvent(ResizeEvent{
		X: int16(size.X),
		Y: int16(size.Y),
	})
}

// Subscribe creates a new channel from which to receive console input events.
func Subscribe() chan interface{} {
	subscribersMutex.Lock()
	defer subscribersMutex.Unlock()

	ch := make(chan interface{})
	subscribers = append(subscribers, ch)

	return ch
}

// readInputContinuous is a blocking call that continuously reads console
// input events. Events will be emitted via channels to subscribers. This
// function returns when stdin is closed, or the quit event is triggered.
func readInputContinuous(quitHandle C.HANDLE) error {
	C.ReadInputContinuous(quitHandle)

	// Once finished, close all existing subscriber channels to notify them
	// of the close (they can resubscribe if it's ever restarted).
	subscribersMutex.Lock()
	defer subscribersMutex.Unlock()

	for _, ch := range subscribers {
		close(ch)
	}
	subscribers = subscribers[:0]

	runningMutex.Lock()
	defer runningMutex.Unlock()
	running = false

	// Close the quit event handle.
	if runningQuitHandle != nil {
		C.CloseHandle(runningQuitHandle)
		runningQuitHandle = nil
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

	running = true
	runningQuitHandle = C.CreateEventA(nil, C.TRUE, C.FALSE, nil)

	go readInputContinuous(runningQuitHandle)

	return nil
}

// Stop sets the stop event, requesting that the input reader quits. Subscriber
// channels will close shortly after calling, and the subscriber list will be
// cleared.
func Stop() {
	runningMutex.Lock()
	defer runningMutex.Unlock()

	if running && runningQuitHandle != nil {
		C.SetEvent(runningQuitHandle)
	}
}
