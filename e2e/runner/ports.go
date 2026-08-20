package main

import (
	"net"
)

// allocatePorts listens on random ports and assigns them to the provided targets, then closes the listeners to free the ports for use.
// NOTE: this could be racy if another process binds to the port between when we close the listener and we use the port, but should be
// quite unlikely in practice (this runs on developer machines and in isolation in CI, and the window of opportunity is very small).
// It probably shouldn't be copied and used elsewhere.
func allocatePorts(targets ...*int) error {
	listeners := make([]net.Listener, 0, len(targets))

	for range targets {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			for _, prev := range listeners {
				prev.Close()
			}

			return err
		}

		listeners = append(listeners, l)
	}

	for i, l := range listeners {
		*targets[i] = l.Addr().(*net.TCPAddr).Port
		l.Close()
	}

	return nil
}
