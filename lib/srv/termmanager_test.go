/*
Copyright 2022 Gravitational, Inc.

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

package srv

import (
	"context"
	"crypto/rand"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCTRLCPassthrough(t *testing.T) {
	m := NewTermManager()
	m.On()
	r, w := io.Pipe()
	m.AddReader("foo", r)
	go w.Write([]byte("\x03"))
	buf := make([]byte, 1)
	_, err := m.Read(buf)
	require.NoError(t, err)
	require.Equal(t, []byte("\x03"), buf)
}

func TestCTRLCCapture(t *testing.T) {
	m := NewTermManager()
	r, w := io.Pipe()
	m.AddReader("foo", r)
	w.Write([]byte("\x03"))

	select {
	case <-m.TerminateNotifier():
	case <-time.After(time.Second * 10):
		t.Fatal("terminateNotifier should've seen an event")
	}
}

func TestHistoryKept(t *testing.T) {
	m := NewTermManager()
	m.On()

	data := make([]byte, 10000)
	n, err := rand.Read(data)
	require.NoError(t, err)
	require.Equal(t, len(data), n)

	n, err = m.Write(data[:len(data)/2])
	require.NoError(t, err)
	require.Equal(t, len(data)/2, n)

	n, err = m.Write(data[len(data)/2:])
	require.NoError(t, err)
	require.Equal(t, len(data)/2, n)

	kept := data[len(data)-maxHistoryBytes:]
	require.Equal(t, m.GetRecentHistory(), kept)
}

func TestBufferedKept(t *testing.T) {
	m := NewTermManager()

	data := make([]byte, 20000)
	n, err := rand.Read(data)
	require.NoError(t, err)
	require.Equal(t, len(data), n)

	n, err = m.Write(data)
	require.NoError(t, err)
	require.Equal(t, len(data), n)

	kept := data[len(data)-maxPausedHistoryBytes:]
	require.Equal(t, m.buffer, kept)
}

func TestNoReadWhenOff(t *testing.T) {
	t.Parallel()
	m := NewTermManager()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	r, w := io.Pipe()
	data := make([]byte, 90)
	n, err := rand.Read(data)
	require.NoError(t, err)

	discarded := make(chan struct{})
	m.onDiscard = func() { close(discarded) }

	// add our data source to the multiplexer
	m.AddReader("reader", r)

	// write some data into our pipe, making it available to the multiplexer
	_, err = w.Write(data[:n])
	require.NoError(t, err)

	// wait for the multiplexer to read the data, it should be discarded which will close the discard channel
	// and continue execution
	select {
	case <-discarded:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	// enable the multiplexer, data should now flow through instead of being dropped
	m.On()

	// write 3 more bytes, these should be buffered until a reader picks them up
	_, err = w.Write([]byte{1, 2, 3})
	require.NoError(t, err)

	// attempt to read the buffered data, this should succeed since the multiplexer should now allow dataflow
	n, err = m.Read(data)
	require.NoError(t, err)
	require.Equal(t, []byte{1, 2, 3}, data[:n])
}
