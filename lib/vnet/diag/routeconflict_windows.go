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

package diag

import (
	"context"
	"net/netip"
	"os/exec"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
)

var ipv4Broadcast = netip.AddrFrom4([4]byte{255, 255, 255, 255})

// WindowsRouting provides Windows-specific [Routing] implementation used by [RouteConflictDiag].
type WindowsRouting struct{}

// GetRouteDestinations gets routes from the OS and then extracts the only
// information needed from them: the route destination and the index of the
// network interface. It operates solely on IPv4 routes.
func (wr *WindowsRouting) GetRouteDestinations() ([]RouteDest, error) {
	rows, err := winipcfg.GetIPForwardTable2(windows.AF_INET)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rds := make([]RouteDest, 0, len(rows))
	for _, row := range rows {
		prefix := row.DestinationPrefix.Prefix()
		addr := prefix.Addr()
		if addr.IsLinkLocalMulticast() || addr == ipv4Broadcast {
			// All interfaces seem to get a link local multicast and broadcast
			// route assigned which would always appear as a conflict, so skip
			// them.
			continue
		}
		if prefix.IsSingleIP() {
			rds = append(rds, &RouteDestIP{
				Addr:       addr,
				ifaceIndex: int(row.InterfaceIndex),
			})
		} else {
			rds = append(rds, &RouteDestPrefix{
				Prefix:     prefix,
				ifaceIndex: int(row.InterfaceIndex),
			})
		}
	}
	return rds, nil
}

func (n *NetInterfaces) interfaceApp(ctx context.Context, ifaceName string) (string, error) {
	// Interfaces usually have descriptive names on Windows (the TUN interfaces
	// used by VNet and Tailscale do, at least).
	return ifaceName, nil
}

func (c *RouteConflictDiag) commands(ctx context.Context) []*exec.Cmd {
	return []*exec.Cmd{
		exec.CommandContext(ctx, "netstat.exe", "-rn"),
		exec.CommandContext(ctx, "ipconfig.exe", "/all"),
		exec.CommandContext(ctx, "netsh.exe", "namespace", "show", "effectivepolicy"),
	}
}
