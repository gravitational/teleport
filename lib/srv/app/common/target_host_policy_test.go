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

package common

import (
	"context"
	"net"
	"net/netip"
	"sync"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestTargetHostPolicyDialContext(t *testing.T) {
	t.Parallel()

	t.Run("unrestricted permits", func(t *testing.T) {
		t.Parallel()
		addr, closeListener := startTargetHostPolicyListener(t)
		defer closeListener()

		conn, err := (TargetHostPolicy{}).DialContext(t.Context(), "tcp", addr, TargetHostAuditContext{})
		require.NoError(t, err)
		require.NoError(t, conn.Close())
	})

	t.Run("allow list permits matching IP", func(t *testing.T) {
		t.Parallel()
		addr, closeListener := startTargetHostPolicyListener(t)
		defer closeListener()

		policy := TargetHostPolicy{AllowedPrefixes: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}}
		conn, err := policy.DialContext(t.Context(), "tcp", addr, TargetHostAuditContext{})
		require.NoError(t, err)
		require.NoError(t, conn.Close())
	})

	t.Run("allow list blocks non-matching IP and emits audit", func(t *testing.T) {
		t.Parallel()
		emitter := &targetHostPolicyEmitter{}
		policy := TargetHostPolicy{AllowedPrefixes: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}

		_, err := policy.DialContext(t.Context(), "tcp", "127.0.0.1:443", newTargetHostAuditContext(t, emitter))
		require.True(t, traceIsAccessDenied(err), "got %v", err)

		event := requireTargetHostDeniedEvent(t, emitter)
		require.Equal(t, events.AppSessionTargetDialDeniedEvent, event.GetType())
		require.Equal(t, "allowed_hosts", event.Policy)
		require.Equal(t, "127.0.0.1", event.BlockedIP)
		require.Equal(t, []string{"127.0.0.1"}, event.ResolvedIPs)
	})

	t.Run("deny list blocks matching IPv6 literal", func(t *testing.T) {
		t.Parallel()
		emitter := &targetHostPolicyEmitter{}
		policy := TargetHostPolicy{DeniedPrefixes: []netip.Prefix{netip.MustParsePrefix("::1/128")}}

		_, err := policy.DialContext(t.Context(), "tcp", "[::1]:443", newTargetHostAuditContext(t, emitter))
		require.True(t, traceIsAccessDenied(err), "got %v", err)

		event := requireTargetHostDeniedEvent(t, emitter)
		require.Equal(t, "denied_hosts", event.Policy)
		require.Equal(t, "::1", event.BlockedIP)
		require.Equal(t, "::1/128", event.BlockedPrefix)
	})

	t.Run("mixed DNS results dial permitted address", func(t *testing.T) {
		t.Parallel()
		addr, closeListener := startTargetHostPolicyListener(t)
		defer closeListener()
		_, port, err := net.SplitHostPort(addr)
		require.NoError(t, err)

		emitter := &targetHostPolicyEmitter{}
		policy := TargetHostPolicy{
			DeniedPrefixes: []netip.Prefix{netip.MustParsePrefix("169.254.0.0/16")},
			Resolver: targetHostPolicyResolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
				return []netip.Addr{
					netip.MustParseAddr("169.254.169.254"),
					netip.MustParseAddr("127.0.0.1"),
				}, nil
			}),
		}

		conn, err := policy.DialContext(t.Context(), "tcp", net.JoinHostPort("target.example.com", port), newTargetHostAuditContext(t, emitter))
		require.NoError(t, err)
		require.NoError(t, conn.Close())

		requireNoTargetHostDeniedEvent(t, emitter)
	})

	t.Run("all DNS results blocked emits audit", func(t *testing.T) {
		t.Parallel()
		emitter := &targetHostPolicyEmitter{}
		policy := TargetHostPolicy{
			DeniedPrefixes: []netip.Prefix{netip.MustParsePrefix("169.254.0.0/16")},
			Resolver: targetHostPolicyResolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
				return []netip.Addr{
					netip.MustParseAddr("169.254.169.254"),
					netip.MustParseAddr("169.254.1.1"),
				}, nil
			}),
		}

		_, err := policy.DialContext(t.Context(), "tcp", "target.example.com:443", newTargetHostAuditContext(t, emitter))
		require.True(t, traceIsAccessDenied(err), "got %v", err)

		event := requireTargetHostDeniedEvent(t, emitter)
		require.Equal(t, "denied_hosts", event.Policy)
		require.Equal(t, "169.254.169.254", event.BlockedIP)
		require.Equal(t, []string{"169.254.169.254", "169.254.1.1"}, event.ResolvedIPs)
	})

	t.Run("empty DNS result returns not found without audit", func(t *testing.T) {
		t.Parallel()
		emitter := &targetHostPolicyEmitter{}
		policy := TargetHostPolicy{
			AllowedPrefixes: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
			Resolver: targetHostPolicyResolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
				return nil, nil
			}),
		}

		_, err := policy.DialContext(t.Context(), "tcp", "target.example.com:443", newTargetHostAuditContext(t, emitter))
		require.True(t, trace.IsNotFound(err), "got %v", err)
		requireNoTargetHostDeniedEvent(t, emitter)
	})

	t.Run("allow and deny are mutually exclusive", func(t *testing.T) {
		t.Parallel()
		policy := TargetHostPolicy{
			AllowedPrefixes: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
			DeniedPrefixes:  []netip.Prefix{netip.MustParsePrefix("169.254.0.0/16")},
		}

		_, err := policy.DialContext(t.Context(), "tcp", "127.0.0.1:443", TargetHostAuditContext{})
		require.True(t, trace.IsBadParameter(err), "got %v", err)
	})

	t.Run("deny list blocks zoned IPv6 literal", func(t *testing.T) {
		t.Parallel()
		emitter := &targetHostPolicyEmitter{}
		policy := TargetHostPolicy{DeniedPrefixes: []netip.Prefix{netip.MustParsePrefix("fe80::/10")}}

		_, err := policy.DialContext(t.Context(), "tcp", "[fe80::1%eth0]:443", newTargetHostAuditContext(t, emitter))
		require.True(t, traceIsAccessDenied(err), "got %v", err)

		event := requireTargetHostDeniedEvent(t, emitter)
		require.Equal(t, "denied_hosts", event.Policy)
		// The zone is stripped before evaluation and reporting.
		require.Equal(t, "fe80::1", event.BlockedIP)
	})

	t.Run("deny list blocks zoned IPv6 from resolver", func(t *testing.T) {
		t.Parallel()
		emitter := &targetHostPolicyEmitter{}
		policy := TargetHostPolicy{
			DeniedPrefixes: []netip.Prefix{netip.MustParsePrefix("fe80::/10")},
			Resolver: targetHostPolicyResolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
				return []netip.Addr{netip.MustParseAddr("fe80::1%eth0")}, nil
			}),
		}

		_, err := policy.DialContext(t.Context(), "tcp", "target.example.com:443", newTargetHostAuditContext(t, emitter))
		require.True(t, traceIsAccessDenied(err), "got %v", err)
		requireTargetHostDeniedEvent(t, emitter)
	})

	t.Run("allow list matches IPv4-mapped resolved address", func(t *testing.T) {
		t.Parallel()
		addr, closeListener := startTargetHostPolicyListener(t)
		defer closeListener()
		_, port, err := net.SplitHostPort(addr)
		require.NoError(t, err)

		policy := TargetHostPolicy{
			AllowedPrefixes: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")},
			Resolver: targetHostPolicyResolverFunc(func(context.Context, string, string) ([]netip.Addr, error) {
				return []netip.Addr{netip.MustParseAddr("::ffff:127.0.0.1")}, nil
			}),
		}

		conn, err := policy.DialContext(t.Context(), "tcp", net.JoinHostPort("target.example.com", port), TargetHostAuditContext{})
		require.NoError(t, err)
		require.NoError(t, conn.Close())
	})
}

func TestHTTPProxyConfiguredInEnv(t *testing.T) {
	// Neutralize any ambient proxy variables so the test is deterministic.
	// Not parallel: mutates the process environment.
	clearProxyEnv := func(t *testing.T) {
		for _, v := range httpProxyEnvVars {
			t.Setenv(v, "")
		}
	}

	t.Run("none set", func(t *testing.T) {
		clearProxyEnv(t)
		_, ok := HTTPProxyConfiguredInEnv()
		require.False(t, ok)
	})

	t.Run("HTTPS_PROXY set", func(t *testing.T) {
		clearProxyEnv(t)
		t.Setenv("HTTPS_PROXY", "http://proxy.example.com:3128")
		name, ok := HTTPProxyConfiguredInEnv()
		require.True(t, ok)
		require.Equal(t, "HTTPS_PROXY", name)
	})

	t.Run("lowercase http_proxy set", func(t *testing.T) {
		clearProxyEnv(t)
		t.Setenv("http_proxy", "http://proxy.example.com:3128")
		name, ok := HTTPProxyConfiguredInEnv()
		require.True(t, ok)
		require.Equal(t, "http_proxy", name)
	})
}

func startTargetHostPolicyListener(t *testing.T) (string, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	return listener.Addr().String(), func() {
		require.NoError(t, listener.Close())
		<-done
	}
}

type targetHostPolicyResolverFunc func(context.Context, string, string) ([]netip.Addr, error)

func (f targetHostPolicyResolverFunc) LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error) {
	return f(ctx, network, host)
}

type targetHostPolicyEmitter struct {
	mu     sync.Mutex
	events []apievents.AuditEvent
}

func (e *targetHostPolicyEmitter) EmitAuditEvent(_ context.Context, event apievents.AuditEvent) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, event)
	return nil
}

func requireTargetHostDeniedEvent(t *testing.T, emitter *targetHostPolicyEmitter) *apievents.AppSessionTargetDialDenied {
	t.Helper()

	emitter.mu.Lock()
	defer emitter.mu.Unlock()
	require.NotEmpty(t, emitter.events)
	event, ok := emitter.events[len(emitter.events)-1].(*apievents.AppSessionTargetDialDenied)
	require.True(t, ok)
	return event
}

func requireNoTargetHostDeniedEvent(t *testing.T, emitter *targetHostPolicyEmitter) {
	t.Helper()

	emitter.mu.Lock()
	defer emitter.mu.Unlock()
	require.Empty(t, emitter.events)
}

func newTargetHostAuditContext(t *testing.T, emitter apievents.Emitter) TargetHostAuditContext {
	t.Helper()

	app, err := types.NewAppV3(types.Metadata{Name: "test"}, types.AppSpecV3{
		URI:        "http://target.example.com",
		PublicAddr: "test.example.com",
	})
	require.NoError(t, err)

	identity := &tlsca.Identity{
		Username: "alice",
		RouteToApp: tlsca.RouteToApp{
			Name:        "test",
			PublicAddr:  "test.example.com",
			SessionID:   "session-id",
			ClusterName: "root.example.com",
		},
	}
	return TargetHostAuditContext{
		Emitter:  emitter,
		ServerID: "server-id",
		Identity: identity,
		App:      app,
	}
}

func traceIsAccessDenied(err error) bool {
	return err != nil && trace.IsAccessDenied(err)
}
