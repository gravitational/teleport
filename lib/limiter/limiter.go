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
	"encoding/json"
	"net/http"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
)

type Limiter struct {
	// both limiters implement http.Handle
	*ConnectionsLimiter
	rateLimiter *RateLimiter
}

type LimiterConfig struct {
	Rates            []Rate
	MaxConnections   int64 `yaml:"max_connections" json:"max_connections"`
	MaxNumberOfUsers int   `yaml:"max_users" json:"max_users"`
}

// SetEnv reads LimiterConfig from JSON string
func (l *LimiterConfig) SetEnv(v string) error {
	if err := json.Unmarshal([]byte(v), l); err != nil {
		return trace.Wrap(err, "expected JSON encoded remote certificate")
	}
	return nil
}

func NewLimiter(config LimiterConfig) (*Limiter, error) {
	var err error
	limiter := Limiter{}

	limiter.ConnectionsLimiter, err = NewConnectionsLimiter(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	limiter.rateLimiter, err = NewRateLimiter(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &limiter, nil
}

func (l *Limiter) RegisterRequest(token string) error {
	return l.rateLimiter.RegisterRequest(token)
}

// Add limiter to the handle
func (l *Limiter) WrapHandle(h http.Handler) {
	l.rateLimiter.Wrap(h)
	l.ConnLimiter.Wrap(l.rateLimiter)
}
