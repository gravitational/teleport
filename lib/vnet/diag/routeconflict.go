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
	"bufio"
	"bytes"
	"context"
	"errors"
	"net"
	"net/netip"
	"regexp"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, "vnet:diag")

type RouteConflictConfig struct {
	VnetIfaceName string
	Routing       Routing
	Interfaces    Interfaces
}

// Routing abstracts away platform-specific logic of obtaining routes with their destinations,
// allowing running tests for the general logic behind RouteConflictDiag on any platform.
type Routing interface {
	GetRouteDestinations() ([]RouteDest, error)
}

// Interfaces abstracts away functions from the net package and calls to ifconfig, allowing mocking
// interactions with them in tests.
type Interfaces interface {
	InterfaceByName(string) (*net.Interface, error)
	InterfaceByIndex(int) (*net.Interface, error)
	// InterfaceApp attempts to return the name of the app that created the interface given the name
	// of the interface.
	InterfaceApp(context.Context, string) (string, error)
}

func (c *RouteConflictConfig) CheckAndSetDefaults() error {
	if c.VnetIfaceName == "" {
		return trace.BadParameter("missing VNet interface name")
	}

	if c.Routing == nil {
		return trace.BadParameter("missing routing")
	}

	if c.Interfaces == nil {
		return trace.BadParameter("missing net interfaces")
	}

	return nil
}

type RouteConflictDiag struct {
	cfg *RouteConflictConfig
}

func NewRouteConflictDiag(cfg *RouteConflictConfig) (*RouteConflictDiag, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &RouteConflictDiag{
		cfg: cfg,
	}, nil
}

// Run scans Ipv4 routing table (equivalent of running `netstat -rn -f inet`) in search of routes
// that overlap with routes set up by VNet.
//
// If a 3rd-party route conflicts with more than one VNet route, Run returns a single RouteConflict
// for that 3rd-party route describing the conflict with the first conflicting VNet route.
func (c *RouteConflictDiag) Run(ctx context.Context) ([]RouteConflict, error) {
	retries := 0
	for {
		crs, err := c.run(ctx)
		if err != nil {
			// UnstableIfaceError usually means that an interface was removed between fetching route
			// messages and getting the details of the interface. In this case, the routes for that
			// interface are likely gone too, so the best course of action is to repeat the whole check.
			if errors.As(err, new(UnstableIfaceError)) && retries < 2 {
				log.DebugContext(ctx, "Repeating check", "error", err)
				retries++
				continue
			}
			return nil, trace.Wrap(err)
		}
		return crs, nil
	}
}

func (c *RouteConflictDiag) run(ctx context.Context) ([]RouteConflict, error) {
	// Unlike in other interactions with Interfaces, it doesn't make sense to re-fetch the routes,
	// hence why NewUnstableIfaceError is not used. If this call gives an error, then VnetIfaceName is
	// likely wrong.
	vnetIface, err := c.cfg.Interfaces.InterfaceByName(c.cfg.VnetIfaceName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rds, err := c.cfg.Routing.GetRouteDestinations()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var vnetDests []RouteDest
	for _, rd := range rds {
		if rd.IfaceIndex() == vnetIface.Index {
			vnetDests = append(vnetDests, rd)
		}
	}

	var crs []RouteConflict
	for _, rd := range rds {
		if rd.IfaceIndex() == vnetIface.Index {
			continue
		}

		// VNet doesn't set up any routes for the default destination, which means that VNet routes
		// always have priority over routes for the default destination.
		if rd.IsDefault() {
			continue
		}

		for _, vnetDest := range vnetDests {
			if !rd.Overlaps(vnetDest) {
				continue
			}

			iface, err := c.cfg.Interfaces.InterfaceByIndex(rd.IfaceIndex())
			if err != nil {
				return nil, trace.Wrap(NewUnstableIfaceError(err))
			}
			ifaceNetworkExtDesc, err := c.cfg.Interfaces.InterfaceApp(ctx, iface.Name)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			crs = append(crs, RouteConflict{
				Dest:      rd,
				VnetDest:  vnetDest,
				IfaceName: iface.Name,
				IfaceApp:  ifaceNetworkExtDesc,
			})
			break
		}
	}

	return crs, nil
}

// RouteConflict describes a conflict between a route set up by a 3rd-party app where the
// destination overlaps with a destination in a route set up by VNet.
type RouteConflict struct {
	// Dest is the destination of the conflicting route.
	Dest RouteDest
	// VnetDest is the destination of a VNet route that Dest overlaps with.
	VnetDest RouteDest
	// IfaceName is the name of the interface the route uses, e.g. "utun4".
	IfaceName string
	// IfaceApp may contain the name of the application responsible for setting up the interface.
	// At the moment, the only source of this information is NetworkExtension description included in
	// the output of `ifconfig -v <interface name>`. Not all VPN applications use this framework, so
	// it's likely to be empty.
	IfaceApp string
}

// RouteDest allows singular treatment of route destinations, no matter if they have a netmask or not.
type RouteDest interface {
	IfaceIndex() int
	String() string
	ToPrefix() netip.Prefix
	Overlaps(RouteDest) bool
	// IsDefault returns true if the route dest is either 0.0.0.0 or 0.0.0.0/0.
	IsDefault() bool
}

// RouteDestIP is [RouteDest] implementation for [netip.Addr]. Assumes that Addr is IPv4.
type RouteDestIP struct {
	netip.Addr
	ifaceIndex int
}

func (r *RouteDestIP) ToPrefix() netip.Prefix {
	return netip.PrefixFrom(r.Addr, 32)
}

func (r *RouteDestIP) Overlaps(other RouteDest) bool {
	return r.ToPrefix().Overlaps(other.ToPrefix())
}

func (r *RouteDestIP) IsDefault() bool {
	return r.IsUnspecified()
}

func (r *RouteDestIP) IfaceIndex() int {
	return r.ifaceIndex
}

// RouteDestPrefix is [RouteDest] implementation for [netip.Prefix].
type RouteDestPrefix struct {
	netip.Prefix
	ifaceIndex int
}

func (r *RouteDestPrefix) ToPrefix() netip.Prefix {
	return r.Prefix
}

func (r *RouteDestPrefix) Overlaps(other RouteDest) bool {
	return r.Prefix.Overlaps(other.ToPrefix())
}

func (r *RouteDestPrefix) IsDefault() bool {
	return r.Addr().IsUnspecified() && r.Bits() == 0
}

func (r *RouteDestPrefix) IfaceIndex() int {
	return r.ifaceIndex
}

// UnstableIfaceError is used in a situation where an interface couldn't be fetched by name or
// index. RouteConflictDiag is going to re-fetch the routes upon encountering this error, up to a
// few times.
type UnstableIfaceError struct {
	err error
}

func NewUnstableIfaceError(err error) UnstableIfaceError {
	return UnstableIfaceError{err: err}
}

func (i UnstableIfaceError) Error() string {
	return i.err.Error()
}

func (i UnstableIfaceError) Unwrap() error {
	return i.err
}

// NetInterfaces implements [Interfaces] by using functions from the net package.
type NetInterfaces struct{}

func (n *NetInterfaces) InterfaceByName(name string) (*net.Interface, error) {
	iface, err := net.InterfaceByName(name)
	return iface, trace.Wrap(err)
}

func (n *NetInterfaces) InterfaceByIndex(index int) (*net.Interface, error) {
	iface, err := net.InterfaceByIndex(index)
	return iface, trace.Wrap(err)
}

// extractNetworkExtDescFromIfconfigOutput is used by [getIfaceNetworkExtDesc] on macOS.
// The function is defined here so that we can run tests against it on other platforms too.
func extractNetworkExtDescFromIfconfigOutput(stdout []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(stdout))
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.Contains(line, "domain:NetworkExtension") {
			continue
		}

		if matches := networkExtDescRe.FindStringSubmatch(line); len(matches) >= 2 {
			return matches[1]
		}
		return ""
	}

	return ""
}

// networkExtDescRe matches the string between double quotes in the desc field, e.g.:
//
// agent domain:NetworkExtension type:VPN flags:0xf desc:"VPN: foobar"
//
// should match `VPN: foobar`.
var networkExtDescRe = regexp.MustCompile(`desc:"([^"]+)"`)
