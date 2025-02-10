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
	"os"
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
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/vnet/dns"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, "vnet")

const (
	nicID                            = 1
	mtu                              = 1500
	tcpReceiveBufferSize             = 0 // 0 means a default will be used.
	maxInFlightTCPConnectionAttempts = 1024
	defaultIPv4CIDRRange             = "100.64.0.0/10"
)

// networkStackConfig holds configuration parameters for the VNet network stack.
type networkStackConfig struct {
	// tunDevice is the OS TUN virtual network interface.
	tunDevice tunDevice
	// ipv6Prefix is the IPv6 ULA prefix to use for all assigned VNet IP addresses.
	ipv6Prefix tcpip.Address
	// dnsIPv6 is the IPv6 address on which to host the DNS server. It must be under IPv6Prefix.
	dnsIPv6 tcpip.Address
	// tcpHandlerResolver will be used to resolve all DNS queries that may be valid public addresses for
	// Teleport apps.
	tcpHandlerResolver tcpHandlerResolver

	// upstreamNameserverSource, if set, overrides the default OS UpstreamNameserverSource which provides the
	// IP addresses that unmatched DNS queries should be forwarded to. It is used in tests.
	upstreamNameserverSource dns.UpstreamNameserverSource
}

// checkAndSetDefaults checks the config and sets defaults.
func (c *networkStackConfig) checkAndSetDefaults() error {
	if c.tunDevice == nil {
		return trace.BadParameter("tunDevice is required")
	}
	if c.ipv6Prefix.Len() != 16 || c.ipv6Prefix.AsSlice()[0] != 0xfd {
		return trace.BadParameter("ipv6Prefix must be an IPv6 ULA address")
	}
	if c.tcpHandlerResolver == nil {
		return trace.BadParameter("tcpHandlerResolver is required")
	}
	return nil
}

// tcpHandlerResolver describes a type that can resolve a fully-qualified domain
// name to a tcpHandlerSpec that defines the CIDR range to assign an IP to
// that handler from, and a handler for all future connections to that IP
// address.
//
// Implementations beware - an FQDN always ends with a '.'.
type tcpHandlerResolver interface {
	// resolveTCPHandler decides if fqdn should match a TCP handler.
	//
	// If fqdn matches a Teleport-managed TCP app it must return a
	// tcpHandlerSpec defining the CIDR range to assign an IP from, and a
	// handler for future connections to any assigned IPs.
	//
	// If fqdn does not match it must return errNoTCPHandler. Avoid using
	// [trace.Wrap] on errNoTCPHandler to prevent collecting a full stack trace
	// on every unhandled query.
	resolveTCPHandler(ctx context.Context, fqdn string) (*tcpHandlerSpec, error)
}

// errNoTCPHandler should be returned by tcpHandlerResolvers when no handler
// matches the FQDN.
//
// Avoid using [trace.Wrap] on errNoTCPHandler where possible, this isn't an
// unexpected error that should require the overhead of collecting a stack trace.
var errNoTCPHandler = &trace.NotFoundError{Message: "no handler for address"}

// tcpHandlerSpec specifies a VNet TCP handler.
type tcpHandlerSpec struct {
	// ipv4CIDRRange is the network that any V4 IP address should be assigned to this handler from.
	ipv4CIDRRange string
	// tcpHandler is the handler for TCP connections.
	tcpHandler tcpHandler
}

// tcpHandler defines the behavior for handling TCP connections from VNet.
//
// Implementations should attempt to dial the target application and return any errors before calling
// [connector] to complete the TCP handshake and get the TCP conn. This is so that clients will see that the
// TCP connection was refused, instead of seeing a successful TCP dial that is immediately closed.
type tcpHandler interface {
	handleTCPConnector(ctx context.Context, localPort uint16, connector func() (net.Conn, error)) error
}

// udpHandler defines the behavior for handling UDP connections from VNet.
type udpHandler interface {
	HandleUDP(context.Context, net.Conn) error
}

// tunDevice abstracts a virtual network TUN device.
type tunDevice interface {
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

// networkStack implements the TCP and UDP networking stack for VNet.
type networkStack struct {
	// stack is the gVisor networking stack.
	stack *stack.Stack

	// tun is the OS TUN device. Incoming IP/L3 packets will be copied from here to [linkEndpoint], and
	// outgoing packets from [linkEndpoint] will be written here.
	tun tunDevice

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
	tcpHandlerResolver tcpHandlerResolver
	// resolveHandlerGroup is a [singleflight.Group] that will be used to avoid resolving the same FQDN
	// multiple times concurrently. Every call to [tcpHandlerResolver.ResolveTCPHandler] will be wrapped by
	// this. The key will be the FQDN.
	resolveHandlerGroup singleflight.Group

	// destroyed is a channel that will be closed when the VNet is in the process of being destroyed.
	// All goroutines should terminate quickly after either this is closed or the context passed to
	// [networkStack.Run] is canceled.
	destroyed chan struct{}
	// wg is a [sync.WaitGroup] that keeps track of all running goroutines started by the [networkStack].
	wg sync.WaitGroup

	// state holds all mutable state for the networkStack.
	state state

	slog *slog.Logger
}

type state struct {
	// mu is a single mutex that protects the whole state struct. This could be optimized as necessary.
	mu sync.RWMutex

	// Each app gets assigned both an IPv4 address and an IPv6 address, where the 4-bit suffix of the IPv6
	// matches the IPv4 address exactly. All per-app state references the smaller IPv4 address only and
	// lookups based on an IPv6 address can use the 4-byte suffix.

	// tcpHandlers holds the map of IP addresses to assigned TCP handlers.
	tcpHandlers map[ipv4]tcpHandler
	// appIPs holds the map of app FQDNs to their assigned IP address, it like a reverse map of [tcpHandlers].
	appIPs map[string]ipv4

	// udpHandlers holds the map of IP addresses to assigned UDP handlers.
	udpHandlers map[ipv4]udpHandler
}

func newState() state {
	return state{
		tcpHandlers: make(map[ipv4]tcpHandler),
		udpHandlers: make(map[ipv4]udpHandler),
		appIPs:      make(map[string]ipv4),
	}
}

// newNetworkStack creates a new VNet network stack with the given configuration and root context.
// It takes ownership of [cfg.TUNDevice] and will handle closing it before Run() returns. Call Run()
// on the returned network stack to start the VNet.
func newNetworkStack(cfg *networkStackConfig) (*networkStack, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
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

	ns := &networkStack{
		tun:                cfg.tunDevice,
		stack:              stack,
		linkEndpoint:       linkEndpoint,
		ipv6Prefix:         cfg.ipv6Prefix,
		tcpHandlerResolver: cfg.tcpHandlerResolver,
		destroyed:          make(chan struct{}),
		state:              newState(),
		slog:               slog,
	}

	tcpForwarder := tcp.NewForwarder(ns.stack, tcpReceiveBufferSize, maxInFlightTCPConnectionAttempts, ns.handleTCP)
	ns.stack.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpForwarder.HandlePacket)

	udpForwarder := udp.NewForwarder(ns.stack, ns.handleUDP)
	ns.stack.SetTransportProtocolHandler(udp.ProtocolNumber, udpForwarder.HandlePacket)

	if cfg.dnsIPv6 != (tcpip.Address{}) {
		upstreamNameserverSource := cfg.upstreamNameserverSource
		if upstreamNameserverSource == nil {
			upstreamNameserverSource, err = dns.NewOSUpstreamNameserverSource()
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		dnsServer, err := dns.NewServer(ns, upstreamNameserverSource)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := ns.assignUDPHandler(cfg.dnsIPv6, dnsServer); err != nil {
			return nil, trace.Wrap(err)
		}
		slog.DebugContext(context.Background(), "Serving DNS on IPv6.", "dns_addr", cfg.dnsIPv6)
	}

	return ns, nil
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

// run starts the VNet networking stack. It blocks until [ctx] is canceled, at
// which point it closes the link endpoint, waits for all goroutines to
// terminate, and destroys the networking stack.
func (ns *networkStack) run(ctx context.Context) error {
	ns.slog.InfoContext(ctx, "Running Teleport VNet.", "ipv6_prefix", ns.ipv6Prefix)

	ctx, cancel := context.WithCancel(ctx)

	allErrors := make(chan error, 2)
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		// Make sure to cancel the context in case this exits prematurely with a nil error.
		defer cancel()
		err := forwardBetweenTunAndNetstack(ctx, ns.tun, ns.linkEndpoint)
		allErrors <- err
		return err
	})
	g.Go(func() error {
		// When the context is canceled for any reason (the caller or one of the other concurrent tasks may
		// have canceled it) destroy everything and quit.
		<-ctx.Done()

		// In-flight connections should start terminating after closing [ns.destroyed].
		close(ns.destroyed)

		// Close the link endpoint and the TUN, this should cause [forwardBetweenTunAndNetstack] to terminate
		// if it hasn't already.
		ns.linkEndpoint.Close()
		err := trace.Wrap(ns.tun.Close(), "closing TUN device")

		allErrors <- err
		return err
	})

	// Deliberately ignoring the error from g.Wait() to return an aggregate of all errors.
	_ = g.Wait()

	// Wait for all connections and goroutines to clean themselves up.
	ns.wg.Wait()

	// Now we can destroy the gVisor networking stack and wait for all its goroutines to terminate.
	ns.stack.Destroy()

	close(allErrors)
	return trace.NewAggregateFromChannel(allErrors, context.Background())
}

func (ns *networkStack) handleTCP(req *tcp.ForwarderRequest) {
	// Add 1 to the waitgroup because the networking stack runs this in its own goroutine.
	ns.wg.Add(1)
	defer ns.wg.Done()

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
	slog := ns.slog.With("request", id)
	slog.DebugContext(ctx, "Handling TCP connection.")
	defer slog.DebugContext(ctx, "Finished handling TCP connection.")

	handler, ok := ns.getTCPHandler(id.LocalAddress)
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

		ns.wg.Add(1)
		go func() {
			defer func() {
				cancel()
				conn.Close()
				ns.wg.Done()
			}()
			select {
			case <-notifyCh:
				slog.DebugContext(ctx, "Got HUP or ERR, canceling request context and closing TCP conn.")
			case <-ns.destroyed:
				slog.DebugContext(ctx, "VNet is being destroyed, canceling request context and closing TCP conn.")
			case <-ctx.Done():
				slog.DebugContext(ctx, "Request context canceled, closing TCP conn.")
			}
		}()

		return conn, nil
	}

	if err := handler.handleTCPConnector(ctx, id.LocalPort, connector); err != nil {
		if errors.Is(err, context.Canceled) {
			slog.DebugContext(ctx, "TCP connection handler returned early due to canceled context.")
		} else {
			slog.DebugContext(ctx, "Error handling TCP connection.", "err", err)
		}
	}
}

func (ns *networkStack) getTCPHandler(addr tcpip.Address) (tcpHandler, bool) {
	ns.state.mu.RLock()
	defer ns.state.mu.RUnlock()
	handler, ok := ns.state.tcpHandlers[ipv4Suffix(addr)]
	return handler, ok
}

// assignTCPHandler assigns an IPv4 address to [handlerSpec] from its preferred CIDR range, and returns that
// new assigned address.
func (ns *networkStack) assignTCPHandler(handlerSpec *tcpHandlerSpec, fqdn string) (ipv4, error) {
	_, ipNet, err := net.ParseCIDR(handlerSpec.ipv4CIDRRange)
	if err != nil {
		return 0, trace.Wrap(err, "parsing CIDR %q", handlerSpec.ipv4CIDRRange)
	}

	ns.state.mu.Lock()
	defer ns.state.mu.Unlock()

	ip, err := randomFreeIPv4InNet(ipNet, func(ip ipv4) bool {
		_, taken := ns.state.tcpHandlers[ip]
		return !taken
	})
	if err != nil {
		return 0, trace.Wrap(err, "assigning IP address")
	}

	ns.state.tcpHandlers[ip] = handlerSpec.tcpHandler
	ns.state.appIPs[fqdn] = ip

	if err := ns.addProtocolAddress(tcpip.AddrFrom4(ip.asArray())); err != nil {
		return 0, trace.Wrap(err)
	}
	if err := ns.addProtocolAddress(ipv6WithSuffix(ns.ipv6Prefix, ip.asSlice())); err != nil {
		return 0, trace.Wrap(err)
	}

	return ip, nil
}

func (ns *networkStack) handleUDP(req *udp.ForwarderRequest) {
	ns.wg.Add(1)
	go func() {
		defer ns.wg.Done()
		ns.handleUDPConcurrent(req)
	}()
}

func (ns *networkStack) handleUDPConcurrent(req *udp.ForwarderRequest) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	id := req.ID()
	slog := ns.slog.With("request", id)
	slog.DebugContext(ctx, "Handling UDP request.")
	defer slog.DebugContext(ctx, "Finished handling UDP request.")

	handler, ok := ns.getUDPHandler(id.LocalAddress)
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

	conn := gonet.NewUDPConn(ns.stack, &wq, endpoint)
	defer conn.Close()

	ns.wg.Add(1)
	go func() {
		defer func() {
			cancel()
			conn.Close()
			ns.wg.Done()
		}()
		select {
		case <-notifyCh:
			slog.DebugContext(ctx, "Got HUP or ERR, canceling request context and closing UDP conn.")
		case <-ns.destroyed:
			slog.DebugContext(ctx, "VNet is being destroyed, canceling request context and closing UDP conn.")
		case <-ctx.Done():
			slog.DebugContext(ctx, "Request context canceled, closing UDP conn.")
		}
	}()

	if err := handler.HandleUDP(ctx, conn); err != nil {
		slog.DebugContext(ctx, "Error handling UDP conn.", "error", err)
	}
}

func (ns *networkStack) getUDPHandler(addr tcpip.Address) (udpHandler, bool) {
	ipv4 := ipv4Suffix(addr)
	ns.state.mu.RLock()
	defer ns.state.mu.RUnlock()
	handler, ok := ns.state.udpHandlers[ipv4]
	return handler, ok
}

func (ns *networkStack) assignUDPHandler(addr tcpip.Address, handler udpHandler) error {
	ipv4 := ipv4Suffix(addr)
	ns.state.mu.Lock()
	defer ns.state.mu.Unlock()
	if _, ok := ns.state.udpHandlers[ipv4]; ok {
		return trace.AlreadyExists("Handler for %s is already set", addr)
	}
	if err := ns.addProtocolAddress(addr); err != nil {
		return trace.Wrap(err)
	}
	ns.state.udpHandlers[ipv4] = handler
	return nil
}

// ResolveA implements [dns.Resolver.ResolveA].
func (ns *networkStack) ResolveA(ctx context.Context, fqdn string) (dns.Result, error) {
	// Do the actual resolution within a [singleflight.Group] keyed by [fqdn] to avoid concurrent requests to
	// resolve an FQDN and then assign an address to it.
	resultAny, err, _ := ns.resolveHandlerGroup.Do(fqdn, func() (any, error) {
		// If we've already assigned an IP address to this app, resolve to it.
		if ip, ok := ns.appIPv4(fqdn); ok {
			return dns.Result{
				A: ip.asArray(),
			}, nil
		}

		// If fqdn is a Teleport-managed app, create a new handler for it.
		handlerSpec, err := ns.tcpHandlerResolver.resolveTCPHandler(ctx, fqdn)
		if err != nil {
			if errors.Is(err, errNoTCPHandler) {
				// Did not find any known app, forward the DNS request upstream.
				return dns.Result{}, nil
			}
			return dns.Result{}, trace.Wrap(err, "resolving TCP handler for fqdn %q", fqdn)
		}

		// Assign an unused IP address to this app's handler.
		ip, err := ns.assignTCPHandler(handlerSpec, fqdn)
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
func (ns *networkStack) ResolveAAAA(ctx context.Context, fqdn string) (dns.Result, error) {
	result, err := ns.ResolveA(ctx, fqdn)
	if err != nil {
		return dns.Result{}, trace.Wrap(err)
	}
	if result.A != ([4]byte{}) {
		result.AAAA = ipv6WithSuffix(ns.ipv6Prefix, result.A[:]).As16()
		result.A = [4]byte{}
	}
	return result, nil
}

func (ns *networkStack) appIPv4(fqdn string) (ipv4, bool) {
	ns.state.mu.RLock()
	defer ns.state.mu.RUnlock()
	ipv4, ok := ns.state.appIPs[fqdn]
	return ipv4, ok
}

func forwardBetweenTunAndNetstack(ctx context.Context, tun tunDevice, linkEndpoint *channel.Endpoint) error {
	slog.DebugContext(ctx, "Forwarding IP packets between OS and VNet.")
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return forwardNetstackToTUN(ctx, linkEndpoint, tun) })
	g.Go(func() error { return forwardTUNtoNetstack(ctx, tun, linkEndpoint) })
	return trace.Wrap(g.Wait())
}

func forwardNetstackToTUN(ctx context.Context, linkEndpoint *channel.Endpoint, tun tunDevice) error {
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

// forwardTUNtoNetstack does not abort on ctx being canceled, but it does check the ctx error before
// returning os.ErrClosed from tun.Read.
func forwardTUNtoNetstack(ctx context.Context, tun tunDevice, linkEndpoint *channel.Endpoint) error {
	const readOffset = device.MessageTransportHeaderSize
	bufs := make([][]byte, tun.BatchSize())
	for i := range bufs {
		bufs[i] = make([]byte, device.MessageTransportHeaderSize+mtu)
	}
	sizes := make([]int, len(bufs))
	for {
		n, err := tun.Read(bufs, sizes, readOffset)
		if err != nil {
			// tun.Read might get interrupted due to the TUN device getting closed after ctx cancellation.
			if errors.Is(err, os.ErrClosed) && ctx.Err() != nil {
				return ctx.Err()
			}
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

func (ns *networkStack) addProtocolAddress(localAddress tcpip.Address) error {
	protocolAddress, err := protocolAddress(localAddress)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := ns.stack.AddProtocolAddress(nicID, protocolAddress, stack.AddressProperties{}); err != nil {
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

// NetworkStackInfo is used to pass information about some aspects of the network stack outside
// of the goroutine or the process that manages [networkStack].
type NetworkStackInfo struct {
	// IfaceName is the name of the interface used by VNet.
	IfaceName string
}
