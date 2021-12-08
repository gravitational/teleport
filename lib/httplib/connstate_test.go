/*
Copyright 2020 Gravitational, Inc.

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

package httplib

import (
	"net"
	"net/http"
	"testing"
	"time"
)

func TestConnStateEvents(t *testing.T) {
	t.Parallel()
	t.Run("Notify non-terminal states", func(t *testing.T) {
		c := NewConnStateEvents()
		ch := c.Channel(AllConnStates)
		states := []http.ConnState{http.StateNew, http.StateActive, http.StateIdle, http.StateActive}
		for _, state := range states {
			c.Notify(state)
		}
		for i, expect := range states {
			got, more := <-ch
			if expect != got {
				t.Errorf("[%d] expected %s got %s", i, expect, got)
			} else if !more {
				t.Error("expected channel to be open")
			}
		}
	})
	t.Run("Notify terminal state", func(t *testing.T) {
		states := []http.ConnState{http.StateHijacked, http.StateClosed}
		for i, state := range states {
			c := NewConnStateEvents()
			ch := c.Channel(state)
			c.Notify(http.StateIdle) // should be ignored
			c.Notify(state)
			got, more := <-ch
			if got != state {
				t.Errorf("[%d] expected %s got %s", i, state, got)
			} else if !more {
				t.Error("expected channel to be open")
			}
			got, more = <-ch
			if got != 0 {
				t.Errorf("[%d] expected zero value got %s", i, got)
			} else if more {
				t.Error("expected channel to be closed")
			}
		}
	})
	t.Run("Stop", func(t *testing.T) {
		c := NewConnStateEvents()
		ch := c.Channel(http.StateActive)
		c.Stop()
		got, more := <-ch
		if got != 0 {
			t.Errorf("expected zero value got %s", got)
		} else if more {
			t.Error("expected channel to be closed")
		}
	})
	t.Run("Notify new conn", func(t *testing.T) {
		c := NewServerConnState()
		conn := new(noopConn)
		c.Notify(conn, http.StateNew)
		expect := http.StateActive
		ch := c.Channel(conn, expect)
		c.Notify(conn, http.StateActive)
		got, more := <-ch
		if got != expect {
			t.Errorf("expected %s got %s", expect, got)
		} else if !more {
			t.Error("expected channel to be open")
		}

	})
}

func TestServerConnState(t *testing.T) {
	t.Parallel()
	t.Run("Multiple connections", func(t *testing.T) {
		c := NewServerConnState()
		connA := new(noopConn)
		connB := new(noopConn)
		chA := c.Channel(connA, AllConnStates)
		chB := c.Channel(connB, http.StateClosed)
		chA2 := c.Channel(connA, http.StateIdle)
		c.Notify(connA, http.StateActive)
		c.Notify(connB, http.StateClosed)
		c.Notify(connA, http.StateIdle)
		c.Notify(connA, http.StateHijacked)

		// chA
		expected := []http.ConnState{http.StateActive, http.StateIdle, http.StateHijacked}
		for i, expect := range expected {
			got, more := <-chA
			if got != expect {
				t.Errorf("[%d] chA expected %s got %s", i, expect, got)
			} else if !more {
				t.Errorf("[%d] chA expected more", i)
			}
		}

		// chA2
		expected = []http.ConnState{http.StateIdle, http.StateHijacked}
		for i, expect := range expected {
			got, more := <-chA2
			if got != expect {
				t.Errorf("[%d] chA expected %s got %s", i, expect, got)
			} else if !more {
				t.Errorf("[%d] chA expected more", i)
			}
		}

		// chB
		expect := http.StateClosed
		got, more := <-chB
		if got != expect {
			t.Errorf("chB expected %s got %s", expect, got)
		} else if !more {
			t.Error("chB expected more")
		}

		// all channels should be closed
		channels := []<-chan http.ConnState{chA, chB, chA2}
		names := []string{"chA", "chB", "chA2"}
		for i, channel := range channels {
			got, more = <-channel
			if got != 0 {
				t.Errorf("%s expected zero value got %s", names[i], got)
			} else if more {
				t.Errorf("%s expected to be closed", names[i])
			}
		}
	})
	t.Run("Release", func(t *testing.T) {
		c := NewServerConnState()
		conn := new(noopConn)
		ch := c.Channel(conn, AllConnStates)
		c.Release(conn)
		got, more := <-ch
		if got != 0 {
			t.Errorf("expected zero value got %s", got)
		} else if more {
			t.Error("expected channel to be closed")
		}
	})
}

type noopConn struct {
	// need a single field prevents compiler optimization
	// when creating multiple noopConns (compiler uses a single
	// instance when creating new instances of a zero field struct)
	_ int
}

func (noopConn) Read(b []byte) (n int, err error)   { return 0, nil }
func (noopConn) Write(b []byte) (n int, err error)  { return 0, nil }
func (noopConn) Close() error                       { return nil }
func (noopConn) LocalAddr() net.Addr                { return nil }
func (noopConn) RemoteAddr() net.Addr               { return nil }
func (noopConn) SetDeadline(t time.Time) error      { return nil }
func (noopConn) SetReadDeadline(t time.Time) error  { return nil }
func (noopConn) SetWriteDeadline(t time.Time) error { return nil }
