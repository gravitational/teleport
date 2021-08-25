package terminal

import "sync"

// ResizeEvent is emitted when a terminal window is resized.
type ResizeEvent struct{}

// StopEvent is emitted when the user sends a SIGSTOP
type StopEvent struct{}

type signalEmitter struct {
	subscribers      []chan interface{}
	subscribersMutex sync.Mutex
}

// ResizeSubscribe creates a channel that will receive events whenever the
// terminal's size has changed.
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
