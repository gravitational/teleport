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

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"gvisor.dev/gvisor/pkg/tcpip"

	"github.com/gravitational/teleport/lib/vnet/dns"
)

func newNetworkStackConfig(ctx context.Context, tun TUNDevice, clt *clientApplicationServiceClient) (*networkStackConfig, error) {
	clock := clockwork.NewRealClock()
	sshProvider, err := newSSHProvider(ctx, sshProviderConfig{
		clt:   clt,
		clock: clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tcpHandlerResolver := newTCPHandlerResolver(&tcpHandlerResolverConfig{
		clt:         clt,
		appProvider: newAppProvider(clt),
		dbProvider:  newDBProvider(clt),
		sshProvider: sshProvider,
		clock:       clock,
		parentCtx:   ctx,
	})
	ipv6Prefix, err := newIPv6Prefix()
	if err != nil {
		return nil, trace.Wrap(err, "creating new IPv6 prefix")
	}
	ipv6Disabled := hostIPv6Disabled(ctx, tun)
	var dnsIPv6 tcpip.Address
	if !ipv6Disabled {
		dnsIPv6 = ipv6WithSuffix(ipv6Prefix, dns.DNSServerSuffix)
	}
	return &networkStackConfig{
		tunDevice:          tun,
		ipv6Prefix:         ipv6Prefix,
		ipv6Disabled:       ipv6Disabled,
		dnsIPv6:            dnsIPv6,
		tcpHandlerResolver: tcpHandlerResolver,
	}, nil
}

// hostIPv6Disabled reports whether IPv6 is disabled host-wide. It is
// best-effort: if the check fails, IPv6 is assumed to be enabled so that a
// real configuration error surfaces later instead of silently degrading.
func hostIPv6Disabled(ctx context.Context, tun TUNDevice) bool {
	tunName, err := tun.Name()
	if err == nil {
		var disabled bool
		if disabled, err = platformHostIPv6Disabled(tunName); err == nil {
			if disabled {
				log.WarnContext(ctx, "IPv6 is disabled on this host, VNet will skip IPv6 configuration and work over IPv4 only.")
			}
			return disabled
		}
	}
	log.WarnContext(ctx, "Failed to check whether IPv6 is disabled on the host, assuming it is enabled.", "error", err)
	return false
}
