/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

// BenchmarkRegisterRequest compares the cost of RegisterRequest with
// no configured rates (noop path) against a limiter that has a high
// default rate configured (the old behavior before the short-circuit).
func BenchmarkRegisterRequest(b *testing.B) {
	for _, bc := range []struct {
		name   string
		config Config
	}{
		{
			name:   "NoRates",
			config: Config{},
		},
		{
			name: "HighDefaultRate",
			config: Config{
				Rates: []Rate{{
					Period:  time.Second,
					Average: 100_000_000,
					Burst:   100_000_000,
				}},
			},
		},
	} {
		b.Run(bc.name, func(b *testing.B) {
			limiter, err := NewLimiter(bc.config)
			require.NoError(b, err)

			b.ResetTimer()
			for b.Loop() {
				_ = limiter.RegisterRequest("127.0.0.1")
			}
		})
	}
}

// BenchmarkRegisterRequest_MultipleIPs measures the per-request cost
// across many distinct client IPs, which exercises the FnCache lookup
// path when rates are configured.
func BenchmarkRegisterRequest_MultipleIPs(b *testing.B) {
	const numIPs = 256
	ips := make([]string, numIPs)
	for i := range numIPs {
		ips[i] = fmt.Sprintf("10.0.0.%d", i)
	}

	for _, bc := range []struct {
		name   string
		config Config
	}{
		{
			name:   "NoRates",
			config: Config{},
		},
		{
			name: "HighDefaultRate",
			config: Config{
				Rates: []Rate{{
					Period:  time.Second,
					Average: 100_000_000,
					Burst:   100_000_000,
				}},
			},
		},
	} {
		b.Run(bc.name, func(b *testing.B) {
			limiter, err := NewLimiter(bc.config)
			require.NoError(b, err)

			b.ResetTimer()
			var i int
			for b.Loop() {
				_ = limiter.RegisterRequest(ips[i%numIPs])
				i++
			}
		})
	}
}

// BenchmarkHTTPMiddleware measures the full HTTP middleware path with
// no limits configured vs a high default rate.
func BenchmarkHTTPMiddleware(b *testing.B) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, bc := range []struct {
		name   string
		config Config
	}{
		{
			name:   "NoLimits",
			config: Config{},
		},
		{
			name: "HighDefaultRate",
			config: Config{
				Rates: []Rate{{
					Period:  time.Second,
					Average: 100_000_000,
					Burst:   100_000_000,
				}},
			},
		},
	} {
		b.Run(bc.name, func(b *testing.B) {
			limiter, err := NewLimiter(bc.config)
			require.NoError(b, err)

			middleware := MakeMiddleware(limiter)
			handler := middleware(okHandler)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "127.0.0.1:9999"

			b.ResetTimer()
			for b.Loop() {
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)
			}
		})
	}
}

// BenchmarkRegisterRequestAndConnection measures the combined
// rate + connection limiter path.
func BenchmarkRegisterRequestAndConnection(b *testing.B) {
	for _, bc := range []struct {
		name   string
		config Config
	}{
		{
			name:   "NoLimits",
			config: Config{},
		},
		{
			name: "HighDefaultRate",
			config: Config{
				Rates: []Rate{{
					Period:  time.Second,
					Average: 100_000_000,
					Burst:   100_000_000,
				}},
			},
		},
		{
			name:   "MaxConnectionsOnly",
			config: Config{MaxConnections: 100_000},
		},
	} {
		b.Run(bc.name, func(b *testing.B) {
			limiter, err := NewLimiter(bc.config)
			require.NoError(b, err)

			b.ResetTimer()
			for b.Loop() {
				release, err := limiter.RegisterRequestAndConnection("127.0.0.1")
				if err != nil {
					b.Fatal(err)
				}
				release()
			}
		})
	}
}

// BenchmarkUnaryInterceptor measures the gRPC unary interceptor path.
func BenchmarkUnaryInterceptor(b *testing.B) {
	for _, bc := range []struct {
		name   string
		config Config
	}{
		{
			name:   "NoLimits",
			config: Config{},
		},
		{
			name: "HighDefaultRate",
			config: Config{
				Rates: []Rate{{
					Period:  time.Second,
					Average: 100_000_000,
					Burst:   100_000_000,
				}},
			},
		},
	} {
		b.Run(bc.name, func(b *testing.B) {
			limiter, err := NewLimiter(bc.config)
			require.NoError(b, err)

			ctx := peer.NewContext(b.Context(), &peer.Peer{Addr: mockAddr{}})
			info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
			handler := func(context.Context, any) (any, error) { return "ok", nil }
			interceptor := limiter.UnaryServerInterceptor()

			b.ResetTimer()
			for b.Loop() {
				_, _ = interceptor(ctx, "request", info, handler)
			}
		})
	}
}

// BenchmarkHTTPServer measures end-to-end HTTP throughput through a
// real TCP listener with the limiter middleware. The upstream handler
// simulates a cheap operation (status-only response) so the limiter's
// contribution to total latency is visible. Each parallel goroutine
// reuses a single persistent connection to avoid exhausting ephemeral
// ports.
func BenchmarkHTTPServer(b *testing.B) {
	for _, bc := range []struct {
		name   string
		config Config
	}{
		{
			name:   "NoLimits",
			config: Config{},
		},
		{
			name: "HighDefaultRate",
			config: Config{
				Rates: []Rate{{
					Period:  time.Second,
					Average: 100_000_000,
					Burst:   100_000_000,
				}},
			},
		},
	} {
		b.Run(bc.name, func(b *testing.B) {
			limiter, err := NewLimiter(bc.config)
			require.NoError(b, err)

			upstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := MakeMiddleware(limiter)
			srv := httptest.NewServer(middleware(upstream))
			b.Cleanup(srv.Close)

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				// Each goroutine gets its own transport with a
				// single persistent connection to avoid port
				// exhaustion.
				client := &http.Client{
					Transport: &http.Transport{
						MaxIdleConnsPerHost: 1,
					},
				}
				for pb.Next() {
					resp, err := client.Get(srv.URL)
					if err != nil {
						b.Fatal(err)
					}
					resp.Body.Close()
				}
			})
		})
	}
}

// BenchmarkStreamInterceptor measures the gRPC stream interceptor path.
func BenchmarkStreamInterceptor(b *testing.B) {
	for _, bc := range []struct {
		name   string
		config Config
	}{
		{
			name:   "NoLimits",
			config: Config{},
		},
		{
			name: "HighDefaultRate",
			config: Config{
				Rates: []Rate{{
					Period:  time.Second,
					Average: 100_000_000,
					Burst:   100_000_000,
				}},
			},
		},
	} {
		b.Run(bc.name, func(b *testing.B) {
			limiter, err := NewLimiter(bc.config)
			require.NoError(b, err)

			ctx := peer.NewContext(b.Context(), &peer.Peer{Addr: mockAddr{}})
			ss := mockServerStream{ctx: ctx}
			info := &grpc.StreamServerInfo{}
			handler := func(any, grpc.ServerStream) error { return nil }
			interceptor := limiter.StreamServerInterceptor

			b.ResetTimer()
			for b.Loop() {
				_ = interceptor(nil, ss, info, handler)
			}
		})
	}
}
