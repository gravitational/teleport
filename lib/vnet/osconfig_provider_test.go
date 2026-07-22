// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package vnet

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

func TestOSConfigProvider(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		desc                 string
		tunName              string
		ipv6Prefix           string
		dnsIPv6              string
		dnsZones             []string
		ipv4CIDRRanges       []string
		hostIPv6Disabled     bool
		getTargetOSConfigErr error
		expectErr            error
		expectTargetOSConfig *osConfig
		expectAddedDNSAddrs  []string
	}{
		{
			// No IPv4 address should be assigned until an IPv4 CIDR range is
			// reported.
			desc:       "no cidr ranges",
			tunName:    "testtun1",
			ipv6Prefix: "fd01:2345:6789::",
			dnsIPv6:    "fd01:2345:6789::2",
			dnsZones:   []string{"test.example.com"},
			expectTargetOSConfig: &osConfig{
				tunName: "testtun1",
				// Should be the first non-broadcast address under the IPv6 prefix.
				tunIPv6:  "fd01:2345:6789::1",
				dnsAddrs: []string{"fd01:2345:6789::2"},
				dnsZones: []string{"test.example.com"},
			},
		},
		{
			desc:           "with cidr range",
			tunName:        "testtun1",
			ipv6Prefix:     "fd01:2345:6789::",
			dnsIPv6:        "fd01:2345:6789::2",
			dnsZones:       []string{"test.example.com"},
			ipv4CIDRRanges: []string{"192.168.1.0/24"},
			expectTargetOSConfig: &osConfig{
				tunName: "testtun1",
				// Should be the first non-broadcast address in the CIDR range.
				tunIPv4:    "192.168.1.1",
				tunIPv4Net: &net.IPNet{IP: []byte{192, 168, 1, 0}, Mask: []byte{255, 255, 255, 0}},
				tunIPv6:    "fd01:2345:6789::1",
				// Should include the second non-broadcast address in the CIDR range.
				dnsAddrs:   []string{"fd01:2345:6789::2", "192.168.1.2"},
				dnsZones:   []string{"test.example.com"},
				cidrRanges: []string{"192.168.1.0/24"},
			},
			expectAddedDNSAddrs: []string{"192.168.1.2"},
		},
		{
			desc:           "multiple cidr ranges",
			tunName:        "testtun1",
			ipv6Prefix:     "fd01:2345:6789::",
			dnsIPv6:        "fd01:2345:6789::2",
			dnsZones:       []string{"test.example.com"},
			ipv4CIDRRanges: []string{"10.64.0.0/10", "192.168.1.0/24"},
			expectTargetOSConfig: &osConfig{
				tunName: "testtun1",
				// Should be chosen from the first CIDR range.
				tunIPv4:    "10.64.0.1",
				tunIPv4Net: &net.IPNet{IP: []byte{10, 64, 0, 0}, Mask: []byte{255, 192, 0, 0}},
				tunIPv6:    "fd01:2345:6789::1",
				dnsAddrs:   []string{"fd01:2345:6789::2", "10.64.0.2"},
				dnsZones:   []string{"test.example.com"},
				cidrRanges: []string{"10.64.0.0/10", "192.168.1.0/24"},
			},
			expectAddedDNSAddrs: []string{"10.64.0.2"},
		},
		{
			// With IPv6 disabled on the host and no logged-in clusters, the
			// target OS config should be empty
			desc:             "ipv6 disabled, no cidr ranges",
			tunName:          "testtun1",
			ipv6Prefix:       "fd01:2345:6789::",
			dnsIPv6:          "fd01:2345:6789::2",
			hostIPv6Disabled: true,
			expectTargetOSConfig: &osConfig{
				tunName: "testtun1",
			},
		},
		{
			// With IPv6 disabled on the host, only the IPv4 address and the
			// IPv4 nameserver should be configured.
			desc:             "ipv6 disabled, with cidr range",
			tunName:          "testtun1",
			ipv6Prefix:       "fd01:2345:6789::",
			dnsIPv6:          "fd01:2345:6789::2",
			dnsZones:         []string{"test.example.com"},
			ipv4CIDRRanges:   []string{"192.168.1.0/24"},
			hostIPv6Disabled: true,
			expectTargetOSConfig: &osConfig{
				tunName:    "testtun1",
				tunIPv4:    "192.168.1.1",
				tunIPv4Net: &net.IPNet{IP: []byte{192, 168, 1, 0}, Mask: []byte{255, 255, 255, 0}},
				dnsAddrs:   []string{"192.168.1.2"},
				dnsZones:   []string{"test.example.com"},
				cidrRanges: []string{"192.168.1.0/24"},
			},
			expectAddedDNSAddrs: []string{"192.168.1.2"},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			targetOSConfigGetter := &fakeTargetOSConfigGetter{
				targetOSConfig: vnetv1.TargetOSConfiguration_builder{
					DnsZones:       tc.dnsZones,
					Ipv4CidrRanges: tc.ipv4CIDRRanges,
				}.Build(),
				err: tc.getTargetOSConfigErr,
			}
			// Keep track of new DNS addresses the osConfigProvider tried to add.
			var addedDNSAddrs []string
			osConfigProvider, err := newOSConfigProvider(ctx, osConfigProviderConfig{
				clt:        targetOSConfigGetter,
				tunName:    tc.tunName,
				ipv6Prefix: tc.ipv6Prefix,
				dnsIPv6:    tc.dnsIPv6,
				addDNSAddress: func(ip net.IP) error {
					addedDNSAddrs = append(addedDNSAddrs, ip.String())
					return nil
				},
				checkHostIPv6Disabled: func(string) (bool, error) {
					return tc.hostIPv6Disabled, nil
				},
			})
			require.NoError(t, err)

			targetOSConfig, err := osConfigProvider.targetOSConfig(ctx)
			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
				return
			}
			require.Equal(t, tc.expectTargetOSConfig, targetOSConfig)

			require.ElementsMatch(t, tc.expectAddedDNSAddrs, addedDNSAddrs)
		})
	}
}

type fakeTargetOSConfigGetter struct {
	targetOSConfig *vnetv1.TargetOSConfiguration
	err            error
}

func (f *fakeTargetOSConfigGetter) GetTargetOSConfiguration(_ context.Context) (*vnetv1.TargetOSConfiguration, error) {
	return f.targetOSConfig, f.err
}
