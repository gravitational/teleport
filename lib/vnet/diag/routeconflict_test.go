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
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	diagv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/diag/v1"
)

const vnetIface = "utun4"
const quuxIface = "utun5"
const vnetIfaceIndex = 1
const quuxIfaceIndex = 2

func TestRouteConflictDiag(t *testing.T) {
	singleRouteConflict := func(t *testing.T, dests []RouteDest, rcs []*diagv1.RouteConflict) {
		require.Len(t, rcs, 1)
		rc := rcs[0]
		require.Equal(t, dests[0].String(), rc.Dest)
		require.Equal(t, dests[1].String(), rc.VnetDest)
		require.Equal(t, quuxIface, rc.InterfaceName)
		require.Equal(t, "foobar", rc.InterfaceApp)
	}

	tests := map[string]struct {
		dests       []RouteDest
		checkResult func(t *testing.T, dests []RouteDest, rcs []*diagv1.RouteConflict)
	}{
		"single IP vs VNet single IP": {
			dests: []RouteDest{
				&RouteDestIP{ifaceIndex: quuxIfaceIndex, Addr: netip.AddrFrom4([4]byte{1, 2, 3, 4})},
				&RouteDestIP{ifaceIndex: vnetIfaceIndex, Addr: netip.AddrFrom4([4]byte{1, 2, 3, 4})},
			},
			checkResult: singleRouteConflict,
		},
		"single IP vs VNet prefix": {
			dests: []RouteDest{
				&RouteDestIP{ifaceIndex: quuxIfaceIndex, Addr: netip.AddrFrom4([4]byte{1, 2, 3, 4})},
				&RouteDestPrefix{ifaceIndex: vnetIfaceIndex, Prefix: netip.MustParsePrefix("1.0.0.0/8")},
			},
			checkResult: singleRouteConflict,
		},
		"prefix vs VNet single IP": {
			dests: []RouteDest{
				&RouteDestPrefix{ifaceIndex: quuxIfaceIndex, Prefix: netip.MustParsePrefix("1.0.0.0/8")},
				&RouteDestIP{ifaceIndex: vnetIfaceIndex, Addr: netip.AddrFrom4([4]byte{1, 2, 3, 4})},
			},
			checkResult: singleRouteConflict,
		},
		"prefix vs VNet prefix": {
			dests: []RouteDest{
				&RouteDestPrefix{ifaceIndex: quuxIfaceIndex, Prefix: netip.MustParsePrefix("1.0.0.0/8")},
				&RouteDestPrefix{ifaceIndex: vnetIfaceIndex, Prefix: netip.MustParsePrefix("0.0.0.0/1")},
			},
			checkResult: singleRouteConflict,
		},
		"two VNet routes, single result": {
			dests: []RouteDest{
				&RouteDestIP{ifaceIndex: quuxIfaceIndex, Addr: netip.AddrFrom4([4]byte{1, 2, 3, 4})},
				&RouteDestPrefix{ifaceIndex: vnetIfaceIndex, Prefix: netip.MustParsePrefix("0.0.0.0/1")},
				&RouteDestIP{ifaceIndex: vnetIfaceIndex, Addr: netip.AddrFrom4([4]byte{1, 2, 3, 4})},
			},
			checkResult: singleRouteConflict,
		},
		"one result for each conflicting 3rd-party route": {
			dests: []RouteDest{
				&RouteDestIP{ifaceIndex: quuxIfaceIndex, Addr: netip.AddrFrom4([4]byte{1, 2, 3, 4})},
				&RouteDestPrefix{ifaceIndex: quuxIfaceIndex, Prefix: netip.MustParsePrefix("1.2.3.0/24")},
				&RouteDestPrefix{ifaceIndex: vnetIfaceIndex, Prefix: netip.MustParsePrefix("0.0.0.0/1")},
			},
			checkResult: func(t *testing.T, dests []RouteDest, rcs []*diagv1.RouteConflict) {
				require.Len(t, rcs, 2)

				rc1 := rcs[0]
				require.Equal(t, "1.2.3.4", rc1.Dest)
				require.Equal(t, "0.0.0.0/1", rc1.VnetDest)
				require.Equal(t, quuxIface, rc1.InterfaceName)
				require.Equal(t, "foobar", rc1.InterfaceApp)

				rc2 := rcs[1]
				require.Equal(t, "1.2.3.0/24", rc2.Dest)
				require.Equal(t, "0.0.0.0/1", rc2.VnetDest)
				require.Equal(t, quuxIface, rc2.InterfaceName)
				require.Equal(t, "foobar", rc2.InterfaceApp)
			},
		},
		"default dests are ignored": {
			dests: []RouteDest{
				&RouteDestIP{ifaceIndex: quuxIfaceIndex, Addr: netip.AddrFrom4([4]byte{0, 0, 0, 0})},
				&RouteDestPrefix{ifaceIndex: quuxIfaceIndex, Prefix: netip.MustParsePrefix("0.0.0.0/0")},
				&RouteDestPrefix{ifaceIndex: vnetIfaceIndex, Prefix: netip.MustParsePrefix("0.0.0.0/1")},
				&RouteDestPrefix{ifaceIndex: vnetIfaceIndex, Prefix: netip.MustParsePrefix("128.0.0.0/1")},
			},
			checkResult: func(t *testing.T, dests []RouteDest, rcs []*diagv1.RouteConflict) {
				require.Empty(t, rcs)
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			interfaces := &FakeInterfaces{
				ifaces: map[int]iface{
					vnetIfaceIndex: {name: vnetIface},
					quuxIfaceIndex: {name: quuxIface, app: "foobar"},
				},
			}
			routing := &FakeRouting{dests: test.dests}

			routeConflictDiag, err := NewRouteConflictDiag(&RouteConflictConfig{
				VnetIfaceName: vnetIface, Interfaces: interfaces, Routing: routing, RefetchRoutesDuration: time.Millisecond,
			})
			require.NoError(t, err)
			report, err := routeConflictDiag.Run(context.Background())
			require.NoError(t, err)
			rcs := report.GetRouteConflictReport().RouteConflicts

			test.checkResult(t, test.dests, rcs)
			require.Equal(t, 1, routing.getRouteDestinationsCallCount, "Unexpected number of calls to Routing.GetRouteDestinations")
		})
	}
}

func TestRouteConflictDiag_RetriesOnUnstableIfaceError(t *testing.T) {
	interfaces := &FakeInterfaces{
		ifaces: map[int]iface{
			vnetIfaceIndex: {name: vnetIface},
			quuxIfaceIndex: {name: quuxIface, app: "foobar", willErrorOnce: true},
		},
	}
	routing := &FakeRouting{dests: []RouteDest{
		&RouteDestIP{ifaceIndex: quuxIfaceIndex, Addr: netip.AddrFrom4([4]byte{1, 2, 3, 4})},
		&RouteDestIP{ifaceIndex: vnetIfaceIndex, Addr: netip.AddrFrom4([4]byte{1, 2, 3, 4})},
	}}

	routeConflictDiag, err := NewRouteConflictDiag(&RouteConflictConfig{
		VnetIfaceName: vnetIface, Interfaces: interfaces, Routing: routing, RefetchRoutesDuration: time.Millisecond,
	})
	require.NoError(t, err)
	_, err = routeConflictDiag.Run(context.Background())
	require.NoError(t, err)

	require.Equal(t, 2, routing.getRouteDestinationsCallCount, "Unexpected number of calls to Routing.GetRouteDestinations")
}

func TestRouteConflictDiag_RetriesUpToThreeTimes(t *testing.T) {
	interfaces := &FakeInterfaces{
		ifaces: map[int]iface{
			vnetIfaceIndex: {name: vnetIface},
			quuxIfaceIndex: {name: quuxIface, app: "foobar", willAlwaysError: true},
		},
	}
	routing := &FakeRouting{dests: []RouteDest{
		&RouteDestIP{ifaceIndex: quuxIfaceIndex, Addr: netip.AddrFrom4([4]byte{1, 2, 3, 4})},
		&RouteDestIP{ifaceIndex: vnetIfaceIndex, Addr: netip.AddrFrom4([4]byte{1, 2, 3, 4})},
	}}

	routeConflictDiag, err := NewRouteConflictDiag(&RouteConflictConfig{
		VnetIfaceName: vnetIface, Interfaces: interfaces, Routing: routing, RefetchRoutesDuration: time.Millisecond,
	})
	require.NoError(t, err)
	_, err = routeConflictDiag.Run(context.Background())
	require.ErrorContains(t, err, "whoops something went wrong")

	require.Equal(t, 3, routing.getRouteDestinationsCallCount, "Unexpected number of calls to Routing.GetRouteDestinations")
}

func TestRouteConflictDiag_RetriesOnNoVnetRouteDestinations(t *testing.T) {
	interfaces := &FakeInterfaces{
		ifaces: map[int]iface{
			vnetIfaceIndex: {name: vnetIface},
			quuxIfaceIndex: {name: quuxIface, app: "foobar"},
		},
	}
	routing := &FakeRouting{dests: []RouteDest{
		&RouteDestIP{ifaceIndex: quuxIfaceIndex, Addr: netip.AddrFrom4([4]byte{1, 2, 3, 4})},
	}}

	routeConflictDiag, err := NewRouteConflictDiag(&RouteConflictConfig{
		VnetIfaceName: vnetIface, Interfaces: interfaces, Routing: routing, RefetchRoutesDuration: time.Millisecond,
	})
	require.NoError(t, err)
	report, err := routeConflictDiag.Run(context.Background())
	require.NoError(t, err)
	require.Empty(t, report.GetRouteConflictReport().RouteConflicts)

	require.Equal(t, 3, routing.getRouteDestinationsCallCount, "Unexpected number of calls to Routing.GetRouteDestinations")
}

type FakeInterfaces struct {
	ifaces map[int]iface
}

type iface struct {
	name string
	app  string
	// willErrorOnce makes the first call that returns this interface fail, after which willErrorOnce
	// is swapped to false. This is in order to test a situation where an interface gets removed after
	// fetching routing messages.
	willErrorOnce bool
	// willAlwaysError is like willErrorOnce, but it never gets swapped to false.
	willAlwaysError bool
}

func (f *FakeInterfaces) InterfaceByName(ifaceName string) (*net.Interface, error) {
	for index, iface := range f.ifaces {
		if iface.name != ifaceName {
			continue
		}

		if iface.willErrorOnce || iface.willAlwaysError {
			iface.willErrorOnce = false
			f.ifaces[index] = iface
			return nil, trace.Errorf("whoops something went wrong")
		}

		return &net.Interface{
			Index: index,
			Name:  iface.name,
		}, nil
	}
	return nil, trace.NotFound("interface %s not found", ifaceName)
}

func (f *FakeInterfaces) InterfaceByIndex(index int) (*net.Interface, error) {
	iface, ok := f.ifaces[index]
	if !ok {
		return nil, trace.NotFound("interface with index %d not found", index)
	}

	if iface.willErrorOnce || iface.willAlwaysError {
		iface.willErrorOnce = false
		f.ifaces[index] = iface
		return nil, trace.Errorf("whoops something went wrong")
	}

	return &net.Interface{
		Index: index,
		Name:  iface.name,
	}, nil
}

func (f *FakeInterfaces) InterfaceApp(ctx context.Context, ifaceName string) (string, error) {
	for _, iface := range f.ifaces {
		if iface.name == ifaceName {
			return iface.app, nil
		}
	}
	return "", trace.NotFound("interface %s not found", ifaceName)
}

type FakeRouting struct {
	dests                         []RouteDest
	getRouteDestinationsCallCount int
}

func (f *FakeRouting) GetRouteDestinations() ([]RouteDest, error) {
	f.getRouteDestinationsCallCount++
	return f.dests, nil
}

func TestExtractNetworkExtDescFromIfconfigOutput(t *testing.T) {
	tests := map[string]struct {
		ifconfigOutput string
		result         string
	}{
		"desc in output": {
			ifconfigOutput: ifconfigOutputWithDesc,
			result:         "VPN: foobar",
		},
		// This is probably unlikely to happen, but we just want to test if the function behaves
		// properly if it cannot match on desc.
		"NetworkExtension line but no desc": {
			ifconfigOutput: ifconfigOutputWithNetworkExtButNoDesc,
			result:         "",
		},
		"no NetworkExtension line": {
			ifconfigOutput: "\n\n",
			result:         "",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, test.result, extractNetworkExtDescFromIfconfigOutput([]byte(test.ifconfigOutput)))
		})
	}
}

// ifconfigOutputWithDesc is the actual output of ifconfig -v utun4 on macOS 15.3.
const ifconfigOutputWithDesc = `utun4: flags=8051<UP,POINTOPOINT,RUNNING,MULTICAST> mtu 1280 index 25
	eflags=5002080<TXSTART,NOAUTOIPV6LL,ECN_ENABLE,CHANNEL_DRV>
	xflags=10004<NOAUTONX,IS_VPN>
	options=6460<TSO4,TSO6,CHANNEL_IO,PARTIAL_CSUM,ZEROINVERT_CSUM>
	hwassist=703000<CSUM_PARTIAL,CSUM_ZERO_INVERT,MULTIPAGES,TSO_V4,TSO_V6>
	inet6 fe80::2e0:4cff:fe3c:b817%utun4 prefixlen 64 scopeid 0x19
	inet 100.87.112.117 --> 100.87.112.117 netmask 0xffffffff
	inet6 fd7a:115c:a1e0::1901:7075 prefixlen 48
	netif: 262EDF8C-DAD8-4EAD-B0BA-2C6E79E15607
	flowswitch: E2367B02-1C93-4E5A-BF04-0108079B828E
	nd6 options=201<PERFORMNUD,DAD>
	generation id: 381
	agent domain:Skywalk type:NetIf flags:0x8443 desc:"Userspace Networking"
	agent domain:Skywalk type:FlowSwitch flags:0x4403 desc:"Userspace Networking"
	agent domain:NetworkExtension type:VPN flags:0xf desc:"VPN: foobar"
	link quality: -1 (unknown)
	state availability: 0 (true)
	scheduler: FQ_CODEL
	effective interface: en0
	qosmarking enabled: no mode: none
	low power mode: disabled
	multi layer packet logging (mpklog): disabled
	routermode4: disabled
	routermode6: disabled
`

const ifconfigOutputWithNetworkExtButNoDesc = `utun4: flags=8051<UP,POINTOPOINT,RUNNING,MULTICAST> mtu 1280 index 25
	agent domain:Skywalk type:NetIf flags:0x8443 desc:"Userspace Networking"
	agent domain:Skywalk type:FlowSwitch flags:0x4403 desc:"Userspace Networking"
	agent domain:NetworkExtension type:VPN flags:0xf
	link quality: -1 (unknown)
`
