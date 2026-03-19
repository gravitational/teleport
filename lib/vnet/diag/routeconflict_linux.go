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
	"net/netip"
	"os/exec"

	"github.com/gravitational/trace"
	"github.com/jsimonetti/rtnetlink/v2"
	"golang.org/x/sys/unix"
)

// LinuxRouting provides Linux-specific [Routing] implementation used by [RouteConflictDiag].
type LinuxRouting struct{}

// GetRouteDestinations gets routes from the OS and then extracts the only information needed from
// them: the route destination and the index of the network interface. It operates solely on IPv4
// routes.
//
// It might be called by [RouteConflictDiag] multiple times in case an interface was removed after
// the routes were fetched.
func (lr *LinuxRouting) GetRouteDestinations() ([]RouteDest, error) {
	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer conn.Close()

	routes, err := conn.Route.List()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rds := make([]RouteDest, 0, len(routes))
	for _, route := range routes {
		if route.Family != unix.AF_INET {
			continue
		}

		dst := route.Attributes.Dst
		ifaceIndex := int(route.Attributes.OutIface)

		// A nil destination means the default route (0.0.0.0/0).
		if dst == nil {
			addr := netip.IPv4Unspecified()
			rds = append(rds, &RouteDestPrefix{
				Prefix:     netip.PrefixFrom(addr, 0),
				ifaceIndex: ifaceIndex,
			})
			continue
		}

		addr, ok := netip.AddrFromSlice(dst)
		if !ok {
			continue
		}

		if addr.IsLinkLocalMulticast() {
			continue
		}

		prefixLen := int(route.DstLength)
		if prefixLen == 32 {
			rds = append(rds, &RouteDestIP{
				Addr:       addr,
				ifaceIndex: ifaceIndex,
			})
		} else {
			rds = append(rds, &RouteDestPrefix{
				Prefix:     netip.PrefixFrom(addr, prefixLen),
				ifaceIndex: ifaceIndex,
			})
		}
	}

	return rds, nil
}

func (n *NetInterfaces) interfaceApp(ctx context.Context, ifaceName string) (string, error) {
	// Linux TUN interfaces don't carry app metadata like macOS NetworkExtension.
	// Return the interface name, similar to the Windows approach.
	return ifaceName, nil
}

func (c *RouteConflictDiag) commands(ctx context.Context) []*exec.Cmd {
	return []*exec.Cmd{
		exec.CommandContext(ctx, "ip", "route", "show"),
		exec.CommandContext(ctx, "resolvectl", "status"),
	}
}
