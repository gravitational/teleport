package httplib

import (
	"net"
	"net/http"
	"sync"
)

// AllConnStates indicates a subscription to all http.ConnState events
// in ConnStateEvents and ServerConnState. Pass to the Notify
// function of each to subscribe to all events.
const AllConnStates = -1

// TerminalConnState returns true when state is terminal.
func TerminalConnState(state http.ConnState) bool {
	return state == http.StateHijacked || state == http.StateClosed
}

// ServerConnState tracks connection state for HTTP server connections.
// Its Notify method implements the http.Server.ConnState function
// which should be attached to the Server instance to initiate tracking.
// Calling Channel and passing a connection will return a chan that
// receives state change events.
//
// All net.Conn instances passed to this type must be comparable or calls
// will panic. The net.Conn instances passed by http.Server such as *tls.Conn
// are comparable.
//
//    server := http.Server{...}
//    connState := NewServerConnState()
//    server.ConnState = connState.Notify
//    ...
//    tlsConn := tls.Server(...)
//    listener := listener.Wrap(tlsConn) // fake listener to pass to Serve
//    ch := connState.Channel(tlsConn, http.StateClosed)
//    server.Serve(listener)
//    <-ch // releases once tlsConn is closed
type ServerConnState struct {
	mu sync.Mutex

	// conns tracks open connection state. A ConnStateEvents is created
	// for each connection, which processes notifications for subscribers
	// of that connection.
	//
	// NOTE: using net.Conn as a map key is not safe if someone creates
	// a net.Conn using a concrete type that is not comparable. net.Conn
	// is stateful, so implementations should be comparable. Using net.Conn
	// simplified the implementation and allows for this functionality to
	// be extended, at the expense of strange implementations that will
	// panic on the first test.
	conns map[net.Conn]*ConnStateEvents
}

// NewServerConnState returns an initialized ServerConnState.
func NewServerConnState() *ServerConnState {
	return &ServerConnState{conns: make(map[net.Conn]*ConnStateEvents)}
}

// Notify subscribers of a connection state change.
// Terminal states remove the connection from tracking.
//
// Notify implements http.Server.ConnState.
func (s *ServerConnState) Notify(conn net.Conn, state http.ConnState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.conns[conn]
	if TerminalConnState(state) {
		delete(s.conns, conn)
	} else if !ok {
		s.conns[conn] = nil
	}

	if c != nil {
		c.Notify(state)
	}
}

// Channel returns a read-only channel that will receive http.ConnState
// events for conn. The state parameter determines which states are
// received by the channel. The channel will receive all events when state
// is AllConnStates. All channels will receive terminal states (http.StateClosed
// & http.StateHijacked) events after which the channel is closed. Multiple
// calls for the same connection are supported.
//
// Passing a closed connection will result in no events received and leak
// both the conn and channel unless followed by a Release call.
//
// Calling Channel when conn is already in the desired state will result
// in no events until the connection moves to a different state and back
// again (e.g. idle->active->idle).
func (s *ServerConnState) Channel(conn net.Conn, state http.ConnState) <-chan http.ConnState {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.conns[conn]
	if c == nil {
		c = NewConnStateEvents()
		s.conns[conn] = c
	}
	return c.Channel(state)
}

// Release resources for conn when something goes wrong after calling
// Channel and before http.Serve registers conn to trigger events. This
// will remove net.Conn state. This method is only required if http.Serve
// returns an error because state is removed when connections enter a
// terminal state.
//
// The conn will be tracked again, without previously registered
// subscriptions, if it is passed to Notify after calling Release.
func (s *ServerConnState) Release(conn net.Conn) {
	s.mu.Lock()
	c := s.conns[conn]
	delete(s.conns, conn)
	s.mu.Unlock()

	if c != nil {
		c.Stop()
	}
}

// ConnStateEvents tracks connection state for a single connection.
// This type is created by ServerConnState for each distinct connection.
type ConnStateEvents struct {
	events chan interface{}
}

// NewConnStateEvents returns an initialized ConnStateEvents.
func NewConnStateEvents() *ConnStateEvents {
	c := &ConnStateEvents{events: make(chan interface{}, connStateEventsQueueSize)}
	go c.processEvents()
	return c
}

// Notify state change.
func (c *ConnStateEvents) Notify(state http.ConnState) {
	c.events <- state
}

// Channel returns a read-only channel that will receive http.ConnState events.
// The state parameter determines which states are received by the channel.
// The channel will receive all events when state is AllConnStates. All channels
// will receive terminal states (http.StateClosed & http.StateHijacked) events
// after which the channel is closed. Multiple calls are supported.
//
// Passing a closed connection will result in no events received and leak
// the channel unless followed by a Stop call.
//
// Calling Channel when already in the desired state will result
// in no events until state moves to a different state and back
// again (e.g. idle->active->idle).
func (c *ConnStateEvents) Channel(state http.ConnState) <-chan http.ConnState {
	// chan buffer=1: assume channel is being serviced, but don't sync with reader
	ch := make(chan http.ConnState, 1)
	c.events <- connStateChannel{state: state, ch: ch}
	return ch
}

// Stop receiving events and close all subscriber channels.
func (c *ConnStateEvents) Stop() {
	c.events <- connStateStop{}
}

// processEvents is the core goroutine that runs for each instance.
// It serializes all events: channel subscriptions, state change events,
// and stop requests. It exits when it receives a connStateStop message,
// which can be accomplished by calling Stop.
func (c *ConnStateEvents) processEvents() {
	var channels []connStateChannel
	for event := range c.events {
		switch v := event.(type) {
		case connStateChannel:
			channels = append(channels, v)
		case http.ConnState:
			c.notify(channels, v)
			if TerminalConnState(v) {
				return
			}
		case connStateStop:
			for _, channel := range channels {
				close(channel.ch)
			}
			return
		}
	}
}

// notify channels of new state & close channels when state is terminal.
// notify is called from processEvents.
func (c *ConnStateEvents) notify(channels []connStateChannel, state http.ConnState) {
	if TerminalConnState(state) {
		for _, channel := range channels {
			channel.ch <- state
			close(channel.ch)
		}
		return
	}

	for _, channel := range channels {
		if state == channel.state || channel.state == AllConnStates {
			channel.ch <- state
		}
	}
}

// connStateChannel is a message passed to ConnStateEvents.processEvents
// from ConnStateEvents.Channel to subscribe to new events.
type connStateChannel struct {
	state http.ConnState
	ch    chan http.ConnState
}

// connStateStop is a message passed to ConnStateEvents.processEvents
// from ConnStateEvents.Stop to stop emitting events.
type connStateStop struct{}

// connStateEventsQueueSize sets the channel buffer size for ConnStateEvents.
// Connections do not change state very quickly and it is assumed (and documented)
// that subscriber channels will be serviced quickly, so this value should not
// have to be very big.
const connStateEventsQueueSize = 8
