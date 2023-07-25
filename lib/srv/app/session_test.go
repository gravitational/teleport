// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/events"
)

func newSessionChunk(timeout time.Duration) *sessionChunk {
	return &sessionChunk{
		id:           uuid.New().String(),
		closeC:       make(chan struct{}),
		inflightCond: sync.NewCond(&sync.Mutex{}),
		closeTimeout: timeout,
		log:          logrus.NewEntry(logrus.StandardLogger()),
		streamCloser: events.NewDiscardRecorder(),
	}
}

func TestSessionChunkCloseNormal(t *testing.T) {
	const timeout = 1 * time.Second

	sess := newSessionChunk(timeout)

	err := sess.acquire()
	require.NoError(t, err)

	go sess.close(context.Background())
	sess.release()

	select {
	case <-time.After(timeout / 2):
		require.FailNow(t, "session chunk did not close, despite all requests ending")
	case <-sess.closeC:
	}
}

func TestSessionChunkCloseTimeout(t *testing.T) {
	const timeout = 1 * time.Second

	start := time.Now()
	sess := newSessionChunk(timeout)

	err := sess.acquire() // this is never released
	require.NoError(t, err)

	go sess.close(context.Background())

	select {
	case <-time.After(timeout * 2):
		require.FailNow(t, "session chunk did not close, despite timeout elapsing")
	case <-sess.closeC:
	}

	elapsed := time.Since(start)
	require.True(t, elapsed > timeout, "expected to wait at least %s before closing, waited only %s", timeout, elapsed)
}

func TestSessionChunkCannotAcquireAfterClosing(t *testing.T) {
	const timeout = 1 * time.Second
	sess := newSessionChunk(timeout)
	sess.close(context.Background())
	require.Error(t, sess.acquire())
}
