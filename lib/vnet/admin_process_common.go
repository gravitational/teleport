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
)

func newNetworkStackConfig(ctx context.Context, tun tunDevice, clt *clientApplicationServiceClient) (*networkStackConfig, error) {
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
		sshProvider: sshProvider,
		clock:       clock,
	})
	ipv6Prefix, err := newIPv6Prefix()
	if err != nil {
		return nil, trace.Wrap(err, "creating new IPv6 prefix")
	}
	dnsIPv6 := ipv6WithSuffix(ipv6Prefix, []byte{2})
	return &networkStackConfig{
		tunDevice:          tun,
		ipv6Prefix:         ipv6Prefix,
		dnsIPv6:            dnsIPv6,
		tcpHandlerResolver: tcpHandlerResolver,
	}, nil
}
