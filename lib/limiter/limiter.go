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
	if config.MaxConnections < 0 {
		config.MaxConnections = 0
	}

	connectionsLimiter, err := NewConnectionsLimiter(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rateLimiter, err := NewRateLimiter(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Limiter{
		ConnectionsLimiter: connectionsLimiter,
		rateLimiter:        rateLimiter,
	}, nil
}

func (l *Limiter) RegisterRequest(token string) error {
	return l.rateLimiter.RegisterRequest(token, nil)
}

func (l *Limiter) RegisterRequestWithCustomRate(token string, customRate *ratelimit.RateSet) error {
	return l.rateLimiter.RegisterRequest(token, customRate)
}

// WrapHandle adds limiter to the handle
func (l *Limiter) WrapHandle(h http.Handler) {
	l.rateLimiter.Wrap(h)
	l.ConnLimiter.Wrap(l.rateLimiter)
}

// RegisterRequestAndConnection register a rate and connection limiter for a given token. Close function is returned,
// and it must be called to release the token. When a limit is hit an error is returned.
// Example usage:
//
//	release, err := limiter.RegisterRequestAndConnection(clientIP)
//	if err != nil {
//		return trace.Wrap(err)
//	}
//	defer release()
func (l *Limiter) RegisterRequestAndConnection(token string) (func(), error) {
	// Apply rate limiting.
	if err := l.RegisterRequest(token); err != nil {
		return func() {}, trace.LimitExceeded("rate limit exceeded for %q", token)
	}

	// Apply connection limiting.
	if err := l.AcquireConnection(token); err != nil {
		return func() {}, trace.LimitExceeded("exceeded connection limit for %q", token)
	}

	return func() { l.ReleaseConnection(token) }, nil
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

// StreamServerInterceptor is a gRPC stream interceptor that rate limits
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

// WrapListener returns a [Listener] that wraps the provided listener
// with one that limits connections
func (l *Limiter) WrapListener(ln net.Listener) (*Listener, error) {
	return NewListener(ln, l.ConnectionsLimiter)
}

type handlerWrapper interface {
	http.Handler
	WrapHandle(http.Handler)
}

// MakeMiddleware creates an HTTP middleware that wraps provided handle.
func MakeMiddleware(limiter handlerWrapper) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		limiter.WrapHandle(next)
		return limiter
	}
}
