/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package limiter

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/limiter/internal/ratelimit"
)

// ConnectionsLimiter is a network connection limiter.
type ConnectionsLimiter struct {
	maxConnections int64
	log            *slog.Logger

	next http.Handler

	sync.Mutex
	connections map[string]int64
}

// NewConnectionsLimiter returns new connection limiter, in case if connection
// limits are not set, they won't be tracked
func NewConnectionsLimiter(maxConnections int64) *ConnectionsLimiter {
	return &ConnectionsLimiter{
		maxConnections: maxConnections,
		log:            slog.With(teleport.ComponentKey, "limiter"),
		connections:    make(map[string]int64),
	}
}

// Wrap wraps an HTTP handler.
func (l *ConnectionsLimiter) Wrap(h http.Handler) {
	l.next = h
}

func (l *ConnectionsLimiter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if l.next == nil {
		sc := http.StatusInternalServerError
		http.Error(w, http.StatusText(sc), sc)
		return
	}

	clientIP, err := ratelimit.ExtractClientIP(r)
	if err != nil {
		l.log.WarnContext(context.Background(), "failed to extract source IP", "remote_addr", r.RemoteAddr)
		ratelimit.ServeHTTPError(w, r, err)
		return
	}

	if err := l.AcquireConnection(clientIP); err != nil {
		l.log.InfoContext(context.Background(), "limiting request", "token", clientIP, "error", err)
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(trace.UserMessage(err)))
		return
	}

	defer l.ReleaseConnection(clientIP)

	l.next.ServeHTTP(w, r)
}

// AcquireConnection acquires connection and bumps counter
func (l *ConnectionsLimiter) AcquireConnection(token string) error {
	if l.maxConnections == 0 {
		return nil
	}

	l.Lock()
	defer l.Unlock()

	numberOfConnections, exists := l.connections[token]
	if !exists {
		l.connections[token] = 1
		return nil
	}

	if numberOfConnections >= l.maxConnections {
		return trace.LimitExceeded("too many connections from %v: %v, max is %v", token, numberOfConnections, l.maxConnections)
	}

	l.connections[token] = numberOfConnections + 1
	return nil
}

// ReleaseConnection decrements the counter
func (l *ConnectionsLimiter) ReleaseConnection(token string) {
	if l.maxConnections == 0 {
		return
	}

	l.Lock()
	defer l.Unlock()

	numberOfConnections, exists := l.connections[token]
	if !exists {
		return
	}

	if numberOfConnections <= 1 {
		delete(l.connections, token)
	} else {
		l.connections[token] = numberOfConnections - 1
	}
}

// GetNumConnection returns the current number of connections for a token
func (l *ConnectionsLimiter) GetNumConnection(token string) (int64, error) {
	if l.maxConnections == 0 {
		return 0, nil
	}

	l.Lock()
	defer l.Unlock()

	numberOfConnections, exists := l.connections[token]
	if !exists {
		return -1, trace.BadParameter("unable to get connections of a nonexistent token: %q", token)
	}

	return numberOfConnections, nil
}
