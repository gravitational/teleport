/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package tools_test

import (
	"net/http"
	"sync"
)

type limitRequest struct {
	limit int64
	lock  chan struct{}
}

// limitedResponseWriter wraps http.ResponseWriter and enforces a write limit
// then block the response until signal is received.
type limitedResponseWriter struct {
	requests chan limitRequest
}

// newLimitedResponseWriter creates a new limitedResponseWriter with the lock.
func newLimitedResponseWriter() *limitedResponseWriter {
	lw := &limitedResponseWriter{
		requests: make(chan limitRequest, 10),
	}
	return lw
}

// Wrap wraps response writer if limit was previously requested, if not, return original one.
func (lw *limitedResponseWriter) Wrap(w http.ResponseWriter) http.ResponseWriter {
	select {
	case request := <-lw.requests:
		return &wrapper{
			ResponseWriter: w,
			request:        request,
		}
	default:
		return w
	}
}

// SetLimitRequest sends limit request to the pool to wrap next response writer with defined limits.
func (lw *limitedResponseWriter) SetLimitRequest(limit limitRequest) {
	lw.requests <- limit
}

// wrapper wraps the http response writer to control writing operation by blocking it.
type wrapper struct {
	http.ResponseWriter

	written  int64
	request  limitRequest
	released bool

	mutex sync.Mutex
}

// Write writes data to the underlying ResponseWriter but respects the byte limit.
func (lw *wrapper) Write(p []byte) (int, error) {
	lw.mutex.Lock()
	defer lw.mutex.Unlock()

	if lw.written >= lw.request.limit && !lw.released {
		// Send signal that lock is acquired and wait till it was released by response.
		lw.request.lock <- struct{}{}
		<-lw.request.lock
		lw.released = true
	}

	n, err := lw.ResponseWriter.Write(p)
	lw.written += int64(n)
	return n, err
}
