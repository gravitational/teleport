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
	lookup func(ctx context.Context, host string) ([]netip.Addr, error)
}

func (f fakeResolver) Lookup(ctx context.Context, host string) ([]netip.Addr, error) {
	return f.lookup(ctx, host)
}

type fakeDirectQuerier struct {
	lookup func(ctx context.Context, host string, server netip.AddrPort) ([]netip.Addr, error)
}

func (f fakeDirectQuerier) LookupDirect(ctx context.Context, host string, server netip.AddrPort) ([]netip.Addr, error) {
	return f.lookup(ctx, host, server)
}

var (
	testVNetDNS    = netip.MustParseAddrPort("[fdec:1fed:139f::2]:53")
	testExpectedIP = netip.MustParseAddr("fdec:1fed:139f::2")
	testOtherIP    = netip.MustParseAddr("2001:db8::1234")
	testIPv4Hijack = netip.MustParseAddr("192.0.2.42")
)

func newTestDNSDiag(t *testing.T, cfg DNSConfig) *DNSDiag {
	t.Helper()
	if !cfg.VNetDNSServer.IsValid() {
		cfg.VNetDNSServer = testVNetDNS
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

func TestDNSDiagReachabilityFailureSkipsPerZone(t *testing.T) {
	resolverCalled := false
	d := newTestDNSDiag(t, DNSConfig{
		DNSZones: []string{"company.test"},
		DirectQuerier: fakeDirectQuerier{
			lookup: func(context.Context, string, netip.AddrPort) ([]netip.Addr, error) {
				return nil, errors.New("connection refused")
			},
		},
		Resolver: fakeResolver{
			lookup: func(context.Context, string) ([]netip.Addr, error) {
				resolverCalled = true
				return []netip.Addr{testExpectedIP}, nil
			},
		},
	})

	report, err := d.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, diagv1.CheckReportStatus_CHECK_REPORT_STATUS_ISSUES_FOUND, report.Status)
	dnsReport := report.GetDnsReport()
	require.False(t, dnsReport.VnetDnsReachable)
	require.Contains(t, dnsReport.VnetDnsUnreachableError, "connection refused")
	require.Empty(t, dnsReport.ZoneResults, "per-zone check must be skipped on reachability check failure")
	require.False(t, resolverCalled, "OS resolver must not be called when VNet DNS is unreachable")
}

func TestDNSDiagAllZonesOK(t *testing.T) {
	d := newTestDNSDiag(t, DNSConfig{
		DNSZones: []string{"company.test", "leaf.mirrors.link"},
		DirectQuerier: fakeDirectQuerier{
			lookup: func(context.Context, string, netip.AddrPort) ([]netip.Addr, error) {
				return []netip.Addr{testExpectedIP}, nil
			},
		},
		Resolver: fakeResolver{
			lookup: func(_ context.Context, host string) ([]netip.Addr, error) {
				return []netip.Addr{testExpectedIP}, nil
			},
		},
	})

	report, err := d.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, diagv1.CheckReportStatus_CHECK_REPORT_STATUS_OK, report.Status)
	dnsReport := report.GetDnsReport()
	require.True(t, dnsReport.VnetDnsReachable)
	require.Len(t, dnsReport.ZoneResults, 2)
	for _, zr := range dnsReport.ZoneResults {
		require.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_OK, zr.Status, zr.Zone)
		require.Equal(t, testExpectedIP.String(), zr.ObservedIp)
		require.Equal(t, testExpectedIP.String(), zr.ExpectedIp)
	}
}

func TestDNSDiagPerZoneClassification(t *testing.T) {
	notFoundErr := &net.DNSError{Name: "x", IsNotFound: true}
	timeoutErr := &net.DNSError{Name: "x", IsTimeout: true, Err: "i/o timeout"}
	unexpectedErr := errors.New("upstream SERVFAIL")

	d := newTestDNSDiag(t, DNSConfig{
		DNSZones: []string{
			"ok.test", "hijacked.test", "missing.test", "slow.test", "borked.test",
			"hijacked-ipv4-only.test", "hijacked-extra-addr.test", "hijacked-mixed.test",
		},
		DirectQuerier: fakeDirectQuerier{
			lookup: func(context.Context, string, netip.AddrPort) ([]netip.Addr, error) {
				return []netip.Addr{testExpectedIP}, nil
			},
		},
		Resolver: fakeResolver{
			lookup: func(_ context.Context, host string) ([]netip.Addr, error) {
				switch zoneOf(host) {
				case "ok.test":
					return []netip.Addr{testExpectedIP}, nil
				case "hijacked.test":
					return []netip.Addr{testOtherIP}, nil
				case "hijacked-ipv4-only.test":
					// VNet answers AAAA only, so any IPv4 came from a non-VNet resolver.
					return []netip.Addr{testIPv4Hijack}, nil
				case "hijacked-extra-addr.test":
					// VNet returns exactly one address; an extra one means another resolver also answered.
					return []netip.Addr{testExpectedIP, testOtherIP}, nil
				case "hijacked-mixed.test":
					// Exercises the IPv6-preferred surfacing path in the HIJACKED branch.
					return []netip.Addr{testIPv4Hijack, testOtherIP}, nil
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
	require.Equal(t, diagv1.CheckReportStatus_CHECK_REPORT_STATUS_ISSUES_FOUND, report.Status)

	byZone := map[string]*diagv1.DNSZoneResult{}
	for _, zr := range report.GetDnsReport().ZoneResults {
		byZone[zr.Zone] = zr
	}

	assert.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_OK, byZone["ok.test"].Status)
	assert.Equal(t, testExpectedIP.String(), byZone["ok.test"].ObservedIp)

	assert.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_HIJACKED, byZone["hijacked.test"].Status)
	assert.Equal(t, testOtherIP.String(), byZone["hijacked.test"].ObservedIp)

	assert.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_HIJACKED, byZone["hijacked-ipv4-only.test"].Status)
	assert.Equal(t, testIPv4Hijack.String(), byZone["hijacked-ipv4-only.test"].ObservedIp,
		"IPv4-only hijack must surface the IPv4 as observed evidence")

	assert.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_HIJACKED, byZone["hijacked-extra-addr.test"].Status,
		"probe present but with extra addresses must classify as HIJACKED — VNet only ever returns one")

	assert.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_HIJACKED, byZone["hijacked-mixed.test"].Status)
	assert.Equal(t, testOtherIP.String(), byZone["hijacked-mixed.test"].ObservedIp,
		"with both IPv4 and IPv6 in a HIJACKED response, the IPv6 must be surfaced as it is the more direct evidence")

	assert.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_NOT_REGISTERED, byZone["missing.test"].Status)
	assert.Empty(t, byZone["missing.test"].ObservedIp)

	assert.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_TIMEOUT, byZone["slow.test"].Status)
	assert.NotEmpty(t, byZone["slow.test"].Error)

	assert.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_RESOLVER_ERROR, byZone["borked.test"].Status)
	assert.Contains(t, byZone["borked.test"].Error, "SERVFAIL")
}

func TestDNSDiagEmptyZonesOnlyReachability(t *testing.T) {
	d := newTestDNSDiag(t, DNSConfig{
		DirectQuerier: fakeDirectQuerier{
			lookup: func(context.Context, string, netip.AddrPort) ([]netip.Addr, error) {
				return []netip.Addr{testExpectedIP}, nil
			},
		},
		Resolver: fakeResolver{
			lookup: func(context.Context, string) ([]netip.Addr, error) {
				t.Fatal("resolver should not be called when there are no zones")
				return nil, nil
			},
		},
	})

	report, err := d.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, diagv1.CheckReportStatus_CHECK_REPORT_STATUS_OK, report.Status)
	require.True(t, report.GetDnsReport().VnetDnsReachable)
	require.Empty(t, report.GetDnsReport().ZoneResults)
}

func TestDNSDiagReachabilityNoAnswer(t *testing.T) {
	// VNet's DNS server responded but returned no records treated as
	// unreachable, since per-zone has nothing to compare against.
	d := newTestDNSDiag(t, DNSConfig{
		DNSZones: []string{"company.test"},
		DirectQuerier: fakeDirectQuerier{
			lookup: func(context.Context, string, netip.AddrPort) ([]netip.Addr, error) {
				return nil, nil
			},
		},
		Resolver: fakeResolver{
			lookup: func(context.Context, string) ([]netip.Addr, error) {
				t.Fatal("per-zone check should be skipped when reachability check returns no answer")
				return nil, nil
			},
		},
	})

	report, err := d.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, diagv1.CheckReportStatus_CHECK_REPORT_STATUS_ISSUES_FOUND, report.Status)
	require.False(t, report.GetDnsReport().VnetDnsReachable)
}

func TestDNSDiagProbeFormat(t *testing.T) {
	a := probeFQDN("company.test")
	b := probeFQDN("company.test")
	require.NotEqual(t, a, b, "probe labels must be unique per call to defeat caches")
	require.True(t, strings.HasPrefix(a, "vnet-diag-"), a)
	require.True(t, strings.HasSuffix(a, ".company.test."), a)
	require.Equal(t, "company.test", zoneOf(a))
}

func TestDNSDiagEmptyCheckReport(t *testing.T) {
	d, err := NewDNSDiag(&DNSConfig{VNetDNSServer: testVNetDNS})
	require.NoError(t, err)
	r := d.EmptyCheckReport()
	_, ok := r.Report.(*diagv1.CheckReport_DnsReport)
	require.True(t, ok, "EmptyCheckReport must return a DnsReport")
}

func TestNewDNSDiagRequiresVNetDNSServer(t *testing.T) {
	_, err := NewDNSDiag(&DNSConfig{})
	require.Error(t, err)
}

func TestDNSDiagNotRegisteredRetryRecovers(t *testing.T) {
	notFoundErr := &net.DNSError{Name: "x", IsNotFound: true}

	var callsPerZone sync.Map
	d := newTestDNSDiag(t, DNSConfig{
		DNSZones: []string{"company.test"},
		DirectQuerier: fakeDirectQuerier{
			lookup: func(context.Context, string, netip.AddrPort) ([]netip.Addr, error) {
				return []netip.Addr{testExpectedIP}, nil
			},
		},
		Resolver: fakeResolver{
			lookup: func(_ context.Context, host string) ([]netip.Addr, error) {
				zone := zoneOf(host)
				n, _ := callsPerZone.LoadOrStore(zone, 0)
				count := n.(int) + 1
				callsPerZone.Store(zone, count)
				if count == 1 {
					return nil, notFoundErr
				}
				return []netip.Addr{testExpectedIP}, nil
			},
		},
	})

	report, err := d.Run(t.Context())
	require.NoError(t, err)
	require.Equal(t, diagv1.CheckReportStatus_CHECK_REPORT_STATUS_OK, report.Status,
		"transient NOT_REGISTERED followed by OK on retry must result in overall OK")
	zr := report.GetDnsReport().ZoneResults[0]
	require.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_OK, zr.Status)

	calls, _ := callsPerZone.Load("company.test")
	require.Equal(t, 2, calls, "resolver must be called twice — once for the failed initial lookup and once for the retry")
}

func TestDNSDiagNotRegisteredRetryPersistentFailure(t *testing.T) {
	notFoundErr := &net.DNSError{Name: "x", IsNotFound: true}

	var calls atomic.Int32
	d := newTestDNSDiag(t, DNSConfig{
		DNSZones: []string{"company.test"},
		DirectQuerier: fakeDirectQuerier{
			lookup: func(context.Context, string, netip.AddrPort) ([]netip.Addr, error) {
				return []netip.Addr{testExpectedIP}, nil
			},
		},
		Resolver: fakeResolver{
			lookup: func(context.Context, string) ([]netip.Addr, error) {
				calls.Add(1)
				return nil, notFoundErr
			},
		},
	})

	report, err := d.Run(t.Context())
	require.NoError(t, err)
	zr := report.GetDnsReport().ZoneResults[0]
	require.Equal(t, diagv1.DNSZoneStatus_DNS_ZONE_STATUS_NOT_REGISTERED, zr.Status,
		"persistent NOT_REGISTERED must still report NOT_REGISTERED after the retry")
	require.Equal(t, int32(2), calls.Load(), "resolver must be called exactly twice (initial + one retry)")
}

func TestDNSDiagReachabilityRetryRecoversFromStartupGap(t *testing.T) {
	var reachabilityCalls atomic.Int32
	d := newTestDNSDiag(t, DNSConfig{
		DNSZones: []string{"company.test"},
		DirectQuerier: fakeDirectQuerier{
			lookup: func(context.Context, string, netip.AddrPort) ([]netip.Addr, error) {
				if reachabilityCalls.Add(1) < 3 {
					return nil, &net.DNSError{Name: "x", IsTimeout: true, Err: "i/o timeout"}
				}
				return []netip.Addr{testExpectedIP}, nil
			},
		},
		Resolver: fakeResolver{
			lookup: func(context.Context, string) ([]netip.Addr, error) {
				return []netip.Addr{testExpectedIP}, nil
			},
		},
	})

	report, err := d.Run(t.Context())
	require.NoError(t, err)
	require.True(t, report.GetDnsReport().VnetDnsReachable,
		"reachability check must succeed on the third attempt once VNet's DNS comes up")
	require.Equal(t, int32(3), reachabilityCalls.Load(), "reachability check must be called 3 times (2 failures + 1 success)")
}

func TestDNSDiagReachabilityRetryPersistentFailure(t *testing.T) {
	var reachabilityCalls atomic.Int32
	d := newTestDNSDiag(t, DNSConfig{
		DNSZones: []string{"company.test"},
		DirectQuerier: fakeDirectQuerier{
			lookup: func(context.Context, string, netip.AddrPort) ([]netip.Addr, error) {
				reachabilityCalls.Add(1)
				return nil, &net.DNSError{Name: "x", IsTimeout: true, Err: "i/o timeout"}
			},
		},
		Resolver: fakeResolver{
			lookup: func(context.Context, string) ([]netip.Addr, error) {
				t.Fatal("per-zone check must be skipped when reachability check fails after all retries")
				return nil, nil
			},
		},
	})

	report, err := d.Run(t.Context())
	require.NoError(t, err)
	dnsReport := report.GetDnsReport()
	require.False(t, dnsReport.VnetDnsReachable)
	require.Contains(t, dnsReport.VnetDnsUnreachableError, "i/o timeout")
	require.Equal(t, int32(defaultReachabilityMaxAttempts), reachabilityCalls.Load(),
		"reachability check must be retried up to ReachabilityMaxAttempts before giving up")
}

func TestDNSDiagNotRegisteredRetryOnlyForNotRegistered(t *testing.T) {
	cases := []struct {
		name    string
		respond func() ([]netip.Addr, error)
	}{
		{"HIJACKED", func() ([]netip.Addr, error) {
			return []netip.Addr{testOtherIP}, nil
		}},
		{"TIMEOUT", func() ([]netip.Addr, error) {
			return nil, &net.DNSError{Name: "x", IsTimeout: true}
		}},
		{"RESOLVER_ERROR", func() ([]netip.Addr, error) {
			return nil, errors.New("upstream SERVFAIL")
		}},
		{"OK", func() ([]netip.Addr, error) {
			return []netip.Addr{testExpectedIP}, nil
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			d := newTestDNSDiag(t, DNSConfig{
				DNSZones: []string{"company.test"},
				DirectQuerier: fakeDirectQuerier{
					lookup: func(context.Context, string, netip.AddrPort) ([]netip.Addr, error) {
						return []netip.Addr{testExpectedIP}, nil
					},
				},
				Resolver: fakeResolver{
					lookup: func(context.Context, string) ([]netip.Addr, error) {
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
