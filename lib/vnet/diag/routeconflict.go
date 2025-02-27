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
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/gravitational/trace"

	diagv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/diag/v1"
)

// RouteConflictConfig includes everything that [RouteConflictDiag] needs to run.
type RouteConflictConfig struct {
	// VnetIfaceName is the name of the network interface set up by VNet. [RouteConflictDiag] needs it
	// to differentiate between routes created by VNet and routes set up by other software.
	VnetIfaceName string
	// Routing abstracts away platform-specific logic of obtaining routes with their destinations.
	Routing Routing
	// Interfaces abstracts away functions from the net package and calls to ifconfig.
	Interfaces Interfaces
	// RefetchRoutesDuration is the duration for which [RouteConflictDiag] is going to wait before
	// re-fetching the list of network routes on the system if it does not see any routes belonging to
	// the interface set up by VNet. It will fetch the routes up to three times. If after the third
	// time there's still no VNet routes, it'll just continue.
	RefetchRoutesDuration time.Duration
}

// Routing abstracts away platform-specific logic of obtaining routes with their destinations,
// allowing running tests for the general logic behind [RouteConflictDiag] on any platform.
type Routing interface {
	// GetRouteDestinations gets routes from the OS and then extracts the only information needed from
	// them: the route destination and the index of the network interface. It operates solely on IPv4
	// routes.
	//
	// It might be called by [RouteConflictDiag] multiple times in case an interface was removed after
	// the routes were fetched.
	GetRouteDestinations() ([]RouteDest, error)
}

// Interfaces abstracts away functions from the net package and calls to ifconfig, allowing mocking
// interactions with them in tests.
type Interfaces interface {
	// InterfaceByName is rarely used, as the only interface we fetch by name is VNet's interface.
	InterfaceByName(string) (*net.Interface, error)
	// InterfaceByIndex is called whenever [RouteConflictDiag] needs to get the name of an interface
	// for which a conflicting route was set up. [RouteDest] does not include the name of the
	// interface, only its index.
	InterfaceByIndex(int) (*net.Interface, error)
	// InterfaceApp attempts to return the name of the app that created the interface given the name
	// of the interface.
	//
	// InterfaceApp is expected to return [UnstableIfaceError] if the interface cannot be found.
	InterfaceApp(context.Context, string) (string, error)
}

// RouteConflictDiag is the diagnostic check which inspects if there are routes that conflict with
// routes set up by VNet.
type RouteConflictDiag struct {
	cfg *RouteConflictConfig
}

// NewRouteConflictDiag instantiates [RouteConflictDiag] given [RouteConflictConfig] and checks if
// the config has expected fields present.
func NewRouteConflictDiag(cfg *RouteConflictConfig) (*RouteConflictDiag, error) {
	if cfg.VnetIfaceName == "" {
		return nil, trace.BadParameter("missing VNet interface name")
	}

	if cfg.Routing == nil {
		return nil, trace.BadParameter("missing routing")
	}

	if cfg.Interfaces == nil {
		return nil, trace.BadParameter("missing net interfaces")
	}

	if cfg.RefetchRoutesDuration == 0 {
		cfg.RefetchRoutesDuration = 500 * time.Millisecond
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
func (c *RouteConflictDiag) Run(ctx context.Context) (*diagv1.CheckReport, error) {
	retries := 0
	for {
		rcs, err := c.run(ctx)
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

		status := diagv1.CheckReportStatus_CHECK_REPORT_STATUS_OK
		if len(rcs) > 0 {
			status = diagv1.CheckReportStatus_CHECK_REPORT_STATUS_ISSUES_FOUND
		}

		return &diagv1.CheckReport{
			Status: status,
			Report: &diagv1.CheckReport_RouteConflictReport{
				RouteConflictReport: &diagv1.RouteConflictReport{
					RouteConflicts: rcs,
				},
			},
		}, nil
	}
}

func (c *RouteConflictDiag) EmptyCheckReport() *diagv1.CheckReport {
	return &diagv1.CheckReport{
		Report: &diagv1.CheckReport_RouteConflictReport{},
	}
}

func (c *RouteConflictDiag) run(ctx context.Context) ([]*diagv1.RouteConflict, error) {
	// Unlike in other interactions with Interfaces, it doesn't make sense to re-fetch the routes,
	// hence why NewUnstableIfaceError is not used. If this call gives an error, then VnetIfaceName is
	// likely wrong.
	vnetIface, err := c.cfg.Interfaces.InterfaceByName(c.cfg.VnetIfaceName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If RouteConflictDiag runs soon after starting VNet or logging in to the first cluster, it might
	// take a few seconds for the VNet admin process to set up relevant network routes. In that
	// situation, RouteConflictDiag should wait for a brief period and then re-fetch routes.
	//
	// If the user does not have a valid cert for any cluster, VNet does not set up any routes. In
	// that niche case, RouteConflictDiag will sleep for 3 * c.cfg.RefetchRoutesDuration and return no
	// route conflicts.
	var rds []RouteDest
	var vnetDests []RouteDest
	attempts := 0
	for len(vnetDests) == 0 && attempts < 3 {
		if attempts > 0 {
			time.Sleep(c.cfg.RefetchRoutesDuration)
		}
		attempts++

		rds, err = c.cfg.Routing.GetRouteDestinations()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, rd := range rds {
			if rd.IfaceIndex() == vnetIface.Index {
				vnetDests = append(vnetDests, rd)
			}
		}
	}

	if len(vnetDests) == 0 {
		return nil, nil
	}

	var rcs []*diagv1.RouteConflict
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

			rcs = append(rcs, &diagv1.RouteConflict{
				Dest:          rd.String(),
				VnetDest:      vnetDest.String(),
				InterfaceName: iface.Name,
				InterfaceApp:  ifaceNetworkExtDesc,
			})
			break
		}
	}

	return rcs, nil
}

// Commands returns the accompanying command showing the state of routes in the system.
func (c *RouteConflictDiag) Commands(ctx context.Context) []*exec.Cmd {
	return c.commands(ctx)
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

// InterfaceApp attempts to return the name of the app that created the interface given the name of
// the interface.
//
// InterfaceApp is expected to return [UnstableIfaceError] if the interface cannot be found.
func (n *NetInterfaces) InterfaceApp(ctx context.Context, ifaceName string) (string, error) {
	appName, err := n.interfaceApp(ctx, ifaceName)
	return appName, trace.Wrap(err)
}

// extractNetworkExtDescFromIfconfigOutput is used by [getIfaceNetworkExtDesc] on macOS.
// The function is defined here so that we can run tests against it on other platforms too.
func extractNetworkExtDescFromIfconfigOutput(stdout []byte) string {
	scanner := bufio.NewScanner(bytes.NewReader(stdout))

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
