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

package dns

import (
	"context"
	"log/slog"
	"net/netip"
	"sort"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
)

// platformLoadUpstreamNameservers attempts to find the default DNS nameservers
// that VNet should forward unmatched queries to. To do this, it finds the
// nameservers configured for each interface and sorts by the interface metric.
func platformLoadUpstreamNameservers(ctx context.Context) ([]string, error) {
	interfaces, err := winipcfg.GetIPInterfaceTable(windows.AF_INET)
	if err != nil {
		return nil, trace.Wrap(err, "looking up local network interfaces")
	}
	sort.Slice(interfaces, func(i, j int) bool {
		return interfaces[i].Metric < interfaces[j].Metric
	})
	var nameservers []string
	for _, iface := range interfaces {
		ifaceNameservers, err := iface.InterfaceLUID.DNS()
		if err != nil {
			return nil, trace.Wrap(err, "looking up DNS nameservers for interface")
		}
		for _, ifaceNameserver := range ifaceNameservers {
			if ignoreUpstreamNameserver(ifaceNameserver) {
				continue
			}
			nameservers = append(nameservers, withDNSPort(ifaceNameserver))
		}
	}
	slog.DebugContext(ctx, "Loaded host upstream nameservers", "nameservers", nameservers)
	return nameservers, nil
}

var (
	// Ignore site-local addresses, which Windows seems to have set on
	// most interfaces, implementing a deprecated IPv6 spec.
	siteLocalPrefix = netip.MustParsePrefix("fec0::/10")
)

func ignoreUpstreamNameserver(addr netip.Addr) bool {
	return siteLocalPrefix.Contains(addr)
}
