// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
	"golang.zx2c4.com/wireguard/device"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	ipv4network "gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	ipv6network "gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/vnet/dns"
)

const (
	nicID                            = 1
	mtu                              = 1500
	tcpReceiveBufferSize             = 0 // 0 means a default will be used.
	maxInFlightTCPConnectionAttempts = 1024
	defaultIPv4CIDRRange             = "100.64.0.0/10"
)

// Config holds configuration parameters for the VNet.
type Config struct {
	// TUNDevice is the OS TUN virtual network interface.
	TUNDevice TUNDevice
	// IPv6Prefix is the IPv6 ULA prefix to use for all assigned VNet IP addresses.
	IPv6Prefix tcpip.Address
	// DNSIPv6 is the IPv6 address on which to host the DNS server. It must be under IPv6Prefix.
	DNSIPv6 tcpip.Address
	// TCPHandlerResolver will be used to resolve all DNS queries that may be valid public addresses for
	// Teleport apps.
	TCPHandlerResolver TCPHandlerResolver

	// upstreamNameserverSource, if set, overrides the default OS UpstreamNameserverSource which provides the
	// IP addresses that unmatched DNS queries should be forwarded to. It is used in tests.
	upstreamNameserverSource dns.UpstreamNameserverSource
}

// CheckAndSetDefaults checks the config and sets defaults.
func (c *Config) CheckAndSetDefaults() error {
	if c.TUNDevice == nil {
		return trace.BadParameter("TUNdevice is required")
	}
	if c.IPv6Prefix.Len() != 16 || c.IPv6Prefix.AsSlice()[0] != 0xfd {
		return trace.BadParameter("IPv6Prefix must be an IPv6 ULA address")
	}
	if c.TCPHandlerResolver == nil {
		return trace.BadParameter("TCPHandlerResolver is required")
	}
	return nil
}

// TCPHandlerResolver describes a type that can resolve a fully-qualified domain name to a TCPHandlerSpec that
// defines the CIDR range to assign an IP to that handler from, and a handler for all future connections to
// that IP address.
//
// Implementations beware - an FQDN always ends with a '.'.
type TCPHandlerResolver interface {
	// ResolveTCPHandler decides if [fqdn] should match a TCP handler.
	//
	// If [fqdn] matches a Teleport-managed TCP app it must return a TCPHandlerSpec defining the range to
	// assign an IP from, and a handler for future connections to any assigned IPs.
	//
	// If [fqdn] does not match it must return ErrNoTCPHandler. Avoid using [trace.Wrap] for expected errors
	// to avoid the overhead of capturing a full stack trace.
	ResolveTCPHandler(ctx context.Context, fqdn string) (*TCPHandlerSpec, error)
}

// ErrNoTCPHandler should be returned by [TCPHandlerResolver]s when no handler matches the FQDN.
var ErrNoTCPHandler = errors.New("no handler for address")

// TCPHandlerSpec specifies a VNet TCP handler.
type TCPHandlerSpec struct {
	// IPv4CIDRRange is the network that any V4 IP address should be assigned to this handler from.
	IPv4CIDRRange string
	// TCPHandler is the handler for TCP connections.
	TCPHandler TCPHandler
}

// TCPHandler defines the behavior for handling TCP connections from VNet.
//
// Implementations should attempt to dial the target application and return any errors before calling
// [connector] to complete the TCP handshake and get the TCP conn. This is so that clients will see that the
// TCP connection was refused, instead of seeing a successful TCP dial that is immediately closed.
type TCPHandler interface {
	HandleTCPConnector(ctx context.Context, connector func() (net.Conn, error)) error
}

// UDPHandler defines the behavior for handling UDP connections from VNet.
type UDPHandler interface {
	HandleUDP(context.Context, net.Conn) error
}

// TUNDevice abstracts a virtual network TUN device.
type TUNDevice interface {
	// Write one or more packets to the device (without any additional headers).
	// On a successful write it returns the number of packets written. A nonzero
	// offset can be used to instruct the Device on where to begin writing from
	// each packet contained within the bufs slice.
	Write(bufs [][]byte, offset int) (int, error)

	// Read one or more packets from the Device (without any additional headers).
	// On a successful read it returns the number of packets read, and sets
	// packet lengths within the sizes slice. len(sizes) must be >= len(bufs).
	// A nonzero offset can be used to instruct the Device on where to begin
	// reading into each element of the bufs slice.
	Read(bufs [][]byte, sizes []int, offset int) (n int, err error)

	// BatchSize returns the preferred/max number of packets that can be read or
	// written in a single read/write call. BatchSize must not change over the
	// lifetime of a Device.
	BatchSize() int

	// Close stops the Device and closes the Event channel.
	Close() error
}

// Manager holds configuration and state for the VNet.
type Manager struct {
	// stack is the gVisor networking stack.
	stack *stack.Stack

	// tun is the OS TUN device. Incoming IP/L3 packets will be copied from here to [linkEndpoint], and
	// outgoing packets from [linkEndpoint] will be written here.
	tun TUNDevice

	// linkEndpoint is the gVisor-side endpoint that emulates the OS TUN device. All incoming IP/L3 packets
	// from the OS TUN device will be injected as inbound packets to this endpoint to be processed by the
	// gVisor netstack which ultimately calls the TCP or UDP protocol handler. When the protocol handler
	// writes packets to the gVisor stack to an address assigned to this endpoint, they will be written to
	// this endpoint, and then copied from this endpoint to the OS TUN device.
	linkEndpoint *channel.Endpoint

	// ipv6Prefix holds the 96-bit prefix that will be used for all IPv6 addresses assigned in the VNet.
	ipv6Prefix tcpip.Address

	// tcpHandlerResolver resolves app FQDNs to a TCP handler that will be used to handle all future TCP
	// connections to IP addresses that will be assigned to that FQDN.
	tcpHandlerResolver TCPHandlerResolver
	// resolveHandlerGroup is a [singleflight.Group] that will be used to avoid resolving the same FQDN
	// multiple times concurrently. Every call to [tcpHandlerResolver.ResolveTCPHandler] will be wrapped by
	// this. The key will be the FQDN.
	resolveHandlerGroup singleflight.Group

	// destroyed is a channel that will be closed when the VNet is in the process of being destroyed.
	// All goroutines should terminate quickly after either this is closed or the context passed to
	// [Manager.Run] is canceled.
	destroyed chan struct{}
	// wg is a [sync.WaitGroup] that keeps track of all running goroutines started by the [Manager].
	wg sync.WaitGroup

	// state holds all mutable state for the Manager, it is currently protect by a single RWMutex, this could
	// be optimized as necessary.
	state state

	slog *slog.Logger
}

type state struct {
	mu sync.RWMutex

	// Each app gets assigned both an IPv4 address and an IPv6 address, where the 4-bit suffix of the IPv6
	// matches the IPv4 address exactly. All per-app state references the smaller IPv4 address only and
	// lookups based on an IPv6 address can use the 4-byte suffix.

	// tcpHandlers holds the map of IP addresses to assigned TCP handlers.
	tcpHandlers map[ipv4]TCPHandler
	// appIPs holds the map of app FQDNs to their assigned IP address, it like a reverse map of [tcpHandlers].
	appIPs map[string]ipv4

	// udpHandlers holds the map of IP addresses to assigned UDP handlers.
	udpHandlers map[ipv4]UDPHandler
}

func newState() state {
	return state{
		tcpHandlers: make(map[ipv4]TCPHandler),
		udpHandlers: make(map[ipv4]UDPHandler),
		appIPs:      make(map[string]ipv4),
	}
}

// NewManager creates a new VNet manager with the given configuration and root
// context. Call Run() on the returned manager to start the VNet.
// NewManager creates a new VNet manager with the given configuration and root context. It takes ownership of
// [cfg.TUNDevice] and will handle closing it before Run() returns. Call Run() on the returned manager to
// start the VNet.
func NewManager(cfg *Config) (*Manager, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	slog := slog.With(teleport.ComponentKey, "VNet")

	stack, linkEndpoint, err := createStack()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := installVnetRoutes(stack); err != nil {
		return nil, trace.Wrap(err)
	}

	m := &Manager{
		tun:                cfg.TUNDevice,
		stack:              stack,
		linkEndpoint:       linkEndpoint,
		ipv6Prefix:         cfg.IPv6Prefix,
		tcpHandlerResolver: cfg.TCPHandlerResolver,
		destroyed:          make(chan struct{}),
		state:              newState(),
		slog:               slog,
	}

	tcpForwarder := tcp.NewForwarder(m.stack, tcpReceiveBufferSize, maxInFlightTCPConnectionAttempts, m.handleTCP)
	m.stack.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpForwarder.HandlePacket)

	udpForwarder := udp.NewForwarder(m.stack, m.handleUDP)
	m.stack.SetTransportProtocolHandler(udp.ProtocolNumber, udpForwarder.HandlePacket)

	if cfg.DNSIPv6 != (tcpip.Address{}) {
		upstreamNameserverSource := cfg.upstreamNameserverSource
		if upstreamNameserverSource == nil {
			upstreamNameserverSource, err = dns.NewOSUpstreamNameserverSource()
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		dnsServer, err := dns.NewServer(m, upstreamNameserverSource)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := m.assignUDPHandler(cfg.DNSIPv6, dnsServer); err != nil {
			return nil, trace.Wrap(err)
		}
		slog.DebugContext(context.Background(), "Serving DNS on IPv6.", "dns_addr", cfg.DNSIPv6)
	}

	return m, nil
}

func createStack() (*stack.Stack, *channel.Endpoint, error) {
	netStack := stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4network.NewProtocol, ipv6network.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol},
	})

	const (
		size     = 512
		linkAddr = ""
	)
	linkEndpoint := channel.New(size, mtu, linkAddr)
	if err := netStack.CreateNIC(nicID, linkEndpoint); err != nil {
		return nil, nil, trace.Errorf("creating VNet NIC: %s", err)
	}

	return netStack, linkEndpoint, nil
}

func installVnetRoutes(stack *stack.Stack) error {
	// Make the network stack pass all outbound IP packets to the NIC, regardless of destination IP address.
	ipv4Subnet, err := tcpip.NewSubnet(tcpip.AddrFrom4([4]byte{}), tcpip.MaskFromBytes(make([]byte, 4)))
	if err != nil {
		return trace.Wrap(err, "creating VNet IPv4 subnet")
	}
	ipv6Subnet, err := tcpip.NewSubnet(tcpip.AddrFrom16([16]byte{}), tcpip.MaskFromBytes(make([]byte, 16)))
	if err != nil {
		return trace.Wrap(err, "creating VNet IPv6 subnet")
	}
	stack.SetRouteTable([]tcpip.Route{
		{
			Destination: ipv4Subnet,
			NIC:         nicID,
		},
		{
			Destination: ipv6Subnet,
			NIC:         nicID,
		},
	})
	return nil
}

// Run starts the VNet. It blocks until [ctx] is canceled, at which point it closes the link endpoint, waits
// for all goroutines to terminate, and destroys the networking stack.
func (m *Manager) Run(ctx context.Context) error {
	m.slog.InfoContext(ctx, "Running Teleport VNet.", "ipv6_prefix", m.ipv6Prefix)

	ctx, cancel := context.WithCancel(ctx)

	allErrors := make(chan error, 2)
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		// Make sure to cancel the context in case this exits prematurely with a nil error.
		defer cancel()
		err := forwardBetweenTunAndNetstack(ctx, m.tun, m.linkEndpoint)
		allErrors <- err
		return err
	})
	g.Go(func() error {
		// When the context is canceled for any reason (the caller or one of the other concurrent tasks may
		// have canceled it) destroy everything and quit.
		<-ctx.Done()

		// In-flight connections should start terminating after closing [m.destroyed].
		close(m.destroyed)

		// Close the link endpoint and the TUN, this should cause [forwardBetweenTunAndNetstack] to terminate
		// if it hasn't already.
		m.linkEndpoint.Close()
		err := trace.Wrap(m.tun.Close(), "closing TUN device")

		allErrors <- err
		return err
	})

	// Deliberately ignoring the error from g.Wait() to return an aggregate of all errors.
	_ = g.Wait()

	// Wait for all connections and goroutines to clean themselves up.
	m.wg.Wait()

	// Now we can destroy the gVisor networking stack and wait for all its goroutines to terminate.
	m.stack.Destroy()

	close(allErrors)
	return trace.NewAggregateFromChannel(allErrors, context.Background())
}

func (m *Manager) handleTCP(req *tcp.ForwarderRequest) {
	// Add 1 to the waitgroup because the networking stack runs this in its own goroutine.
	m.wg.Add(1)
	defer m.wg.Done()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Clients of *tcp.ForwarderRequest must eventually call Complete on it exactly once.
	// [req] consumes 1 of [maxInFlightTCPConnectionAttempts] until [req.Complete] is called.
	var completed bool
	defer func() {
		if !completed {
			req.Complete(true /*send TCP reset*/)
		}
	}()

	id := req.ID()
	slog := m.slog.With("request", id)
	slog.DebugContext(ctx, "Handling TCP connection.")
	defer slog.DebugContext(ctx, "Finished handling TCP connection.")

	handler, ok := m.getTCPHandler(id.LocalAddress)
	if !ok {
		slog.DebugContext(ctx, "No handler for address.", "addr", id.LocalAddress)
		return
	}

	connector := func() (net.Conn, error) {
		var wq waiter.Queue
		waitEntry, notifyCh := waiter.NewChannelEntry(waiter.EventErr | waiter.EventHUp)
		wq.EventRegister(&waitEntry)

		endpoint, err := req.CreateEndpoint(&wq)
		if err != nil {
			// This err doesn't actually implement [error]
			return nil, trace.Errorf("creating TCP endpoint: %s", err)
		}

		completed = true
		req.Complete(false /*don't send TCP reset*/)

		endpoint.SocketOptions().SetKeepAlive(true)

		conn := gonet.NewTCPConn(&wq, endpoint)

		m.wg.Add(1)
		go func() {
			defer func() {
				cancel()
				conn.Close()
				m.wg.Done()
			}()
			select {
			case <-notifyCh:
				slog.DebugContext(ctx, "Got HUP or ERR, canceling request context and closing TCP conn.")
			case <-m.destroyed:
				slog.DebugContext(ctx, "VNet is being destroyed, canceling request context and closing TCP conn.")
			case <-ctx.Done():
				slog.DebugContext(ctx, "Request context canceled, closing TCP conn.")
			}
		}()

		return conn, nil
	}

	if err := handler.HandleTCPConnector(ctx, connector); err != nil {
		if errors.Is(err, context.Canceled) {
			slog.DebugContext(ctx, "TCP connection handler returned early due to canceled context.")
		} else {
			slog.DebugContext(ctx, "Error handling TCP connection.", "err", err)
		}
	}
}

func (m *Manager) getTCPHandler(addr tcpip.Address) (TCPHandler, bool) {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	handler, ok := m.state.tcpHandlers[ipv4Suffix(addr)]
	return handler, ok
}

// assignTCPHandler assigns an IPv4 address to [handlerSpec] from its preferred CIDR range, and returns that
// new assigned address.
func (m *Manager) assignTCPHandler(handlerSpec *TCPHandlerSpec, fqdn string) (ipv4, error) {
	_, ipNet, err := net.ParseCIDR(handlerSpec.IPv4CIDRRange)
	if err != nil {
		return 0, trace.Wrap(err, "parsing CIDR %q", handlerSpec.IPv4CIDRRange)
	}

	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	ip, err := randomFreeIPv4InNet(ipNet, func(ip ipv4) bool {
		_, taken := m.state.tcpHandlers[ip]
		return !taken
	})
	if err != nil {
		return 0, trace.Wrap(err, "assigning IP address")
	}

	m.state.tcpHandlers[ip] = handlerSpec.TCPHandler
	m.state.appIPs[fqdn] = ip

	if err := m.addProtocolAddress(tcpip.AddrFrom4(ip.asArray())); err != nil {
		return 0, trace.Wrap(err)
	}
	if err := m.addProtocolAddress(ipv6WithSuffix(m.ipv6Prefix, ip.asSlice())); err != nil {
		return 0, trace.Wrap(err)
	}

	return ip, nil
}

func (m *Manager) handleUDP(req *udp.ForwarderRequest) {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.handleUDPConcurrent(req)
	}()
}

func (m *Manager) handleUDPConcurrent(req *udp.ForwarderRequest) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	id := req.ID()
	slog := m.slog.With("request", id)
	slog.DebugContext(ctx, "Handling UDP request.")
	defer slog.DebugContext(ctx, "Finished handling UDP request.")

	handler, ok := m.getUDPHandler(id.LocalAddress)
	if !ok {
		slog.DebugContext(ctx, "No handler for address.")
		return
	}

	var wq waiter.Queue
	waitEntry, notifyCh := waiter.NewChannelEntry(waiter.EventErr | waiter.EventHUp)
	wq.EventRegister(&waitEntry)

	endpoint, err := req.CreateEndpoint(&wq)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create UDP endpoint.", "error", err)
		return
	}

	conn := gonet.NewUDPConn(m.stack, &wq, endpoint)
	defer conn.Close()

	m.wg.Add(1)
	go func() {
		defer func() {
			cancel()
			conn.Close()
			m.wg.Done()
		}()
		select {
		case <-notifyCh:
			slog.DebugContext(ctx, "Got HUP or ERR, canceling request context and closing UDP conn.")
		case <-m.destroyed:
			slog.DebugContext(ctx, "VNet is being destroyed, canceling request context and closing UDP conn.")
		case <-ctx.Done():
			slog.DebugContext(ctx, "Request context canceled, closing UDP conn.")
		}
	}()

	if err := handler.HandleUDP(ctx, conn); err != nil {
		slog.DebugContext(ctx, "Error handling UDP conn.", "error", err)
	}
}

func (m *Manager) getUDPHandler(addr tcpip.Address) (UDPHandler, bool) {
	ipv4 := ipv4Suffix(addr)
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	handler, ok := m.state.udpHandlers[ipv4]
	return handler, ok
}

func (m *Manager) assignUDPHandler(addr tcpip.Address, handler UDPHandler) error {
	ipv4 := ipv4Suffix(addr)
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	if _, ok := m.state.udpHandlers[ipv4]; ok {
		return trace.AlreadyExists("Handler for %s is already set", addr)
	}
	if err := m.addProtocolAddress(addr); err != nil {
		return trace.Wrap(err)
	}
	m.state.udpHandlers[ipv4] = handler
	return nil
}

// ResolveA implements [dns.Resolver.ResolveA].
func (m *Manager) ResolveA(ctx context.Context, fqdn string) (dns.Result, error) {
	// Do the actual resolution within a [singleflight.Group] keyed by [fqdn] to avoid concurrent requests to
	// resolve an FQDN and then assign an address to it.
	resultAny, err, _ := m.resolveHandlerGroup.Do(fqdn, func() (any, error) {
		// If we've already assigned an IP address to this app, resolve to it.
		if ip, ok := m.appIPv4(fqdn); ok {
			return dns.Result{
				A: ip.asArray(),
			}, nil
		}

		// If fqdn is a Teleport-managed app, create a new handler for it.
		handlerSpec, err := m.tcpHandlerResolver.ResolveTCPHandler(ctx, fqdn)
		if err != nil {
			if errors.Is(err, ErrNoTCPHandler) {
				// Did not find any known app, forward the DNS request upstream.
				return dns.Result{}, nil
			}
			return dns.Result{}, trace.Wrap(err, "resolving TCP handler for fqdn %q", fqdn)
		}

		// Assign an unused IP address to this app's handler.
		ip, err := m.assignTCPHandler(handlerSpec, fqdn)
		if err != nil {
			return dns.Result{}, trace.Wrap(err, "assigning address to handler for %q", fqdn)
		}

		// And resolve to the assigned address.
		return dns.Result{
			A: ip.asArray(),
		}, nil
	})
	if err != nil {
		return dns.Result{}, trace.Wrap(err)
	}
	return resultAny.(dns.Result), nil
}

// ResolveAAAA implements [dns.Resolver.ResolveAAAA].
func (m *Manager) ResolveAAAA(ctx context.Context, fqdn string) (dns.Result, error) {
	result, err := m.ResolveA(ctx, fqdn)
	if err != nil {
		return dns.Result{}, trace.Wrap(err)
	}
	if result.A != ([4]byte{}) {
		result.AAAA = ipv6WithSuffix(m.ipv6Prefix, result.A[:]).As16()
		result.A = [4]byte{}
	}
	return result, nil
}

func (m *Manager) appIPv4(fqdn string) (ipv4, bool) {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	ipv4, ok := m.state.appIPs[fqdn]
	return ipv4, ok
}

func forwardBetweenTunAndNetstack(ctx context.Context, tun TUNDevice, linkEndpoint *channel.Endpoint) error {
	slog.DebugContext(ctx, "Forwarding IP packets between OS and VNet.")
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return forwardNetstackToTUN(ctx, linkEndpoint, tun) })
	g.Go(func() error { return forwardTUNtoNetstack(tun, linkEndpoint) })
	return trace.Wrap(g.Wait())
}

func forwardNetstackToTUN(ctx context.Context, linkEndpoint *channel.Endpoint, tun TUNDevice) error {
	bufs := [][]byte{make([]byte, device.MessageTransportHeaderSize+mtu)}
	for {
		packet := linkEndpoint.ReadContext(ctx)
		if packet.IsNil() {
			// Nil packet is returned when context is canceled.
			return trace.Wrap(ctx.Err())
		}
		offset := device.MessageTransportHeaderSize
		for _, s := range packet.AsSlices() {
			offset += copy(bufs[0][offset:], s)
		}
		packet.DecRef()
		bufs[0] = bufs[0][:offset]
		if _, err := tun.Write(bufs, device.MessageTransportHeaderSize); err != nil {
			return trace.Wrap(err, "writing packets to TUN")
		}
		bufs[0] = bufs[0][:cap(bufs[0])]
	}
}

func forwardTUNtoNetstack(tun TUNDevice, linkEndpoint *channel.Endpoint) error {
	const readOffset = device.MessageTransportHeaderSize
	bufs := make([][]byte, tun.BatchSize())
	for i := range bufs {
		bufs[i] = make([]byte, device.MessageTransportHeaderSize+mtu)
	}
	sizes := make([]int, len(bufs))
	for {
		n, err := tun.Read(bufs, sizes, readOffset)
		if err != nil {
			return trace.Wrap(err, "reading packets from TUN")
		}
		for i := range sizes[:n] {
			protocol, ok := protocolVersion(bufs[i][readOffset])
			if !ok {
				continue
			}
			packet := stack.NewPacketBuffer(stack.PacketBufferOptions{
				Payload: buffer.MakeWithData(bufs[i][readOffset : readOffset+sizes[i]]),
			})
			linkEndpoint.InjectInbound(protocol, packet)
			packet.DecRef()
		}
	}
}

func (m *Manager) addProtocolAddress(localAddress tcpip.Address) error {
	protocolAddress, err := protocolAddress(localAddress)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := m.stack.AddProtocolAddress(nicID, protocolAddress, stack.AddressProperties{}); err != nil {
		return trace.Errorf("%s", err)
	}
	return nil
}

func protocolAddress(addr tcpip.Address) (tcpip.ProtocolAddress, error) {
	addrWithPrefix := addr.WithPrefix()
	var protocol tcpip.NetworkProtocolNumber
	switch addrWithPrefix.PrefixLen {
	case 32:
		protocol = ipv4network.ProtocolNumber
	case 128:
		protocol = ipv6network.ProtocolNumber
	default:
		return tcpip.ProtocolAddress{}, trace.BadParameter("unhandled prefix len %d", addrWithPrefix.PrefixLen)
	}
	return tcpip.ProtocolAddress{
		AddressWithPrefix: addrWithPrefix,
		Protocol:          protocol,
	}, nil
}

func protocolVersion(b byte) (tcpip.NetworkProtocolNumber, bool) {
	switch b >> 4 {
	case 4:
		return header.IPv4ProtocolNumber, true
	case 6:
		return header.IPv6ProtocolNumber, true
	}
	return 0, false
}
