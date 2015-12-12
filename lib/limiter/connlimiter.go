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

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/vulcand/oxy/connlimit"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/vulcand/oxy/utils"
)

type ConnectionsLimiter struct {
	*connlimit.ConnLimiter
	*sync.Mutex
	connections    map[string]int64
	maxConnections int64
}

func NewConnectionsLimiter(config LimiterConfig) (*ConnectionsLimiter, error) {
	limiter := ConnectionsLimiter{
		Mutex:          &sync.Mutex{},
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

// Add connection limiter to the handle
func (l *ConnectionsLimiter) WrapHandle(h http.Handler) {
	l.ConnLimiter.Wrap(h)
}

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
	} else {
		if numberOfConnections >= l.maxConnections {
			return trace.Errorf("Too many connections from %v", token)
		}
		l.connections[token] = numberOfConnections + 1
		return nil
	}
}

func (l *ConnectionsLimiter) ReleaseConnection(token string) {
	if l.maxConnections == 0 {
		return
	}

	l.Lock()
	defer l.Unlock()

	numberOfConnections, exists := l.connections[token]
	if !exists {
		log.Errorf("Trying to set negative number of connections")
	} else {
		if numberOfConnections <= 1 {
			delete(l.connections, token)
		} else {
			l.connections[token] = numberOfConnections - 1
		}
	}
}
