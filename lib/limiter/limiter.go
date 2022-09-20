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

// Package limiter implements connection and rate limiters for teleport
package limiter

import (
	"context"
	"encoding/json"
	"net"
	"net/http"

	"github.com/gravitational/oxy/ratelimit"
	"github.com/gravitational/trace"
	"github.com/mailgun/timetools"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

// Limiter helps limiting connections and request rates
type Limiter struct {
	// ConnectionsLimiter limits simultaneous connection
	*ConnectionsLimiter
	// rateLimiter limits request rate
	rateLimiter *RateLimiter
}

// Config sets up rate limits and configuration limits parameters
type Config struct {
	// Rates set ups rate limits
	Rates []Rate
	// MaxConnections configures maximum number of connections
	MaxConnections int64
	// MaxNumberOfUsers controls maximum number of simultaneously active users
	MaxNumberOfUsers int
	// Clock is an optional parameter, if not set, will use system time
	Clock timetools.TimeProvider
}

// SetEnv reads LimiterConfig from JSON string
func (l *Config) SetEnv(v string) error {
	if err := json.Unmarshal([]byte(v), l); err != nil {
		return trace.Wrap(err, "expected JSON encoded remote certificate")
	}
	return nil
}

// NewLimiter returns new rate and connection limiter
func NewLimiter(config Config) (*Limiter, error) {
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
	return l.rateLimiter.RegisterRequest(token, nil)
}

func (l *Limiter) RegisterRequestWithCustomRate(token string, customRate *ratelimit.RateSet) error {
	return l.rateLimiter.RegisterRequest(token, customRate)
}

// Add limiter to the handle
func (l *Limiter) WrapHandle(h http.Handler) {
	l.rateLimiter.Wrap(h)
	l.ConnLimiter.Wrap(l.rateLimiter)
}

// UnaryServerInterceptor returns a gRPC unary interceptor which
// rate limits by client IP.
func (l *Limiter) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return l.UnaryServerInterceptorWithCustomRate(func(string) *ratelimit.RateSet {
		return nil
	})
}

// CustomRateFunc is a function type which returns a custom *ratelimit.RateSet
// for a given endpoint string.
type CustomRateFunc func(endpoint string) *ratelimit.RateSet

// UnaryServerInterceptorWithCustomRate returns a gRPC unary interceptor which
// rate limits by client IP. Accepts a CustomRateFunc to set custom rates for
// specific gRPC methods.
func (l *Limiter) UnaryServerInterceptorWithCustomRate(customRate CustomRateFunc) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		peerInfo, ok := peer.FromContext(ctx)
		if !ok {
			return nil, trace.AccessDenied("missing peer info")
		}
		// Limit requests per second and simultaneous connection by client IP.
		clientIP, _, err := net.SplitHostPort(peerInfo.Addr.String())
		if err != nil {
			return nil, trace.BadParameter("missing client IP")
		}
		if err := l.RegisterRequestWithCustomRate(clientIP, customRate(info.FullMethod)); err != nil {
			return nil, trace.LimitExceeded("rate limit exceeded")
		}
		if err := l.ConnLimiter.Acquire(clientIP, 1); err != nil {
			return nil, trace.LimitExceeded("connection limit exceeded")
		}
		defer l.ConnLimiter.Release(clientIP, 1)
		return handler(ctx, req)
	}
}

// StreamServerInterceptor is a GPRC stream interceptor that rate limits
// incoming requests by client IP.
func (l *Limiter) StreamServerInterceptor(srv interface{}, serverStream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	peerInfo, ok := peer.FromContext(serverStream.Context())
	if !ok {
		return trace.AccessDenied("missing peer info")
	}
	// Limit requests per second and simultaneous connection by client IP.
	clientIP, _, err := net.SplitHostPort(peerInfo.Addr.String())
	if err != nil {
		return trace.BadParameter("missing client IP")
	}
	if err := l.RegisterRequest(clientIP); err != nil {
		return trace.LimitExceeded("rate limit exceeded")
	}
	if err := l.ConnLimiter.Acquire(clientIP, 1); err != nil {
		return trace.LimitExceeded("connection limit exceeded")
	}
	defer l.ConnLimiter.Release(clientIP, 1)
	return handler(srv, serverStream)
}
