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
	"testing"

	"github.com/stretchr/testify/require"

	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

func TestRemoteOSConfigProvider(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		desc                 string
		tunName              string
		ipv6Prefix           string
		dnsAddr              string
		dnsZones             []string
		ipv4CIDRRanges       []string
		getTargetOSConfigErr error
		expectErr            error
		expectTargetOSConfig *osConfig
	}{
		{
			// No IPv4 address should be assigned until an IPv4 CIDR range is
			// reported.
			desc:       "no cidr ranges",
			tunName:    "testtun1",
			ipv6Prefix: "fd01:2345:6789::",
			dnsAddr:    "fd01:2345:6789::2",
			dnsZones:   []string{"test.example.com"},
			expectTargetOSConfig: &osConfig{
				tunName: "testtun1",
				// Should be the first non-broadcast address under the IPv6 prefix.
				tunIPv6:  "fd01:2345:6789::1",
				dnsAddr:  "fd01:2345:6789::2",
				dnsZones: []string{"test.example.com"},
			},
		},
		{
			desc:           "with cidr range",
			tunName:        "testtun1",
			ipv6Prefix:     "fd01:2345:6789::",
			dnsAddr:        "fd01:2345:6789::2",
			dnsZones:       []string{"test.example.com"},
			ipv4CIDRRanges: []string{"192.168.1.0/24"},
			expectTargetOSConfig: &osConfig{
				tunName: "testtun1",
				// Should be the first non-broadcast address in the CIDR range.
				tunIPv4:    "192.168.1.1",
				tunIPv6:    "fd01:2345:6789::1",
				dnsAddr:    "fd01:2345:6789::2",
				dnsZones:   []string{"test.example.com"},
				cidrRanges: []string{"192.168.1.0/24"},
			},
		},
		{
			desc:           "multiple cidr ranges",
			tunName:        "testtun1",
			ipv6Prefix:     "fd01:2345:6789::",
			dnsAddr:        "fd01:2345:6789::2",
			dnsZones:       []string{"test.example.com"},
			ipv4CIDRRanges: []string{"10.64.0.0/16", "192.168.1.0/24"},
			expectTargetOSConfig: &osConfig{
				tunName: "testtun1",
				// Should be chosen from the first CIDR range.
				tunIPv4:    "10.64.0.1",
				tunIPv6:    "fd01:2345:6789::1",
				dnsAddr:    "fd01:2345:6789::2",
				dnsZones:   []string{"test.example.com"},
				cidrRanges: []string{"10.64.0.0/16", "192.168.1.0/24"},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			targetOSConfigGetter := &fakeTargetOSConfigGetter{
				targetOSConfig: &vnetv1.TargetOSConfiguration{
					DnsZones:       tc.dnsZones,
					Ipv4CidrRanges: tc.ipv4CIDRRanges,
				},
				err: tc.getTargetOSConfigErr,
			}
			remoteOSConfigProvider, err := newRemoteOSConfigProvider(targetOSConfigGetter, tc.tunName, tc.ipv6Prefix, tc.dnsAddr)
			require.NoError(t, err)
			// Make sure remoteOSConfigProvider conforms to the targetOSConfigProvider
			// interface.
			osConfigProvider := targetOSConfigProvider(remoteOSConfigProvider)

			targetOSConfig, err := osConfigProvider.targetOSConfig(ctx)
			if tc.expectErr != nil {
				require.ErrorIs(t, err, tc.expectErr)
				return
			}
			require.Equal(t, tc.expectTargetOSConfig, targetOSConfig)
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
