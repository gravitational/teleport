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
#include <Windows.h>
#include <stdlib.h>
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

	running      bool = false
	runningMutex sync.Mutex
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

// ReadInputContinuous is a blocking call that continuously reads console
// input events. Events will be emitted via channels to subscribers.
func ReadInputContinuous() error {
	runningMutex.Lock()
	defer runningMutex.Unlock()

	if running {
		return fmt.Errorf("only one call to ReadInputContinuous is allowed")
	}

	C.ReadInputContinuous()
	return nil
}
