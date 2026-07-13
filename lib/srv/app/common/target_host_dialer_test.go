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
	"syscall"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/tlsca"
)

// TestTargetDialer exercises the dialer end-to-end against real loopback
// sockets, enforcing a policy on the resolved IP. Multi-candidate classification
// is covered by TestTargetDialerControl.
func TestTargetDialer(t *testing.T) {
	t.Parallel()

	t.Run("unrestricted permits", func(t *testing.T) {
		t.Parallel()
		addr, closeListener := startTargetHostPolicyListener(t)
		defer closeListener()

		conn, err := NewTargetDialer(TargetHostPolicy{}, TargetHostAuditContext{}).DialContext(t.Context(), "tcp", addr)
		require.NoError(t, err)
		require.NoError(t, conn.Close())
	})

	t.Run("allow list permits matching IP", func(t *testing.T) {
		t.Parallel()
		addr, closeListener := startTargetHostPolicyListener(t)
		defer closeListener()

		emitter := &targetHostPolicyEmitter{}
		policy := TargetHostPolicy{AllowedPrefixes: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}}
		conn, err := NewTargetDialer(policy, newTargetHostAuditContext(t, emitter)).DialContext(t.Context(), "tcp", addr)
		require.NoError(t, err)
		require.NoError(t, conn.Close())
		requireNoTargetHostDeniedEvent(t, emitter)
	})

	t.Run("allow list blocks non-matching IP and emits audit", func(t *testing.T) {
		t.Parallel()
		emitter := &targetHostPolicyEmitter{}
		policy := TargetHostPolicy{AllowedPrefixes: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}

		_, err := NewTargetDialer(policy, newTargetHostAuditContext(t, emitter)).DialContext(t.Context(), "tcp", "127.0.0.1:443")
		require.True(t, traceIsAccessDenied(err), "got %v", err)

		event := requireTargetHostDeniedEvent(t, emitter)
		require.Equal(t, events.AppSessionTargetDialDeniedEvent, event.GetType())
		require.Equal(t, "allowed_hosts", event.Policy)
		require.Equal(t, "127.0.0.1", event.BlockedIP)
		require.Equal(t, []string{"127.0.0.1"}, event.ResolvedIPs)
	})

	t.Run("deny list blocks matching IP and emits audit", func(t *testing.T) {
		t.Parallel()
		emitter := &targetHostPolicyEmitter{}
		policy := TargetHostPolicy{DeniedPrefixes: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}}

		_, err := NewTargetDialer(policy, newTargetHostAuditContext(t, emitter)).DialContext(t.Context(), "tcp", "127.0.0.1:443")
		require.True(t, traceIsAccessDenied(err), "got %v", err)

		event := requireTargetHostDeniedEvent(t, emitter)
		require.Equal(t, "denied_hosts", event.Policy)
		require.Equal(t, "127.0.0.1", event.BlockedIP)
		require.Equal(t, "127.0.0.0/8", event.BlockedPrefix)
	})

	t.Run("deny list blocks matching IPv6 literal", func(t *testing.T) {
		t.Parallel()
		emitter := &targetHostPolicyEmitter{}
		policy := TargetHostPolicy{DeniedPrefixes: []netip.Prefix{netip.MustParsePrefix("::1/128")}}

		_, err := NewTargetDialer(policy, newTargetHostAuditContext(t, emitter)).DialContext(t.Context(), "tcp", "[::1]:443")
		require.True(t, traceIsAccessDenied(err), "got %v", err)

		event := requireTargetHostDeniedEvent(t, emitter)
		require.Equal(t, "denied_hosts", event.Policy)
		require.Equal(t, "::1", event.BlockedIP)
		require.Equal(t, "::1/128", event.BlockedPrefix)
	})

	t.Run("allow and deny are mutually exclusive", func(t *testing.T) {
		t.Parallel()
		policy := TargetHostPolicy{
			AllowedPrefixes: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
			DeniedPrefixes:  []netip.Prefix{netip.MustParsePrefix("169.254.0.0/16")},
		}

		_, err := NewTargetDialer(policy, TargetHostAuditContext{}).DialContext(t.Context(), "tcp", "127.0.0.1:443")
		require.True(t, trace.IsBadParameter(err), "got %v", err)
	})
}

// TestTargetDialerControl drives the dialer's ControlContext hook directly with
// the concrete candidate addresses the standard dialer would visit, covering the
// classification and denial-reporting logic without DNS or real sockets.
func TestTargetDialerControl(t *testing.T) {
	t.Parallel()

	// runControl feeds the given IPs through the policy control hook in order,
	// mimicking the standard dialer visiting each resolved candidate, and
	// returns the per-candidate errors.
	runControl := func(a *dialAttempt, ips ...string) []error {
		errs := make([]error, 0, len(ips))
		for _, ip := range ips {
			errs = append(errs, a.control(context.Background(), "tcp", net.JoinHostPort(ip, "443"), nil))
		}
		return errs
	}

	t.Run("deny list blocks matching, allows others", func(t *testing.T) {
		t.Parallel()
		policy := TargetHostPolicy{DeniedPrefixes: []netip.Prefix{netip.MustParsePrefix("169.254.0.0/16")}}
		a := &dialAttempt{policy: policy, host: "target.example.com", port: "443"}

		errs := runControl(a, "169.254.169.254", "127.0.0.1")
		require.Error(t, errs[0])
		require.NoError(t, errs[1])

		// An allowed candidate was observed, so this is not a policy denial.
		_, ok := a.denial()
		require.False(t, ok)
	})

	t.Run("deny list all blocked is a denial with every candidate", func(t *testing.T) {
		t.Parallel()
		policy := TargetHostPolicy{DeniedPrefixes: []netip.Prefix{netip.MustParsePrefix("169.254.0.0/16")}}
		a := &dialAttempt{policy: policy, host: "target.example.com", port: "443"}

		errs := runControl(a, "169.254.169.254", "169.254.1.1")
		require.Error(t, errs[0])
		require.Error(t, errs[1])

		denial, ok := a.denial()
		require.True(t, ok)
		require.Equal(t, targetHostPolicyDeny, denial.Policy)
		require.Equal(t, "169.254.169.254", denial.BlockedIP.String())
		require.Equal(t, "169.254.0.0/16", denial.BlockedPrefix.String())
		require.Equal(t, []string{"169.254.169.254", "169.254.1.1"}, stringifyAddrs(denial.ResolvedIPs))
	})

	t.Run("allow list all blocked is a denial without a matched prefix", func(t *testing.T) {
		t.Parallel()
		policy := TargetHostPolicy{AllowedPrefixes: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}}
		a := &dialAttempt{policy: policy, host: "target.example.com", port: "443"}

		errs := runControl(a, "127.0.0.1")
		require.Error(t, errs[0])

		denial, ok := a.denial()
		require.True(t, ok)
		require.Equal(t, targetHostPolicyAllow, denial.Policy)
		require.Equal(t, "127.0.0.1", denial.BlockedIP.String())
		require.False(t, denial.BlockedPrefix.IsValid())
	})

	t.Run("no candidates is not a denial", func(t *testing.T) {
		t.Parallel()
		policy := TargetHostPolicy{DeniedPrefixes: []netip.Prefix{netip.MustParsePrefix("169.254.0.0/16")}}
		a := &dialAttempt{policy: policy, host: "target.example.com", port: "443"}

		_, ok := a.denial()
		require.False(t, ok)
	})

	t.Run("zone identifier is stripped before evaluation", func(t *testing.T) {
		t.Parallel()
		policy := TargetHostPolicy{DeniedPrefixes: []netip.Prefix{netip.MustParsePrefix("fe80::/10")}}
		a := &dialAttempt{policy: policy, host: "target.example.com", port: "443"}

		errs := runControl(a, "fe80::1%eth0")
		require.Error(t, errs[0])

		denial, ok := a.denial()
		require.True(t, ok)
		require.Equal(t, "fe80::1", denial.BlockedIP.String())
	})

	t.Run("IPv4-mapped address matches IPv4 prefix", func(t *testing.T) {
		t.Parallel()
		policy := TargetHostPolicy{AllowedPrefixes: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}}
		a := &dialAttempt{policy: policy, host: "target.example.com", port: "443"}

		errs := runControl(a, "::ffff:127.0.0.1")
		require.NoError(t, errs[0])

		_, ok := a.denial()
		require.False(t, ok)
	})

	t.Run("concurrent control invocations are safe", func(t *testing.T) {
		t.Parallel()
		policy := TargetHostPolicy{DeniedPrefixes: []netip.Prefix{netip.MustParsePrefix("169.254.0.0/16")}}
		a := &dialAttempt{policy: policy, host: "target.example.com", port: "443"}

		var wg sync.WaitGroup
		for _, ip := range []string{"169.254.1.1", "169.254.1.2", "169.254.1.3", "169.254.1.4"} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = a.control(context.Background(), "tcp", net.JoinHostPort(ip, "443"), nil)
			}()
		}
		wg.Wait()

		denial, ok := a.denial()
		require.True(t, ok)
		require.Len(t, denial.ResolvedIPs, 4)
	})
}

// TestControlContextCandidateFallback verifies the dialer behavior the policy
// depends on: when a net.Dialer.ControlContext hook rejects a candidate, the
// dialer advances to the next resolved address instead of failing the dial.
// This is what lets a deny list skip a blocked IP and still reach an allowed
// one, and what lets a denied dial observe every resolved IP for the audit event.
func TestControlContextCandidateFallback(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	hostAddrs, err := net.DefaultResolver.LookupHost(ctx, "localhost")
	require.NoError(t, err)
	if len(hostAddrs) < 2 {
		t.Skipf("localhost resolves to %v; a dual-stack loopback is required to exercise candidate fallback", hostAddrs)
	}

	// Listen on every resolved address at a shared port so that whichever
	// candidate the dialer tries after the rejected one can connect, regardless
	// of the platform's address ordering.
	var port string
	for i, host := range hostAddrs {
		addr := net.JoinHostPort(host, "0")
		if i > 0 {
			addr = net.JoinHostPort(host, port)
		}
		l, err := net.Listen("tcp", addr)
		if err != nil {
			t.Skipf("could not listen on %s: %v", addr, err)
		}
		defer l.Close()
		if i == 0 {
			_, port, err = net.SplitHostPort(l.Addr().String())
			require.NoError(t, err)
		}
		go func() {
			for {
				conn, err := l.Accept()
				if err != nil {
					return
				}
				_ = conn.Close()
			}
		}()
	}

	var mu sync.Mutex
	var rejectedFirst bool
	dialer := net.Dialer{
		ControlContext: func(_ context.Context, _, _ string, _ syscall.RawConn) error {
			mu.Lock()
			defer mu.Unlock()
			if !rejectedFirst {
				rejectedFirst = true
				return trace.AccessDenied("reject first candidate")
			}
			return nil
		},
	}

	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort("localhost", port))
	require.NoError(t, err, "dialer should have advanced to a later candidate after the first was rejected")
	require.NoError(t, conn.Close())

	mu.Lock()
	require.True(t, rejectedFirst, "the control hook should have rejected the first candidate")
	mu.Unlock()
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
