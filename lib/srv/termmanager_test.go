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
	go w.Write([]byte("\x03"))

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
	rand.Read(data)

	n, err := m.Write(data)
	require.NoError(t, err)
	require.Equal(t, len(data), n)

	kept := data[len(data)-maxHistoryBytes:]
	require.Equal(t, m.GetRecentHistory(), kept)
}

func TestBufferedKeptt(t *testing.T) {
	m := NewTermManager()

	data := make([]byte, 20000)
	rand.Read(data)

	n, err := m.Write(data)
	require.NoError(t, err)
	require.Equal(t, len(data), n)

	kept := data[len(data)-maxPausedHistoryBytes:]
	require.Equal(t, m.buffer, kept)
}
