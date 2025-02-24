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
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os/exec"
	"strings"
	"syscall"

	"github.com/gravitational/trace"
	"golang.org/x/net/route"
)

// DarwinRouting provides macOS-specific [Routing] implementation used by [RouteConflictDiag].
type DarwinRouting struct{}

// GetRouteDestinations gets routes from the OS and then extracts the only information needed from
// them: the route destination and the index of the network interface. It operates solely on IPv4
// routes.
//
// It might be called by [RouteConflictDiag] multiple times in case an interface was removed after
// the routes were fetched.
func (dr *DarwinRouting) GetRouteDestinations() ([]RouteDest, error) {
	rib, err := route.FetchRIB(syscall.AF_INET, route.RIBTypeRoute, 0)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	messages, err := route.ParseRIB(route.RIBTypeRoute, rib)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rds := make([]RouteDest, 0, len(messages))

	for _, m := range messages {
		rm, ok := m.(*route.RouteMessage)
		if !ok {
			log.WarnContext(context.Background(), "Skipping message of unexpected type",
				"message_type", fmt.Sprintf("%T", m), "message", m)
			continue
		}

		rd, err := routeMessageToDest(rm)
		if err != nil {
			log.WarnContext(context.Background(), "Skipping route message as it couldn't be converted to RouteDest",
				"route_message", rm, "error", err)
			continue
		}

		rds = append(rds, rd)
	}

	return rds, nil
}

func routeMessageToDest(rm *route.RouteMessage) (RouteDest, error) {
	rawDest := rm.Addrs[syscall.RTAX_DST]
	destInet4Addr, ok := rawDest.(*route.Inet4Addr)
	if !ok {
		return nil, trace.Errorf("expected destination to be *route.Inet4Addr, got %T", rawDest)
	}

	var ip4Mask net.IPMask
	destNetipAddr := netip.AddrFrom4(destInet4Addr.IP)
	rawMask := rm.Addrs[syscall.RTAX_NETMASK]

	// The route destination has no netmask, return just the IP address.
	if rawMask == nil {
		return &RouteDestIP{Addr: destNetipAddr, ifaceIndex: rm.Index}, nil
	}

	mask, ok := rawMask.(*route.Inet4Addr)
	if !ok {
		return nil, trace.Errorf("expected netmask to be *route.Inet4Addr, got %T", rawMask)
	}
	ip4Mask = net.IPv4Mask(mask.IP[0], mask.IP[1], mask.IP[2], mask.IP[3])

	ones, _ := ip4Mask.Size()
	prefix := netip.PrefixFrom(destNetipAddr, ones)
	return &RouteDestPrefix{Prefix: prefix, ifaceIndex: rm.Index}, nil
}

// interfaceApp returns desc field of NetworkExtension that's included in the output of `ifconfig -v
// <ifaceName>` if the interface was created by an app that uses the NetworkExtension macOS
// framework. Returns an empty string if no description was found.
//
// According to a post from Apple Developer Forums from 2019, that's the only way this description
// can be extracted given an interface name.
// https://developer.apple.com/forums/thread/113491
func (n *NetInterfaces) interfaceApp(ctx context.Context, ifaceName string) (string, error) {
	cmd := exec.CommandContext(ctx, "ifconfig", "-v", ifaceName)
	stdout, err := cmd.Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			stderr := string(exitError.Stderr)
			log.ErrorContext(ctx, "Failed to get interface details", "stderr", stderr)

			if strings.Contains(stderr, "does not exist") {
				return "", trace.Wrap(NewUnstableIfaceError(exitError))
			}
		}
		return "", trace.Wrap(err)
	}

	return extractNetworkExtDescFromIfconfigOutput(stdout), nil
}

func (c *RouteConflictDiag) commands(ctx context.Context) []*exec.Cmd {
	return []*exec.Cmd{
		exec.CommandContext(ctx, "netstat", "-rn", "-f", "inet"),
	}
}
