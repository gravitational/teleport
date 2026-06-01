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
	"slices"
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
	// VNetDNSIPv4 is the address of VNet's IPv4 DNS server. Zero if VNet is not
	// serving DNS on IPv4.
	VNetDNSIPv4 netip.AddrPort
	// VNetDNSIPv6 is the address of VNet's IPv6 DNS server. Zero if VNet is not
	// serving DNS on IPv6.
	VNetDNSIPv6 netip.AddrPort
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
// applications use, so per-zone OS resolver routing is honored. The network
// argument is "ip4" or "ip6" so the caller can ask each record type
// independently.
type Resolver interface {
	Lookup(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// DirectQuerier sends a DNS query directly to a specific server, bypassing the
// OS resolver. Used by the reachability check to verify VNet's DNS process is
// reachable independent of OS resolver configuration. The network argument is
// "ip4" or "ip6" so the caller can ask each record type independently.
type DirectQuerier interface {
	LookupDirect(ctx context.Context, network, host string, server netip.AddrPort) ([]netip.Addr, error)
}

// recordResult is the result of probing one server for one record type.
type recordResult struct {
	addr netip.Addr
	err  error
}

// DNSDiag is the diagnostic check that verifies the OS resolver routes queries
// for each VNet-managed DNS zone to VNet's nameserver. It runs two steps:
//
//  1. Reachability check: direct DNS queries to VNet's IPv4 and IPv6 nameservers
//     asking for both A and AAAA records. If neither nameserver returns a
//     response for either record type, the per-zone check is skipped.
//  2. Per-zone check: for each zone, A and AAAA queries through the OS resolver.
//     Each response is compared to the reachability response to determine whether
//     the OS resolver routed the query to VNet (OK), to some other resolver
//     (HIJACKED), or not at all (NOT_REGISTERED / TIMEOUT / RESOLVER_ERROR).
type DNSDiag struct {
	cfg *DNSConfig
}

// NewDNSDiag returns a new DNSDiag.
func NewDNSDiag(cfg *DNSConfig) (*DNSDiag, error) {
	if !cfg.VNetDNSIPv4.IsValid() && !cfg.VNetDNSIPv6.IsValid() {
		return nil, trace.BadParameter("at least one of VNetDNSIPv4 or VNetDNSIPv6 must be set")
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

	v4, v6 := d.runReachabilityCheck(ctx)
	report.Ipv4Reachability = toReachabilityProto(v4)
	report.Ipv6Reachability = toReachabilityProto(v6)

	expectedA, expectedAAAA := mergeExpected(ctx, v6, v4)

	if expectedA.IsValid() || expectedAAAA.IsValid() {
		report.ZoneResults = d.runPerZoneCheck(ctx, expectedA, expectedAAAA)
	}

	return &diagv1.CheckReport{
		Status: computeReportStatus(report),
		Report: &diagv1.CheckReport_DnsReport{DnsReport: report},
	}, nil
}

// reachabilityCheckResult captures the result of probing one VNet nameserver for both A and AAAA.
type reachabilityCheckResult struct {
	server netip.AddrPort
	a      recordResult
	aaaa   recordResult
}

// runReachabilityCheck probes each configured VNet nameserver for both A and AAAA records
func (d *DNSDiag) runReachabilityCheck(ctx context.Context) (v4, v6 reachabilityCheckResult) {
	v6.server = d.cfg.VNetDNSIPv6
	v4.server = d.cfg.VNetDNSIPv4

	var wg sync.WaitGroup
	if v6.server.IsValid() {
		wg.Go(func() { v6.a = d.queryServer(ctx, "ip4", v6.server) })
		wg.Go(func() { v6.aaaa = d.queryServer(ctx, "ip6", v6.server) })
	}
	if v4.server.IsValid() {
		wg.Go(func() { v4.a = d.queryServer(ctx, "ip4", v4.server) })
		wg.Go(func() { v4.aaaa = d.queryServer(ctx, "ip6", v4.server) })
	}
	wg.Wait()
	return
}

func (d *DNSDiag) queryServer(ctx context.Context, network string, server netip.AddrPort) recordResult {
	var lastErr error
	// Retries to skip the startup window where VNet's DNS isn't responding yet.
	for attempt := 0; attempt < d.cfg.ReachabilityMaxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return recordResult{err: trace.Wrap(ctx.Err(), "querying VNet DNS server at %s for %s", server, network)}
			case <-time.After(d.cfg.ReachabilityRetryDelay):
			}
		}
		addr, err := d.queryServerOnce(ctx, network, server)
		if err == nil {
			return recordResult{addr: addr}
		}
		lastErr = err
	}
	return recordResult{err: lastErr}
}

// queryServerOnce fires a single direct DNS query for one record type.
func (d *DNSDiag) queryServerOnce(ctx context.Context, network string, server netip.AddrPort) (netip.Addr, error) {
	ctx, cancel := context.WithTimeout(ctx, d.cfg.QueryTimeout)
	defer cancel()

	addrs, err := d.cfg.DirectQuerier.LookupDirect(ctx, network, probeFQDN(reachabilityProbeZone), server)
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			return netip.Addr{}, nil
		}
		return netip.Addr{}, trace.Wrap(err, "querying VNet DNS server at %s for %s", server, network)
	}
	if len(addrs) == 0 {
		return netip.Addr{}, nil
	}
	return addrs[0], nil
}

func toReachabilityProto(o reachabilityCheckResult) *diagv1.VNetDNSReachability {
	// At this point server may be either empty or valid. An empty server means
	// VNet does not serve DNS on the address family this check was performed for
	if !o.server.IsValid() {
		return nil
	}
	m := &diagv1.VNetDNSReachability{
		Address:       o.server.String(),
		RespondedA:    o.a.addr.IsValid(),
		RespondedAaaa: o.aaaa.addr.IsValid(),
	}
	m.Reachable = m.RespondedA || m.RespondedAaaa
	if !m.Reachable {
		var errs []string
		if o.a.err != nil {
			errs = append(errs, fmt.Sprintf("A: %s", o.a.err))
		}
		if o.aaaa.err != nil {
			errs = append(errs, fmt.Sprintf("AAAA: %s", o.aaaa.err))
		}
		m.Error = strings.Join(errs, "; ")
		if m.Error == "" {
			m.Error = "server returned no records"
		}
	}
	return m
}

// mergeExpected picks the expected A and AAAA from the reachability results.
// Prefers IPv6 to match the order in which the OS resolver is most likely to
// query.
//
// On macOS and Windows the OS prefers IPv6 over IPv4 per RFC 6724, on Linux
// with systemd-resolved we register the IPv6 nameserver first and
// systemd-resolved respects configuration order.
func mergeExpected(ctx context.Context, v6, v4 reachabilityCheckResult) (expectedA, expectedAAAA netip.Addr) {
	if v6.a.addr.IsValid() {
		expectedA = v6.a.addr
	} else {
		expectedA = v4.a.addr
	}
	if v6.aaaa.addr.IsValid() {
		expectedAAAA = v6.aaaa.addr
	} else {
		expectedAAAA = v4.aaaa.addr
	}

	// Log a warning when both nameservers returned a value for the same record and
	// they disagree that should never happen and indicates a bug IN VNet
	if v6.a.addr.IsValid() && v4.a.addr.IsValid() && v6.a.addr != v4.a.addr {
		log.WarnContext(ctx, "VNet DNS returned different A records from IPv6 vs IPv4 nameservers",
			"v6_response", v6.a.addr, "v4_response", v4.a.addr)
	}
	if v6.aaaa.addr.IsValid() && v4.aaaa.addr.IsValid() && v6.aaaa.addr != v4.aaaa.addr {
		log.WarnContext(ctx, "VNet DNS returned different AAAA records from IPv6 vs IPv4 nameservers",
			"v6_response", v6.aaaa.addr, "v4_response", v4.aaaa.addr)
	}
	return
}

// runPerZoneCheck queries the probe for each configured zone through the OS
// resolver in parallel, asking for each record type for which an expected IP
// was captured.
func (d *DNSDiag) runPerZoneCheck(ctx context.Context, expectedA, expectedAAAA netip.Addr) []*diagv1.DNSZoneResult {
	results := make([]*diagv1.DNSZoneResult, len(d.cfg.DNSZones))
	var wg sync.WaitGroup
	for i, zone := range d.cfg.DNSZones {
		wg.Go(func() {
			results[i] = d.queryZone(ctx, zone, expectedA, expectedAAAA)
		})
	}
	wg.Wait()
	return results
}

// queryZone runs A and AAAA queries for a single zone in parallel.
func (d *DNSDiag) queryZone(ctx context.Context, zone string, expectedA, expectedAAAA netip.Addr) *diagv1.DNSZoneResult {
	result := &diagv1.DNSZoneResult{Zone: zone}
	var wg sync.WaitGroup
	if expectedA.IsValid() {
		wg.Go(func() {
			result.ARecord = d.queryZoneRecord(ctx, zone, "ip4", expectedA)
		})
	}
	if expectedAAAA.IsValid() {
		wg.Go(func() {
			result.AaaaRecord = d.queryZoneRecord(ctx, zone, "ip6", expectedAAAA)
		})
	}
	wg.Wait()
	return result
}

// queryZoneRecord runs a single record-type query.
func (d *DNSDiag) queryZoneRecord(ctx context.Context, zone, network string, expected netip.Addr) *diagv1.RecordResult {
	result := d.queryZoneRecordOnce(ctx, zone, network, expected)
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
	return d.queryZoneRecordOnce(ctx, zone, network, expected)
}

func (d *DNSDiag) queryZoneRecordOnce(ctx context.Context, zone, network string, expected netip.Addr) *diagv1.RecordResult {
	ctx, cancel := context.WithTimeout(ctx, d.cfg.QueryTimeout)
	defer cancel()

	addrs, err := d.cfg.Resolver.Lookup(ctx, network, probeFQDN(zone))
	return classifyRecordResult(addrs, err, expected)
}

// classifyRecordResult maps a resolver outcome for a single record type to a
// [diagv1.RecordResult].
func classifyRecordResult(addrs []netip.Addr, err error, expected netip.Addr) *diagv1.RecordResult {
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) {
			switch {
			case dnsErr.IsNotFound:
				return &diagv1.RecordResult{Status: diagv1.DNSZoneStatus_DNS_ZONE_STATUS_NOT_REGISTERED}
			case dnsErr.IsTimeout:
				return &diagv1.RecordResult{
					Status: diagv1.DNSZoneStatus_DNS_ZONE_STATUS_TIMEOUT,
					Error:  err.Error(),
				}
			}
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return &diagv1.RecordResult{
				Status: diagv1.DNSZoneStatus_DNS_ZONE_STATUS_TIMEOUT,
				Error:  err.Error(),
			}
		}
		return &diagv1.RecordResult{
			Status: diagv1.DNSZoneStatus_DNS_ZONE_STATUS_RESOLVER_ERROR,
			Error:  err.Error(),
		}
	}
	if len(addrs) == 0 {
		return &diagv1.RecordResult{Status: diagv1.DNSZoneStatus_DNS_ZONE_STATUS_NOT_REGISTERED}
	}
	if slices.Contains(addrs, expected) {
		return &diagv1.RecordResult{Status: diagv1.DNSZoneStatus_DNS_ZONE_STATUS_OK}
	}
	return &diagv1.RecordResult{
		Status:     diagv1.DNSZoneStatus_DNS_ZONE_STATUS_HIJACKED,
		ObservedIp: addrs[0].String(),
	}
}

// computeReportStatus returns ISSUES_FOUND if any configured reachability check
// failed, or if any per-zone record result is not OK; otherwise OK.
func computeReportStatus(report *diagv1.DNSReport) diagv1.CheckReportStatus {
	for _, r := range []*diagv1.VNetDNSReachability{report.Ipv4Reachability, report.Ipv6Reachability} {
		if r != nil && !r.Reachable {
			return diagv1.CheckReportStatus_CHECK_REPORT_STATUS_ISSUES_FOUND
		}
	}
	for _, zr := range report.ZoneResults {
		if rr := zr.ARecord; rr != nil && rr.Status != diagv1.DNSZoneStatus_DNS_ZONE_STATUS_OK {
			return diagv1.CheckReportStatus_CHECK_REPORT_STATUS_ISSUES_FOUND
		}
		if rr := zr.AaaaRecord; rr != nil && rr.Status != diagv1.DNSZoneStatus_DNS_ZONE_STATUS_OK {
			return diagv1.CheckReportStatus_CHECK_REPORT_STATUS_ISSUES_FOUND
		}
	}
	return diagv1.CheckReportStatus_CHECK_REPORT_STATUS_OK
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

// DNSServerForIPv4CIDRRange returns the address of VNet's IPv4 DNS server for a given IPv4
// CIDR range (e.g., 100.64.0.0/24 → 100.64.0.2:53). Matches lib/vnet/osconfig_provider.go.
func DNSServerForIPv4CIDRRange(cidr string) (netip.AddrPort, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return netip.AddrPort{}, trace.Wrap(err, "parsing IPv4 CIDR %q", cidr)
	}
	ip := slices.Clone(ipNet.IP)
	ip[len(ip)-1] += 2
	addr, _ := netip.AddrFromSlice(ip)
	return netip.AddrPortFrom(addr, vnetdns.DNSServerPort), nil
}

// NetResolver is the default [Resolver] implementation that uses [net.DefaultResolver].
type NetResolver struct{}

// Lookup implements [Resolver].
func (NetResolver) Lookup(ctx context.Context, network, host string) ([]netip.Addr, error) {
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, network, host)
	return addrs, trace.Wrap(err)
}

// NetDirectQuerier is the default [DirectQuerier] implementation. It uses a
// custom [net.Resolver] whose Dial function forces every connection to the
// configured target server, bypassing the OS resolver entirely.
type NetDirectQuerier struct{}

// LookupDirect implements [DirectQuerier].
func (NetDirectQuerier) LookupDirect(ctx context.Context, network, host string, server netip.AddrPort) ([]netip.Addr, error) {
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return new(net.Dialer).DialContext(ctx, "udp", server.String())
		},
	}
	addrs, err := r.LookupNetIP(ctx, network, host)
	return addrs, trace.Wrap(err)
}
