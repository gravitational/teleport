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
	"crypto/tls"
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/tlsca"
)

// targetHostDialKeepAlive mirrors the net.Dialer keep-alive setting used by
// net/http.DefaultTransport.
const targetHostDialKeepAlive = 30 * time.Second

// TargetHostAuditConfig contains audit metadata for denied target dials.
type TargetHostAuditConfig struct {
	Emitter  apievents.Emitter
	Logger   *slog.Logger
	ServerID string
	Identity *tlsca.Identity
	App      types.Application
}

// TargetDialer dials application targets, enforcing a TargetHostPolicy against
// the concrete destination IP and emitting an audit event when a dial is
// denied.
type TargetDialer struct {
	policy TargetHostPolicy
	audit  TargetHostAuditConfig
}

// NewTargetDialer returns a dialer that enforces the given policy and reports
// denied dials through the given audit context.
func NewTargetDialer(policy TargetHostPolicy, audit TargetHostAuditConfig) TargetDialer {
	return TargetDialer{policy: policy, audit: audit}
}

// DialContext dials an application target, rejecting the connection when the
// concrete destination IP violates the target host policy. The policy runs via
// net.Dialer.ControlContext on each candidate immediately before connect, so the
// address evaluated is exactly the one connected to (no re-resolution, no DNS
// rebinding window). The dialed address keeps the original hostname, so the
// caller's Host header, TLS SNI, and certificate verification are unaffected.
func (d TargetDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if !d.policy.Enabled() {
		return newNetDialer().DialContext(ctx, network, address)
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return d.dial(ctx, host, port, func(nd *net.Dialer) (net.Conn, error) {
		return nd.DialContext(ctx, network, address)
	})
}

// DialTLS dials an application target and performs a TLS handshake using the
// provided config, applying the same policy enforcement as DialContext.
func (d TargetDialer) DialTLS(ctx context.Context, network, address string, tlsConfig *tls.Config) (net.Conn, error) {
	if !d.policy.Enabled() {
		dialer := &tls.Dialer{NetDialer: newNetDialer(), Config: tlsConfig}
		return dialer.DialContext(ctx, network, address)
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return d.dial(ctx, host, port, func(nd *net.Dialer) (net.Conn, error) {
		dialer := &tls.Dialer{NetDialer: nd, Config: tlsConfig}
		return dialer.DialContext(ctx, network, address)
	})
}

// dial runs the provided dial function with a policy-aware control hook. When
// the dial fails because every candidate address was blocked and none were
// allowed, it emits a denied audit event and returns an access-denied error.
// Any other failure (including a failure to reach an allowed address) is
// returned as-is.
func (d TargetDialer) dial(ctx context.Context, host, port string, dialFn func(*net.Dialer) (net.Conn, error)) (net.Conn, error) {
	attempt := &dialAttempt{policy: d.policy, host: host, port: port}
	nd := newNetDialer()
	nd.ControlContext = attempt.control

	conn, err := dialFn(nd)
	if err != nil {
		if denial, ok := attempt.denial(); ok {
			d.emitDenied(ctx, denial)
			return nil, deniedTargetHostError(host)
		}
		return nil, trace.Wrap(err)
	}
	return conn, nil
}

func (d TargetDialer) emitDenied(ctx context.Context, denial targetHostDenial) {
	if d.audit.Emitter == nil || d.audit.App == nil || d.audit.Identity == nil {
		return
	}

	event := &apievents.AppSessionTargetDialDenied{
		Metadata: apievents.Metadata{
			Type:        events.AppSessionTargetDialDeniedEvent,
			Code:        events.AppSessionTargetDialDeniedCode,
			ClusterName: d.audit.Identity.RouteToApp.ClusterName,
		},
		UserMetadata:    d.audit.Identity.GetUserMetadata(),
		SessionMetadata: getSessionMetadata(d.audit.Identity),
		ServerMetadata: apievents.ServerMetadata{
			ServerVersion:   teleport.Version,
			ServerID:        d.audit.ServerID,
			ServerNamespace: apidefaults.Namespace,
		},
		AppMetadata:   *MakeAppMetadata(d.audit.App),
		TargetHost:    denial.Host,
		TargetPort:    denial.Port,
		Policy:        denial.Policy,
		ResolvedIPs:   stringifyAddrs(denial.ResolvedIPs),
		BlockedIP:     stringifyAddr(denial.BlockedIP),
		BlockedPrefix: stringifyPrefix(denial.BlockedPrefix),
	}

	// Detach from ctx cancellation: on the dial paths ctx is the client request
	// context, so a client disconnecting as the dial is denied would otherwise
	// cause the async emitter to drop this security-relevant record.
	emitCtx := context.WithoutCancel(ctx)
	if err := d.audit.Emitter.EmitAuditEvent(emitCtx, event); err != nil && d.audit.Logger != nil {
		d.audit.Logger.WarnContext(emitCtx, "Failed to emit app target dial denied audit event.", "error", err)
	}
}

// newNetDialer builds the base net.Dialer used for target dials. The control
// hook is attached per attempt by TargetDialer.dial.
func newNetDialer() *net.Dialer {
	return &net.Dialer{
		Timeout:   apidefaults.DefaultIOTimeout,
		KeepAlive: targetHostDialKeepAlive,
	}
}

type targetHostDenial struct {
	Host          string
	Port          string
	ResolvedIPs   []netip.Addr
	Policy        string
	BlockedIP     netip.Addr
	BlockedPrefix netip.Prefix
}

// dialAttempt records the outcome of the policy control hook across all
// candidate addresses tried for a single dial. The standard dialer can invoke
// the hook from concurrent goroutines (dual-stack Happy Eyeballs), so access is
// guarded by a mutex.
type dialAttempt struct {
	policy TargetHostPolicy
	host   string
	port   string

	mu          sync.Mutex
	resolved    []netip.Addr
	allowedSeen bool
	hasBlocked  bool
	blockedIP   netip.Addr
}

// control is a net.Dialer.ControlContext hook. It runs after the socket is
// created but before connect, with the concrete IP the dialer is about to dial.
// Returning an error aborts that candidate, so the dialer moves on to the next
// resolved address.
func (a *dialAttempt) control(_ context.Context, _ string, address string, _ syscall.RawConn) error {
	ipStr, _, err := net.SplitHostPort(address)
	if err != nil {
		return trace.Wrap(err)
	}
	addr, err := netip.ParseAddr(ipStr)
	if err != nil {
		return trace.Wrap(err)
	}
	addr = canonicalAddr(addr)

	a.mu.Lock()
	defer a.mu.Unlock()
	a.resolved = append(a.resolved, addr)

	if !a.policy.blocked(addr) {
		a.allowedSeen = true
		return nil
	}
	if !a.hasBlocked {
		a.hasBlocked = true
		a.blockedIP = addr
	}
	return deniedTargetHostError(a.host)
}

// denial reports whether the dial should be treated as a policy denial: at
// least one candidate must have been blocked and none allowed. A dial that saw
// an allowed candidate but still failed is a network error, not a denial.
func (a *dialAttempt) denial() (targetHostDenial, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.allowedSeen || !a.hasBlocked {
		return targetHostDenial{}, false
	}
	return targetHostDenial{
		Host:          a.host,
		Port:          a.port,
		ResolvedIPs:   slices.Clone(a.resolved),
		Policy:        a.policy.mode(),
		BlockedIP:     a.blockedIP,
		BlockedPrefix: a.policy.deniedPrefix(a.blockedIP),
	}, true
}

// canonicalAddr normalizes a resolved address for policy evaluation. It strips
// any IPv6 zone identifier and unmaps IPv4-in-IPv6 addresses. Both are required
// for correct prefix matching: netip.Prefix.Contains always returns false for an
// address carrying a zone, and an IPv4-mapped IPv6 address would not match an
// IPv4 prefix. Without this normalization a target resolving to (or configured
// as) e.g. "fe80::1%eth0" or "::ffff:169.254.169.254" would silently slip past a
// deny rule.
func canonicalAddr(addr netip.Addr) netip.Addr {
	return addr.WithZone("").Unmap()
}

func deniedTargetHostError(host string) error {
	return trace.AccessDenied("application target host %q is not permitted by app_service target host policy", host)
}

func stringifyAddrs(addrs []netip.Addr) []string {
	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		out = append(out, stringifyAddr(addr))
	}
	return out
}

func stringifyAddr(addr netip.Addr) string {
	if !addr.IsValid() {
		return ""
	}
	return addr.String()
}

func stringifyPrefix(prefix netip.Prefix) string {
	if !prefix.IsValid() {
		return ""
	}
	return prefix.String()
}
