/*
Copyright 2015 Gravitational, Inc.

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

package limiter

import (
	"net/http"
	"sync"

	"github.com/gravitational/oxy/connlimit"
	"github.com/gravitational/oxy/utils"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// ConnectionsLimiter is a network connection limiter and tracker
type ConnectionsLimiter struct {
	*connlimit.ConnLimiter
	maxConnections int64

	sync.Mutex
	connections map[string]int64
}

// NewConnectionsLimiter returns new connection limiter, in case if connection
// limits are not set, they won't be tracked
func NewConnectionsLimiter(config Config) (*ConnectionsLimiter, error) {
	limiter := ConnectionsLimiter{
		maxConnections: config.MaxConnections,
		connections:    make(map[string]int64),
	}

	ipExtractor, err := utils.NewExtractor("client.ip")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	limiter.ConnLimiter, err = connlimit.New(nil, ipExtractor, config.MaxConnections)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &limiter, nil
}

// WrapHandle adds connection limiter to the handle
func (l *ConnectionsLimiter) WrapHandle(h http.Handler) {
	l.ConnLimiter.Wrap(h)
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
		log.Errorf("Trying to set negative number of connections")
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
