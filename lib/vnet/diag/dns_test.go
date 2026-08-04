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
	"errors"
	"net"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	diagv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/diag/v1"
)

// fakeResolver and fakeDirectQuerier inject lookup behavior per test.
type fakeResolver struct {
	lookup func(ctx context.Context, network, host string) ([]netip.Addr, error)
}

func (f fakeResolver) Lookup(ctx context.Context, network, host string) ([]netip.Addr, error) {
	return f.lookup(ctx, network, host)
}

type fakeDirectQuerier struct {
	lookup func(ctx context.Context, network, host string, server netip.AddrPort) ([]netip.Addr, error)
}

func (f fakeDirectQuerier) LookupDirect(ctx context.Context, network, host string, server netip.AddrPort) ([]netip.Addr, error) {
	return f.lookup(ctx, network, host, server)
}

var (
	testVNetDNSv6    = netip.MustParseAddrPort("[fdec:1fed:139f::2]:53")
	testVNetDNSv4    = netip.MustParseAddrPort("100.64.0.2:53")
	testExpectedAAAA = netip.MustParseAddr("fdec:1fed:139f::2")
	testExpectedA    = netip.MustParseAddr("100.64.0.2")
	testHijackAAAA   = netip.MustParseAddr("2001:db8::1234")
	testHijackA      = netip.MustParseAddr("192.0.2.42")
)

func newTestDNSDiag(t *testing.T, cfg DNSConfig) *DNSDiag {
	t.Helper()
	if !cfg.VNetDNSIPv4.IsValid() && !cfg.VNetDNSIPv6.IsValid() {
		cfg.VNetDNSIPv4 = testVNetDNSv4
		cfg.VNetDNSIPv6 = testVNetDNSv6
	}
	if cfg.QueryTimeout == 0 {
		cfg.QueryTimeout = 100 * time.Millisecond
	}
	if cfg.NotRegisteredRetryDelay == 0 {
		cfg.NotRegisteredRetryDelay = time.Nanosecond
	}
	if cfg.ReachabilityRetryDelay == 0 {
		cfg.ReachabilityRetryDelay = time.Nanosecond
	}
	d, err := NewDNSDiag(&cfg)
	require.NoError(t, err)
	return d
}

// goodDirectQuerier is a fakeDirectQuerier that returns the expected A and AAAA
// for any server.
var goodDirectQuerier = constantDirectQuerier(testExpectedA, testExpectedAAAA)

// constantDirectQuerier returns a, aaaa for every query regardless of server.
// Either argument may be a zero addr to fake a server that doesn't support
// this record type.
func constantDirectQuerier(a, aaaa netip.Addr) fakeDirectQuerier {
	return fakeDirectQuerier{
		lookup: func(_ context.Context, network, _ string, _ netip.AddrPort) ([]netip.Addr, error) {
			switch network {
			case "ip4":
				if a.IsValid() {
					return []netip.Addr{a}, nil
				}
			case "ip6":
				if aaaa.IsValid() {
					return []netip.Addr{aaaa}, nil
				}
			}
			return nil, nil
		},
	}
}

// goodResolver is a fakeResolver that returns the expected A and AAAA for any
// host.
var goodResolver = constantResolver(testExpectedA, testExpectedAAAA)

// constantResolver returns a, aaaa for every query regardless of host. Either
// argument may be a zero addr to fake a resolver that doesn't return this
// record type.
func constantResolver(a, aaaa netip.Addr) fakeResolver {
	return fakeResolver{
		lookup: func(_ context.Context, network, _ string) ([]netip.Addr, error) {
			switch network {
			case "ip4":
				if a.IsValid() {
					return []netip.Addr{a}, nil
				}
			case "ip6":
				if aaaa.IsValid() {
					return []netip.Addr{aaaa}, nil
				}
			}
			return nil, nil
		},
	}
}

// zoneOf extracts the zone from a probe FQDN
func zoneOf(fqdn string) string {
	const prefix = "vnet-diag-"
	rest, ok := strings.CutPrefix(fqdn, prefix)
	if !ok {
		return ""
	}
	// rest is "<hex>.<zone>."
	_, zone, ok := strings.Cut(rest, ".")
	if !ok {
		return ""
	}
	return strings.TrimSuffix(zone, ".")
}

func TestDNSDiagAllZonesOK(t *testing.T) {
	d := newTestDNSDiag(t, DNSConfig{
		DNSZones:      []string{"company.test", "example.test"},
		DirectQuerier: goodDirectQuerier,
		Resolver:      goodResolver,
	})

	report, err := d.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, diagv1.CheckReportStatus_CHECK_REPORT_STATUS_OK, report.GetStatus())
	dnsReport := report.GetDnsReport()
	require.NotNil(t, dnsReport.GetIpv4Reachability())
	require.True(t, dnsReport.GetIpv4Reachability().GetReachable())
	require.True(t, dnsReport.GetIpv4Reachability().GetRespondedA())
	require.True(t, dnsReport.GetIpv4Reachability().GetRespondedAaaa())
	require.NotNil(t, dnsReport.GetIpv6Reachability())
	require.True(t, dnsReport.GetIpv6Reachability().GetReachable())
	require.True(t, dnsReport.GetIpv6Reachability().GetRespondedA())
	require.True(t, dnsReport.GetIpv6Reachability().GetRespondedAaaa())
	require.Len(t, dnsReport.GetZoneResults(), 2)
	for _, zr := range dnsReport.GetZoneResults() {
		require.NotNil(t, zr.GetARecord(), zr.GetZone())
		require.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_OK, zr.GetARecord().GetStatus(), zr.GetZone())
		require.NotNil(t, zr.GetAaaaRecord(), zr.GetZone())
		require.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_OK, zr.GetAaaaRecord().GetStatus(), zr.GetZone())
	}
}

func TestDNSDiagBothReachabilityFailureSkipsPerZone(t *testing.T) {
	resolverCalled := false
	d := newTestDNSDiag(t, DNSConfig{
		DNSZones: []string{"company.test"},
		DirectQuerier: fakeDirectQuerier{
			lookup: func(context.Context, string, string, netip.AddrPort) ([]netip.Addr, error) {
				return nil, errors.New("connection refused")
			},
		},
		Resolver: fakeResolver{
			lookup: func(context.Context, string, string) ([]netip.Addr, error) {
				resolverCalled = true
				return []netip.Addr{testExpectedAAAA}, nil
			},
		},
	})

	report, err := d.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, diagv1.CheckReportStatus_CHECK_REPORT_STATUS_ISSUES_FOUND, report.GetStatus())
	dnsReport := report.GetDnsReport()
	require.False(t, dnsReport.GetIpv4Reachability().GetReachable())
	require.Contains(t, dnsReport.GetIpv4Reachability().GetError(), "connection refused")
	require.False(t, dnsReport.GetIpv6Reachability().GetReachable())
	require.Contains(t, dnsReport.GetIpv6Reachability().GetError(), "connection refused")
	require.Empty(t, dnsReport.GetZoneResults(), "per-zone check must be skipped when no nameserver returned expected IPs")
	require.False(t, resolverCalled, "OS resolver must not be called when both VNet DNS nameservers are unreachable")
}

func TestDNSDiagOnlyIPv6Reachable(t *testing.T) {
	// IPv4 server is unreachable; IPv6 server returns both records. Per-zone
	// check should still run for both record types using IPv6's response.
	d := newTestDNSDiag(t, DNSConfig{
		DNSZones: []string{"company.test"},
		DirectQuerier: fakeDirectQuerier{
			lookup: func(ctx context.Context, network, host string, server netip.AddrPort) ([]netip.Addr, error) {
				if server != testVNetDNSv6 {
					return nil, errors.New("network unreachable")
				}
				return goodDirectQuerier.lookup(ctx, network, host, server)
			},
		},
		Resolver: goodResolver,
	})

	report, err := d.Run(t.Context())
	require.NoError(t, err)
	dnsReport := report.GetDnsReport()
	require.False(t, dnsReport.GetIpv4Reachability().GetReachable())
	require.True(t, dnsReport.GetIpv6Reachability().GetReachable())
	require.Len(t, dnsReport.GetZoneResults(), 1)
	zr := dnsReport.GetZoneResults()[0]
	require.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_OK, zr.GetARecord().GetStatus(),
		"A still verified because the IPv6 nameserver returned an expected A")
	require.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_OK, zr.GetAaaaRecord().GetStatus())
	// One unreachable nameserver does not fail the overall check
	require.Equal(t, diagv1.CheckReportStatus_CHECK_REPORT_STATUS_OK, report.GetStatus())
}

func TestDNSDiagOnlyIPv6ServerNoAAnswer(t *testing.T) {
	// Only the IPv6 nameserver is configured, and it returns AAAA only.
	d := newTestDNSDiag(t, DNSConfig{
		VNetDNSIPv6:   testVNetDNSv6,
		DNSZones:      []string{"company.test"},
		DirectQuerier: constantDirectQuerier(netip.Addr{}, testExpectedAAAA),
		Resolver: fakeResolver{
			lookup: func(_ context.Context, network, _ string) ([]netip.Addr, error) {
				if network == "ip4" {
					t.Fatalf("A lookup must not happen when no expected A was captured")
				}
				return []netip.Addr{testExpectedAAAA}, nil
			},
		},
	})

	report, err := d.Run(t.Context())
	require.NoError(t, err)
	dnsReport := report.GetDnsReport()
	require.Nil(t, dnsReport.GetIpv4Reachability(), "IPv4 reachability must be unset when IPv4 server is not configured")
	require.NotNil(t, dnsReport.GetIpv6Reachability())
	require.True(t, dnsReport.GetIpv6Reachability().GetReachable())
	require.True(t, dnsReport.GetIpv6Reachability().GetRespondedAaaa())
	require.False(t, dnsReport.GetIpv6Reachability().GetRespondedA(), "expected no A response")
	require.Len(t, dnsReport.GetZoneResults(), 1)
	zr := dnsReport.GetZoneResults()[0]
	require.Nil(t, zr.GetARecord(), "A record result must be nil when no expected A was captured")
	require.NotNil(t, zr.GetAaaaRecord())
	require.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_OK, zr.GetAaaaRecord().GetStatus())
}

func TestDNSDiagPerRecordTypeClassification(t *testing.T) {
	notFoundErr := &net.DNSError{Name: "x", IsNotFound: true}
	timeoutErr := &net.DNSError{Name: "x", IsTimeout: true, Err: "i/o timeout"}
	unexpectedErr := errors.New("upstream SERVFAIL")

	zones := []string{
		"ok.test",
		"a-hijacked.test",
		"aaaa-hijacked.test",
		"mixed.test", // A TIMEOUT, AAAA HIJACKED
		"missing.test",
		"slow.test",
		"borked.test",
	}

	d := newTestDNSDiag(t, DNSConfig{
		DNSZones:      zones,
		DirectQuerier: goodDirectQuerier,
		Resolver: fakeResolver{
			lookup: func(_ context.Context, network, host string) ([]netip.Addr, error) {
				switch zoneOf(host) {
				case "ok.test":
					if network == "ip4" {
						return []netip.Addr{testExpectedA}, nil
					}
					return []netip.Addr{testExpectedAAAA}, nil
				case "a-hijacked.test":
					if network == "ip4" {
						return []netip.Addr{testHijackA}, nil
					}
					return []netip.Addr{testExpectedAAAA}, nil
				case "aaaa-hijacked.test":
					if network == "ip4" {
						return []netip.Addr{testExpectedA}, nil
					}
					return []netip.Addr{testHijackAAAA}, nil
				case "mixed.test":
					if network == "ip4" {
						return nil, timeoutErr
					}
					return []netip.Addr{testHijackAAAA}, nil
				case "missing.test":
					return nil, notFoundErr
				case "slow.test":
					return nil, timeoutErr
				case "borked.test":
					return nil, unexpectedErr
				}
				t.Fatalf("unexpected host %q", host)
				return nil, nil
			},
		},
	})

	report, err := d.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, diagv1.CheckReportStatus_CHECK_REPORT_STATUS_ISSUES_FOUND, report.GetStatus())

	byZone := map[string]*diagv1.DNSZoneResult{}
	for _, zr := range report.GetDnsReport().GetZoneResults() {
		byZone[zr.GetZone()] = zr
	}

	assert.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_OK, byZone["ok.test"].GetARecord().GetStatus())
	assert.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_OK, byZone["ok.test"].GetAaaaRecord().GetStatus())

	assert.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_HIJACKED, byZone["a-hijacked.test"].GetARecord().GetStatus())
	assert.Equal(t, testHijackA.String(), byZone["a-hijacked.test"].GetARecord().GetObservedIp())
	assert.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_OK, byZone["a-hijacked.test"].GetAaaaRecord().GetStatus())

	assert.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_OK, byZone["aaaa-hijacked.test"].GetARecord().GetStatus())
	assert.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_HIJACKED, byZone["aaaa-hijacked.test"].GetAaaaRecord().GetStatus())
	assert.Equal(t, testHijackAAAA.String(), byZone["aaaa-hijacked.test"].GetAaaaRecord().GetObservedIp())

	assert.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_TIMEOUT, byZone["mixed.test"].GetARecord().GetStatus())
	assert.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_HIJACKED, byZone["mixed.test"].GetAaaaRecord().GetStatus())
	assert.Equal(t, testHijackAAAA.String(), byZone["mixed.test"].GetAaaaRecord().GetObservedIp())

	assert.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_NOT_REGISTERED, byZone["missing.test"].GetARecord().GetStatus())
	assert.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_NOT_REGISTERED, byZone["missing.test"].GetAaaaRecord().GetStatus())

	assert.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_TIMEOUT, byZone["slow.test"].GetARecord().GetStatus())
	assert.NotEmpty(t, byZone["slow.test"].GetARecord().GetError())

	assert.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_RESOLVER_ERROR, byZone["borked.test"].GetARecord().GetStatus())
	assert.Contains(t, byZone["borked.test"].GetARecord().GetError(), "SERVFAIL")
}

func TestDNSDiagEmptyZonesOnlyReachability(t *testing.T) {
	d := newTestDNSDiag(t, DNSConfig{
		DirectQuerier: goodDirectQuerier,
		Resolver: fakeResolver{
			lookup: func(context.Context, string, string) ([]netip.Addr, error) {
				t.Fatal("resolver should not be called when there are no zones")
				return nil, nil
			},
		},
	})

	report, err := d.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, diagv1.CheckReportStatus_CHECK_REPORT_STATUS_OK, report.GetStatus())
	require.True(t, report.GetDnsReport().GetIpv4Reachability().GetReachable())
	require.True(t, report.GetDnsReport().GetIpv6Reachability().GetReachable())
	require.Empty(t, report.GetDnsReport().GetZoneResults())
}

func TestDNSDiagReachabilityNoAnswer(t *testing.T) {
	// VNet's DNS server responded but returned no records.
	d := newTestDNSDiag(t, DNSConfig{
		DNSZones:      []string{"company.test"},
		DirectQuerier: constantDirectQuerier(netip.Addr{}, netip.Addr{}),
		Resolver: fakeResolver{
			lookup: func(context.Context, string, string) ([]netip.Addr, error) {
				t.Fatal("per-zone check should be skipped when no expected IPs were captured")
				return nil, nil
			},
		},
	})

	report, err := d.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, diagv1.CheckReportStatus_CHECK_REPORT_STATUS_ISSUES_FOUND, report.GetStatus())
	require.False(t, report.GetDnsReport().GetIpv4Reachability().GetReachable())
	require.Contains(t, report.GetDnsReport().GetIpv4Reachability().GetError(), "server returned no records")
	require.False(t, report.GetDnsReport().GetIpv6Reachability().GetReachable())
	require.Contains(t, report.GetDnsReport().GetIpv6Reachability().GetError(), "server returned no records")
}

func TestDNSDiagCrossTransportFallback(t *testing.T) {
	d := newTestDNSDiag(t, DNSConfig{
		DNSZones: []string{"company.test"},
		DirectQuerier: fakeDirectQuerier{
			lookup: func(_ context.Context, network, _ string, server netip.AddrPort) ([]netip.Addr, error) {
				// IPv6 nameserver returns AAAA only, IPv4 nameserver returns both.
				if server == testVNetDNSv6 && network == "ip4" {
					return nil, nil
				}
				switch network {
				case "ip4":
					return []netip.Addr{testExpectedA}, nil
				case "ip6":
					return []netip.Addr{testExpectedAAAA}, nil
				}
				return nil, nil
			},
		},
		Resolver: fakeResolver{
			lookup: func(_ context.Context, network, _ string) ([]netip.Addr, error) {
				if network == "ip4" {
					return []netip.Addr{testExpectedA}, nil
				}
				return []netip.Addr{testExpectedAAAA}, nil
			},
		},
	})

	report, err := d.Run(t.Context())
	require.NoError(t, err)
	dnsReport := report.GetDnsReport()
	require.True(t, dnsReport.GetIpv6Reachability().GetRespondedAaaa())
	require.False(t, dnsReport.GetIpv6Reachability().GetRespondedA())
	require.True(t, dnsReport.GetIpv4Reachability().GetRespondedA())
	require.True(t, dnsReport.GetIpv4Reachability().GetRespondedAaaa())
	require.Len(t, dnsReport.GetZoneResults(), 1)
	zr := dnsReport.GetZoneResults()[0]
	require.NotNil(t, zr.GetARecord(), "A check must still run using A captured from the IPv4 nameserver")
	require.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_OK, zr.GetARecord().GetStatus())
	require.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_OK, zr.GetAaaaRecord().GetStatus())
}

func TestDNSDiagProbeFormat(t *testing.T) {
	a := probeName("company.test")
	b := probeName("company.test")
	require.NotEqual(t, a, b, "probe labels must be unique per call to defeat caches")
	require.True(t, strings.HasPrefix(a, "vnet-diag-"), a)
	require.True(t, strings.HasSuffix(a, ".company.test"), a)
	require.False(t, strings.HasSuffix(a, "."), "probe name must not be FQDN")
	require.Equal(t, "company.test", zoneOf(a))
}

func TestDNSDiagEmptyCheckReport(t *testing.T) {
	d, err := NewDNSDiag(&DNSConfig{VNetDNSIPv6: testVNetDNSv6})
	require.NoError(t, err)
	r := d.EmptyCheckReport()
	_, ok := r.Report.(*diagv1.CheckReport_DnsReport)
	require.True(t, ok, "EmptyCheckReport must return a DnsReport")
}

func TestNewDNSDiagRequiresAtLeastOneServer(t *testing.T) {
	_, err := NewDNSDiag(&DNSConfig{})
	require.Error(t, err)
}

func TestDNSDiagNotRegisteredRetryRecovers(t *testing.T) {
	notFoundErr := &net.DNSError{Name: "x", IsNotFound: true}

	type key struct{ zone, network string }
	var callsPerKey sync.Map
	d := newTestDNSDiag(t, DNSConfig{
		DNSZones:      []string{"company.test"},
		DirectQuerier: goodDirectQuerier,
		Resolver: fakeResolver{
			lookup: func(_ context.Context, network, host string) ([]netip.Addr, error) {
				k := key{zoneOf(host), network}
				n, _ := callsPerKey.LoadOrStore(k, 0)
				count := n.(int) + 1
				callsPerKey.Store(k, count)
				if count == 1 {
					return nil, notFoundErr
				}
				if network == "ip4" {
					return []netip.Addr{testExpectedA}, nil
				}
				return []netip.Addr{testExpectedAAAA}, nil
			},
		},
	})

	report, err := d.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, diagv1.CheckReportStatus_CHECK_REPORT_STATUS_OK, report.GetStatus(),
		"transient NOT_REGISTERED followed by OK on retry must result in overall OK")
	zr := report.GetDnsReport().GetZoneResults()[0]
	require.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_OK, zr.GetARecord().GetStatus())
	require.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_OK, zr.GetAaaaRecord().GetStatus())

	for _, network := range []string{"ip4", "ip6"} {
		calls, _ := callsPerKey.Load(key{"company.test", network})
		require.Equal(t, 2, calls, "resolver must be called twice for %s, once for the failed initial lookup and once for the retry", network)
	}
}

func TestDNSDiagNotRegisteredRetryPersistentFailure(t *testing.T) {
	notFoundErr := &net.DNSError{Name: "x", IsNotFound: true}

	var calls atomic.Int32
	d := newTestDNSDiag(t, DNSConfig{
		DNSZones:      []string{"company.test"},
		DirectQuerier: goodDirectQuerier,
		Resolver: fakeResolver{
			lookup: func(context.Context, string, string) ([]netip.Addr, error) {
				calls.Add(1)
				return nil, notFoundErr
			},
		},
	})

	report, err := d.Run(t.Context())
	require.NoError(t, err)
	zr := report.GetDnsReport().GetZoneResults()[0]
	require.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_NOT_REGISTERED, zr.GetARecord().GetStatus(),
		"persistent NOT_REGISTERED must still report NOT_REGISTERED after the retry")
	require.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_NOT_REGISTERED, zr.GetAaaaRecord().GetStatus())
	require.Equal(t, int32(4), calls.Load(), "resolver must be called exactly 4 times (2 record types × 2 attempts each)")
}

func TestDNSDiagReachabilityRetryRecoversFromStartupGap(t *testing.T) {
	var attempts atomic.Int32
	d := newTestDNSDiag(t, DNSConfig{
		VNetDNSIPv6: testVNetDNSv6,
		DNSZones:    []string{"company.test"},
		DirectQuerier: fakeDirectQuerier{
			lookup: func(_ context.Context, network, _ string, _ netip.AddrPort) ([]netip.Addr, error) {
				if network != "ip6" {
					return nil, nil
				}
				if attempts.Add(1) < 3 {
					return nil, &net.DNSError{Name: "x", IsTimeout: true, Err: "i/o timeout"}
				}
				return []netip.Addr{testExpectedAAAA}, nil
			},
		},
		Resolver: fakeResolver{
			lookup: func(context.Context, string, string) ([]netip.Addr, error) {
				return []netip.Addr{testExpectedAAAA}, nil
			},
		},
	})

	report, err := d.Run(t.Context())
	require.NoError(t, err)
	require.True(t, report.GetDnsReport().GetIpv6Reachability().GetReachable(),
		"reachability check must succeed on the third attempt once VNet's DNS comes up")
	require.Equal(t, int32(3), attempts.Load(), "reachability check must be attempted 3 times (2 failures + 1 success)")
}

func TestDNSDiagReachabilityRetryPersistentFailure(t *testing.T) {
	var attempts atomic.Int32
	d := newTestDNSDiag(t, DNSConfig{
		VNetDNSIPv6: testVNetDNSv6,
		DNSZones:    []string{"company.test"},
		DirectQuerier: fakeDirectQuerier{
			lookup: func(_ context.Context, network, _ string, _ netip.AddrPort) ([]netip.Addr, error) {
				if network == "ip6" {
					attempts.Add(1)
				}
				return nil, &net.DNSError{Name: "x", IsTimeout: true, Err: "i/o timeout"}
			},
		},
		Resolver: fakeResolver{
			lookup: func(context.Context, string, string) ([]netip.Addr, error) {
				t.Fatal("per-zone check must be skipped when reachability check fails after all retries")
				return nil, nil
			},
		},
	})

	report, err := d.Run(t.Context())
	require.NoError(t, err)
	dnsReport := report.GetDnsReport()
	require.False(t, dnsReport.GetIpv6Reachability().GetReachable())
	require.Contains(t, dnsReport.GetIpv6Reachability().GetError(), "i/o timeout")
	require.Equal(t, int32(defaultReachabilityMaxAttempts), attempts.Load(),
		"reachability check must be retried up to ReachabilityMaxAttempts before giving up")
}

func TestDNSDiagNotRegisteredRetryOnlyForNotRegistered(t *testing.T) {
	cases := []struct {
		name    string
		respond func() ([]netip.Addr, error)
	}{
		{"HIJACKED", func() ([]netip.Addr, error) {
			return []netip.Addr{testHijackAAAA}, nil
		}},
		{"TIMEOUT", func() ([]netip.Addr, error) {
			return nil, &net.DNSError{Name: "x", IsTimeout: true}
		}},
		{"RESOLVER_ERROR", func() ([]netip.Addr, error) {
			return nil, errors.New("upstream SERVFAIL")
		}},
		{"OK", func() ([]netip.Addr, error) {
			return []netip.Addr{testExpectedAAAA}, nil
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			d := newTestDNSDiag(t, DNSConfig{
				VNetDNSIPv6:   testVNetDNSv6,
				DNSZones:      []string{"company.test"},
				DirectQuerier: constantDirectQuerier(netip.Addr{}, testExpectedAAAA),
				Resolver: fakeResolver{
					lookup: func(context.Context, string, string) ([]netip.Addr, error) {
						calls.Add(1)
						return tc.respond()
					},
				},
			})
			_, err := d.Run(t.Context())
			require.NoError(t, err)
			require.Equal(t, int32(1), calls.Load(), "%s must not trigger a retry", tc.name)
		})
	}
}

func TestDNSServerForIPv6Prefix(t *testing.T) {
	cases := []struct {
		prefix  string
		want    string
		wantErr bool
	}{
		{prefix: "fdec:1fed:139f::", want: "[fdec:1fed:139f::2]:53"},
		{prefix: "fd00::", want: "[fd00::2]:53"},
		{prefix: "fdec:1fed:139f::5", want: "[fdec:1fed:139f::2]:53"},
		{prefix: "not an addr", wantErr: true},
		{prefix: "172.20.0.0", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.prefix, func(t *testing.T) {
			got, err := DNSServerForIPv6Prefix(tc.prefix)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got.String())
		})
	}
}

func TestDNSServerForIPv4CIDRRange(t *testing.T) {
	cases := []struct {
		cidr    string
		want    string
		wantErr bool
	}{
		{cidr: "100.64.0.0/24", want: "100.64.0.2:53"},
		{cidr: "172.16.0.0/16", want: "172.16.0.2:53"},
		{cidr: "10.0.0.0/8", want: "10.0.0.2:53"},
		{cidr: "not a cidr", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.cidr, func(t *testing.T) {
			got, err := DNSServerForIPv4CIDRRange(tc.cidr)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got.String())
		})
	}
}
