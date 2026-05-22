// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package diag

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"

	diagv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/diag/v1"
	vnetdns "github.com/gravitational/teleport/lib/vnet/dns"
)

const (
	defaultDNSQueryTimeout         = 2 * time.Second
	defaultNotRegisteredRetryDelay = 500 * time.Millisecond
	defaultReachabilityMaxAttempts = 3
	defaultReachabilityRetryDelay  = 500 * time.Millisecond
	// reachabilityProbeZone is the TLD used to build the reachability probe FQDN.
	// VNet's probe handler matches on the "vnet-diag-" prefix only, so the zone
	// is purely DNS-message filler. ".test" is RFC 2606 reserved.
	reachabilityProbeZone = "test"
)

// DNSConfig is the configuration for DNSDiag.
type DNSConfig struct {
	// DNSZones is the list of DNS zones registered with the OS resolver. The
	// check sends one query per zone through the OS resolver and verifies it
	// reached VNet's nameserver.
	DNSZones []string
	// VNetDNSServer is the address of VNet's DNS server
	VNetDNSServer netip.AddrPort
	// Resolver does name resolution through the OS resolver.
	Resolver Resolver
	// DirectQuerier sends DNS queries directly to a specific nameserver, bypassing the OS resolver.
	DirectQuerier DirectQuerier
	// QueryTimeout caps the duration of an individual DNS query.
	QueryTimeout time.Duration
	// NotRegisteredRetryDelay delays the retry of a per-zone query that returned NOT_REGISTERED.
	NotRegisteredRetryDelay time.Duration
	// ReachabilityMaxAttempts caps reachability-check retries before reporting
	// VNet's DNS as unreachable.
	ReachabilityMaxAttempts int
	// ReachabilityRetryDelay is the backoff between reachability attempts.
	ReachabilityRetryDelay time.Duration
}

// Resolver does name resolution through the OS resolver the same path real
// applications use, so per-zone OS resolver routing is honored.
type Resolver interface {
	Lookup(ctx context.Context, host string) ([]netip.Addr, error)
}

// DirectQuerier sends a DNS query directly to a specific server, bypassing the
// OS resolver. Used by the reachability check to verify VNet's DNS process is
// reachable independent of OS resolver configuration.
type DirectQuerier interface {
	LookupDirect(ctx context.Context, host string, server netip.AddrPort) ([]netip.Addr, error)
}

// DNSDiag is the diagnostic check that verifies the OS resolver routes queries
// for each VNet-managed DNS zone to VNet's nameserver. It runs two steps:
//
//  1. Reachability check: a direct DNS query to VNet's nameserver. If this
//     fails, VNet's DNS is unreachable and the per-zone check is skipped.
//  2. Per-zone check: one query per zone through the OS resolver. Each
//     response is compared to the reachability response to determine whether
//     the OS resolver routed the query to VNet (OK), to some other resolver
//     (HIJACKED), or not at all (NOT_REGISTERED / TIMEOUT).
type DNSDiag struct {
	cfg *DNSConfig
}

// NewDNSDiag returns a new DNSDiag.
func NewDNSDiag(cfg *DNSConfig) (*DNSDiag, error) {
	if !cfg.VNetDNSServer.IsValid() {
		return nil, trace.BadParameter("missing VNet DNS server address")
	}
	if cfg.Resolver == nil {
		cfg.Resolver = NetResolver{}
	}
	if cfg.DirectQuerier == nil {
		cfg.DirectQuerier = NetDirectQuerier{}
	}
	if cfg.QueryTimeout == 0 {
		cfg.QueryTimeout = defaultDNSQueryTimeout
	}
	if cfg.NotRegisteredRetryDelay == 0 {
		cfg.NotRegisteredRetryDelay = defaultNotRegisteredRetryDelay
	}
	if cfg.ReachabilityMaxAttempts == 0 {
		cfg.ReachabilityMaxAttempts = defaultReachabilityMaxAttempts
	}
	if cfg.ReachabilityRetryDelay == 0 {
		cfg.ReachabilityRetryDelay = defaultReachabilityRetryDelay
	}
	return &DNSDiag{cfg: cfg}, nil
}

func (d *DNSDiag) EmptyCheckReport() *diagv1.CheckReport {
	return &diagv1.CheckReport{Report: &diagv1.CheckReport_DnsReport{}}
}

// Commands returns platform-specific commands that capture additional DNS configuration state.
func (d *DNSDiag) Commands(ctx context.Context) []*exec.Cmd {
	return d.commands(ctx)
}

// Run executes the DNS check.
func (d *DNSDiag) Run(ctx context.Context) (*diagv1.CheckReport, error) {
	report := &diagv1.DNSReport{}

	// Reachability check
	expectedIP, err := d.runReachabilityCheck(ctx)
	if err != nil {
		report.VnetDnsReachable = false
		report.VnetDnsUnreachableError = err.Error()
		return &diagv1.CheckReport{
			Status: diagv1.CheckReportStatus_CHECK_REPORT_STATUS_ISSUES_FOUND,
			Report: &diagv1.CheckReport_DnsReport{DnsReport: report},
		}, nil
	}
	report.VnetDnsReachable = true

	// Per-zone check
	report.ZoneResults = d.runPerZoneCheck(ctx, expectedIP)

	status := diagv1.CheckReportStatus_CHECK_REPORT_STATUS_OK
	for _, zr := range report.ZoneResults {
		if zr.Status != diagv1.DNSZoneStatus_DNS_ZONE_STATUS_OK {
			status = diagv1.CheckReportStatus_CHECK_REPORT_STATUS_ISSUES_FOUND
			break
		}
	}
	return &diagv1.CheckReport{
		Status: status,
		Report: &diagv1.CheckReport_DnsReport{DnsReport: report},
	}, nil
}

// runReachabilityCheck queries VNet's nameserver directly and returns the response
// IP, which becomes the expected_ip for every per-zone result.
func (d *DNSDiag) runReachabilityCheck(ctx context.Context) (netip.Addr, error) {
	var lastErr error
	// Retries to skip the startup window where VNet's DNS isn't responding yet.
	for attempt := 0; attempt < d.cfg.ReachabilityMaxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return netip.Addr{}, trace.Wrap(ctx.Err(), "querying VNet DNS server at %s", d.cfg.VNetDNSServer)
			case <-time.After(d.cfg.ReachabilityRetryDelay):
			}
		}
		addr, err := d.queryReachabilityOnce(ctx)
		if err == nil {
			return addr, nil
		}
		lastErr = err
	}
	return netip.Addr{}, lastErr
}

func (d *DNSDiag) queryReachabilityOnce(ctx context.Context) (netip.Addr, error) {
	ctx, cancel := context.WithTimeout(ctx, d.cfg.QueryTimeout)
	defer cancel()

	addrs, err := d.cfg.DirectQuerier.LookupDirect(ctx, probeFQDN(reachabilityProbeZone), d.cfg.VNetDNSServer)
	if err != nil {
		return netip.Addr{}, trace.Wrap(err, "querying VNet DNS server at %s", d.cfg.VNetDNSServer)
	}
	if len(addrs) == 0 {
		return netip.Addr{}, trace.Errorf("VNet DNS server at %s returned no answer", d.cfg.VNetDNSServer)
	}
	return addrs[0], nil
}

// runPerZoneCheck queries the probe for each configured zone through the OS
// resolver in parallel and returns a per-zone result.
func (d *DNSDiag) runPerZoneCheck(ctx context.Context, expectedIP netip.Addr) []*diagv1.DNSZoneResult {
	results := make([]*diagv1.DNSZoneResult, len(d.cfg.DNSZones))
	var wg sync.WaitGroup
	for i, zone := range d.cfg.DNSZones {
		wg.Go(func() {
			results[i] = d.queryZone(ctx, zone, expectedIP)
		})
	}
	wg.Wait()
	return results
}

// queryZone runs a single zone query with one retry on NOT_REGISTERED, to
// skip a possible gap where the OS resolver hasn't yet picked up VNet's per-zone entry.
func (d *DNSDiag) queryZone(ctx context.Context, zone string, expectedIP netip.Addr) *diagv1.DNSZoneResult {
	result := d.queryZoneOnce(ctx, zone, expectedIP)
	if result.Status != diagv1.DNSZoneStatus_DNS_ZONE_STATUS_NOT_REGISTERED {
		return result
	}
	if d.cfg.NotRegisteredRetryDelay <= 0 {
		return result
	}
	select {
	case <-ctx.Done():
		return result
	case <-time.After(d.cfg.NotRegisteredRetryDelay):
	}
	return d.queryZoneOnce(ctx, zone, expectedIP)
}

func (d *DNSDiag) queryZoneOnce(ctx context.Context, zone string, expectedIP netip.Addr) *diagv1.DNSZoneResult {
	ctx, cancel := context.WithTimeout(ctx, d.cfg.QueryTimeout)
	defer cancel()

	addrs, err := d.cfg.Resolver.Lookup(ctx, probeFQDN(zone))
	status, observed, errStr := classifyDNSResult(addrs, err, expectedIP)

	result := &diagv1.DNSZoneResult{
		Zone:       zone,
		Status:     status,
		ExpectedIp: expectedIP.String(),
		Error:      errStr,
	}
	if observed.IsValid() {
		result.ObservedIp = observed.String()
	}
	return result
}

// classifyDNSResult maps a resolver outcome to a [diagv1.DNSZoneStatus].
func classifyDNSResult(addrs []netip.Addr, err error, expectedIP netip.Addr) (diagv1.DNSZoneStatus, netip.Addr, string) {
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) {
			switch {
			case dnsErr.IsNotFound:
				return diagv1.DNSZoneStatus_DNS_ZONE_STATUS_NOT_REGISTERED, netip.Addr{}, ""
			case dnsErr.IsTimeout:
				return diagv1.DNSZoneStatus_DNS_ZONE_STATUS_TIMEOUT, netip.Addr{}, err.Error()
			}
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return diagv1.DNSZoneStatus_DNS_ZONE_STATUS_TIMEOUT, netip.Addr{}, err.Error()
		}
		return diagv1.DNSZoneStatus_DNS_ZONE_STATUS_RESOLVER_ERROR, netip.Addr{}, err.Error()
	}
	// NOERROR with no answer is also classified as NOT_REGISTERED.
	if len(addrs) == 0 {
		return diagv1.DNSZoneStatus_DNS_ZONE_STATUS_NOT_REGISTERED, netip.Addr{}, ""
	}
	if len(addrs) == 1 && addrs[0] == expectedIP {
		return diagv1.DNSZoneStatus_DNS_ZONE_STATUS_OK, addrs[0], ""
	}
	// HIJACKED: surface the IPv6 if present, probes get AAAA only from VNet
	// so a non-VNet IPv6 is the most direct evidence of interception.
	//
	// TODO(tangyatsu): proto carries a single observed_ip today. Consider
	// widening to a repeated field so the UI can show multiple addresses for
	// dual-stack hijackers or round-robin pools.
	for _, a := range addrs {
		if a.Is6() {
			return diagv1.DNSZoneStatus_DNS_ZONE_STATUS_HIJACKED, a, ""
		}
	}
	return diagv1.DNSZoneStatus_DNS_ZONE_STATUS_HIJACKED, addrs[0], ""
}

// probeFQDN builds a probe hostname for a given zone with a fresh
// random label, defeating any intermediate DNS caches. Format:
//
//	vnet-diag-<hex>.<zone>.
func probeFQDN(zone string) string {
	const probeRandomBytes = 8
	var b [probeRandomBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand should never fail; if it does, return a deterministic
		// fallback so the diag check at least produces a result.
		return fmt.Sprintf("%sfallback.%s.", vnetdns.DiagProbePrefix, strings.TrimSuffix(zone, "."))
	}
	return fmt.Sprintf("%s%s.%s.", vnetdns.DiagProbePrefix, hex.EncodeToString(b[:]), strings.TrimSuffix(zone, "."))
}

// DNSServerForIPv6Prefix returns the address of VNet's IPv6 DNS server for a
// given IPv6 ULA prefix (e.g., fdec:1fed:139f:: → [fdec:1fed:139f::2]:53).
func DNSServerForIPv6Prefix(prefix string) (netip.AddrPort, error) {
	addr, err := netip.ParseAddr(prefix)
	if err != nil {
		return netip.AddrPort{}, trace.Wrap(err, "parsing IPv6 prefix %q", prefix)
	}
	if !addr.Is6() {
		return netip.AddrPort{}, trace.BadParameter("prefix %q is not IPv6", prefix)
	}
	// Same suffix VNet uses internally
	b := addr.As16()
	copy(b[16-len(vnetdns.DNSServerSuffix):], vnetdns.DNSServerSuffix)
	return netip.AddrPortFrom(netip.AddrFrom16(b), vnetdns.DNSServerPort), nil
}

// NetResolver is the default [Resolver] implementation that uses [net.DefaultResolver].
type NetResolver struct{}

// Lookup implements [Resolver].
func (NetResolver) Lookup(ctx context.Context, host string) ([]netip.Addr, error) {
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	return addrs, trace.Wrap(err)
}

// NetDirectQuerier is the default [DirectQuerier] implementation. It uses a
// custom [net.Resolver] whose Dial function forces every connection to the
// configured target server, bypassing the OS resolver entirely.
type NetDirectQuerier struct{}

// LookupDirect implements [DirectQuerier].
func (NetDirectQuerier) LookupDirect(ctx context.Context, host string, server netip.AddrPort) ([]netip.Addr, error) {
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, network, server.String())
		},
	}
	addrs, err := r.LookupNetIP(ctx, "ip", host)
	return addrs, trace.Wrap(err)
}
