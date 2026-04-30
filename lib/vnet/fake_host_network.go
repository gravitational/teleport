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

package vnet

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"sync"

	"github.com/gravitational/trace"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	netipv4 "gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	netipv6 "gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// NewFakeHostNetwork returns an in-memory emulation of a host network, intended
// for use in tests, backed by the gVisor network stack. Users must call Close on
// the network to clean up its resources.
func NewFakeHostNetwork() (*FakeHostNetwork, error) {
	stack, nic, err := createStack()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hostTun, vnetTun := newSplitTUN()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := forwardBetweenTunAndNetstack(ctx, hostTun, nic); err != nil {
			slog.ErrorContext(ctx, "Failed to forward packets between TUN and network stack")
		}
	}()

	return &FakeHostNetwork{
		tun:   vnetTun,
		stack: stack,
		nic:   nic,
		closeFn: func() {
			cancel()

			if err := hostTun.Close(); err != nil {
				slog.ErrorContext(ctx, "failed to close host-side TUN")
			}
		},
		readyCh: make(chan struct{}),
	}, nil
}

// FakeHostNetwork implements an in-memory emulation of a host network for use
// in tests.
type FakeHostNetwork struct {
	tun     TUNDevice
	stack   *stack.Stack
	nic     *channel.Endpoint
	closeFn func()

	mu         sync.Mutex
	readyCh    chan struct{}
	configured bool
	dnsAddrs   []string
	dnsZones   []string
}

// TUNDevice returns a TUN device VNet can use to receive traffic from the host
// network.
func (f *FakeHostNetwork) TUNDevice() TUNDevice {
	return f.tun
}

// Ready returns a channel that is closed when the network has been configured.
func (f *FakeHostNetwork) Ready() <-chan struct{} {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.readyCh
}

// DNSAddrs returns the configured nameserver addresses.
func (f *FakeHostNetwork) DNSAddrs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.dnsAddrs
}

// DNSZones returns the configured DNS zones.
func (f *FakeHostNetwork) DNSZones() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.dnsZones
}

// ResolveAndDial resolves the given hostname to an IP address and dials a
// connection to it.
func (f *FakeHostNetwork) ResolveAndDial(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	portNum, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ips, err := f.DNSResolver().LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(ips) == 0 {
		return nil, trace.Errorf("failed to resolve %s", host)
	}
	netAddr, _ := netip.AddrFromSlice(ips[0])
	return f.DialIP(
		ctx,
		"tcp",
		netip.AddrPortFrom(netAddr, uint16(portNum)).String(),
	)
}

// DialIP dials a connection to a given IP address.
func (f *FakeHostNetwork) DialIP(ctx context.Context, network, addr string) (net.Conn, error) {
	target, err := netip.ParseAddrPort(addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fullAddr := &tcpip.FullAddress{
		NIC:      nicID,
		Addr:     tcpip.AddrFromSlice(target.Addr().AsSlice()),
		Port:     target.Port(),
		LinkAddr: f.nic.LinkAddress(),
	}

	protocolNumber := netipv4.ProtocolNumber
	if target.Addr().Is6() {
		protocolNumber = netipv6.ProtocolNumber
	}

	switch network {
	case "udp":
		return gonet.DialUDP(f.stack, nil, fullAddr, protocolNumber)
	case "tcp":
		return gonet.DialContextTCP(ctx, f.stack, *fullAddr, protocolNumber)
	default:
		return nil, trace.Errorf("unsupported network: %s", network)
	}
}

// Configure the network's routing table, etc.
func (f *FakeHostNetwork) Configure(ctx context.Context, cfg *EmbeddedVNetHostConfig) error {
	if cfg == nil {
		return nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.configured {
		return nil
	}

	f.configured = true
	f.dnsAddrs = cfg.DNSAddrs
	f.dnsZones = cfg.DNSZones

	// Assign the interface an IPv4 address.
	if err := f.stack.AddProtocolAddress(
		nicID,
		tcpip.ProtocolAddress{
			Protocol: netipv4.ProtocolNumber,
			AddressWithPrefix: tcpip.AddressWithPrefix{
				Address:   tcpip.AddrFromSlice(net.ParseIP(cfg.DeviceIPv4).To4()),
				PrefixLen: 32,
			},
		},
		stack.AddressProperties{},
	); err != nil {
		return trace.Errorf("failed to add IPv4 address: %s", err.String())
	}

	// Assign the interface an IPv6 address.
	if err := f.stack.AddProtocolAddress(
		nicID,
		tcpip.ProtocolAddress{
			Protocol: netipv6.ProtocolNumber,
			AddressWithPrefix: tcpip.AddressWithPrefix{
				Address:   tcpip.AddrFromSlice(net.ParseIP(cfg.DeviceIPv6)),
				PrefixLen: 128,
			},
		},
		stack.AddressProperties{},
	); err != nil {
		return trace.Errorf("failed to add IPv6 address: %s", err.String())
	}

	// Configure the route table.
	var routes []tcpip.Route
	for _, cidr := range cfg.CIDRRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return trace.Wrap(err)
		}
		dest, err := tcpip.NewSubnet(
			tcpip.AddrFromSlice(network.IP),
			tcpip.MaskFromBytes(network.Mask),
		)
		if err != nil {
			return trace.Wrap(err)
		}
		routes = append(routes, tcpip.Route{
			Destination: dest,
			NIC:         nicID,
		})
	}
	f.stack.SetRouteTable(routes)

	close(f.readyCh)
	return nil
}

// DNSResolver returns a net.Resolver that will always dial the VNet nameserver.
func (f *FakeHostNetwork) DNSResolver() *net.Resolver {
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			nameservers := f.DNSAddrs()
			if len(nameservers) == 0 {
				return nil, trace.Errorf("no nameservers configured")
			}
			addr, err := netip.ParseAddr(nameservers[0])
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return f.DialIP(ctx, network, netip.AddrPortFrom(addr, 53).String())
		},
	}
}

// HTTPTransport returns an HTTP transport that resolves and dials targets via
// the host network.
func (f *FakeHostNetwork) HTTPTransport() *http.Transport {
	return &http.Transport{
		DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
			return f.ResolveAndDial(ctx, "tcp", addr)
		},
	}
}

// Close the network to free its resources.
func (f *FakeHostNetwork) Close() { f.closeFn() }

// newSplitTUN returns two fake TUN devices that are tied together: writes to one can be read on the other,
// and vice versa.
func newSplitTUN() (*fakeTUN, *fakeTUN) {
	aClosed := make(chan struct{})
	bClosed := make(chan struct{})
	ab := make(chan []byte)
	ba := make(chan []byte)
	return &fakeTUN{
			name:            "tun1",
			writePacketsTo:  ab,
			readPacketsFrom: ba,
			closed:          aClosed,
			closeOnce:       sync.OnceFunc(func() { close(aClosed) }),
		}, &fakeTUN{
			name:            "tun2",
			writePacketsTo:  ba,
			readPacketsFrom: ab,
			closed:          bClosed,
			closeOnce:       sync.OnceFunc(func() { close(bClosed) }),
		}
}

var errFakeTUNClosed = errors.New("TUN closed")

type fakeTUN struct {
	name                            string
	writePacketsTo, readPacketsFrom chan []byte
	closed                          chan struct{}
	closeOnce                       func()
}

func (f *fakeTUN) Name() (string, error) {
	return f.name, nil
}

func (f *fakeTUN) BatchSize() int {
	return 1
}

// Write one or more packets to the device (without any additional headers).
// On a successful write it returns the number of packets written. A nonzero
// offset can be used to instruct the Device on where to begin writing from
// each packet contained within the bufs slice.
func (f *fakeTUN) Write(bufs [][]byte, offset int) (int, error) {
	if len(bufs) != 1 {
		return 0, trace.BadParameter("batchsize is 1")
	}
	packet := make([]byte, len(bufs[0][offset:]))
	copy(packet, bufs[0][offset:])
	select {
	case <-f.closed:
		return 0, errFakeTUNClosed
	case f.writePacketsTo <- packet:
	}
	return 1, nil
}

// Read one or more packets from the Device (without any additional headers).
// On a successful read it returns the number of packets read, and sets
// packet lengths within the sizes slice. len(sizes) must be >= len(bufs).
// A nonzero offset can be used to instruct the Device on where to begin
// reading into each element of the bufs slice.
func (f *fakeTUN) Read(bufs [][]byte, sizes []int, offset int) (n int, err error) {
	if len(bufs) != 1 {
		return 0, trace.BadParameter("batchsize is 1")
	}
	var packet []byte
	select {
	case <-f.closed:
		return 0, errFakeTUNClosed
	case packet = <-f.readPacketsFrom:
	}
	sizes[0] = copy(bufs[0][offset:], packet)
	return 1, nil
}

func (f *fakeTUN) Close() error {
	f.closeOnce()
	return nil
}
