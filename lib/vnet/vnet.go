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
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/davecgh/go-spew/spew"
	"github.com/gravitational/trace"
	"github.com/vulcand/predicate/builder"
	"golang.org/x/sync/errgroup"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/vnet/dns"
)

const (
	nicID = 1
	mtu   = 1500
)

var (
	defaultDNSAddress = tcpip.AddrFrom4([4]byte{100, 127, 100, 127})
)

func BaseIPv6Address() (tcpip.Address, error) {
	// |   8 bits   |  40 bits   |  16 bits  |          64 bits           |
	// +------------+------------+-----------+----------------------------+
	// | ULA Prefix | Global ID  | Subnet ID |        Interface ID        |
	// +------------+------------+-----------+----------------------------+
	// ULA Prefix is always 0xfd
	// Global ID is random bytes
	// Subnet ID is always 0
	// Interface ID will be the IPv4 address prefixed with zeros.
	var bytes [16]byte
	bytes[0] = 0xfd
	if _, err := rand.Read(bytes[1:6]); err != nil {
		return tcpip.Address{}, trace.Wrap(err)
	}
	return tcpip.AddrFrom16(bytes), nil
}

// Run is a blocking call to create and start Teleport VNet.
func Run(ctx context.Context, tc *client.TeleportClient, customDNSZones []string) error {
	baseAddress, err := BaseIPv6Address()
	if err != nil {
		return trace.Wrap(err)
	}

	tun, cleanup, err := CreateAndSetupTUNDevice(ctx, baseAddress.String(), customDNSZones)
	if err != nil {
		return trace.Wrap(err)
	}
	defer cleanup()

	manager, err := NewManager(ctx, &Config{
		Client:          tc,
		TUNDevice:       tun,
		BaseIPv6Address: baseAddress,
		customDNSZones:  customDNSZones,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if err := manager.Run(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Config holds configuration parameters for the VNet.
type Config struct {
	Client          *client.TeleportClient
	TUNDevice       tun.Device
	DNSAddress      tcpip.Address
	BaseIPv6Address tcpip.Address
	customDNSZones  []string
}

// CheckAndSetDefaults checks the config and sets defaults.
func (c *Config) CheckAndSetDefaults() error {
	if c.Client == nil {
		return trace.BadParameter("client is required")
	}
	if c.TUNDevice == nil {
		return trace.BadParameter("TUN device is required")
	}
	if c.DNSAddress == (tcpip.Address{}) {
		c.DNSAddress = defaultDNSAddress
	}
	return nil
}

type tcpConnector func() (io.ReadWriteCloser, error)
type tcpHandler interface {
	handleTCP(context.Context, tcpConnector) error
}

type udpHandler func(context.Context, io.ReadWriteCloser) error

type state struct {
	mu          sync.RWMutex
	tcpHandlers map[tcpip.Address]tcpHandler
	udpHandlers map[tcpip.Address]udpHandler
	ips         map[string]tcpip.Address
	nextFreeIP  uint32
}

func newState() state {
	return state{
		tcpHandlers: make(map[tcpip.Address]tcpHandler),
		udpHandlers: make(map[tcpip.Address]udpHandler),
		ips:         make(map[string]tcpip.Address),
		nextFreeIP:  uint32(100<<24 + 64<<16 + 0<<8 + 2<<0),
	}
}

// Manager holds configuration and state for the VNet.
type Manager struct {
	tc            *client.TeleportClient
	tun           tun.Device
	dnsAddress    tcpip.Address
	stack         *stack.Stack
	rootCtx       context.Context
	rootCtxCancel context.CancelFunc
	dnsServer     *dns.Server
	slog          *slog.Logger
	state         state
	// TODO: remove this and get custom DNS zones per cluster.
	globalCustomDNSZones []string
	baseIPv6Address      tcpip.Address
}

// NewManager creates a new VNet manager with the given configuration and root
// context. Call Run() on the returned manager to start the VNet.
func NewManager(ctx context.Context, cfg *Config) (*Manager, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	stack, err := createStack()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	slog := slog.With(teleport.ComponentKey, "VNet")
	ctx, cancel := context.WithCancel(ctx)
	m := &Manager{
		tc:                   cfg.Client,
		tun:                  cfg.TUNDevice,
		dnsAddress:           cfg.DNSAddress,
		stack:                stack,
		rootCtx:              ctx,
		rootCtxCancel:        cancel,
		slog:                 slog,
		state:                newState(),
		globalCustomDNSZones: cfg.customDNSZones,
		baseIPv6Address:      cfg.BaseIPv6Address,
	}

	dnsServer, err := dns.NewServer(slog, m)
	if err != nil {
		return nil, trace.Wrap(err, "creating DNS server")
	}
	m.dnsServer = dnsServer
	m.state.udpHandlers[cfg.DNSAddress] = dnsServer.HandleUDPConn
	return m, nil
}

// Run starts the VNet.
func (m *Manager) Run() error {
	defer m.rootCtxCancel()

	const (
		// TODO(nklaassen): Figure out optimal values for these.
		tcpReceiveBufferSize          = 0 // 0 means a default will be used.
		maxInFlightConnectionAttempts = 1024
	)
	tcpForwarder := tcp.NewForwarder(m.stack, tcpReceiveBufferSize, maxInFlightConnectionAttempts, m.handleTCP)
	udpForwarder := udp.NewForwarder(m.stack, m.handleUDP)
	m.stack.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpForwarder.HandlePacket)
	m.stack.SetTransportProtocolHandler(udp.ProtocolNumber, udpForwarder.HandlePacket)
	const (
		size     = 512
		linkAddr = ""
	)
	linkEndpoint := channel.New(size, mtu, linkAddr)
	if err := m.stack.CreateNIC(nicID, linkEndpoint); err != nil {
		return trace.Errorf("creating VNet NIC: %s", err)
	}
	// Make the NIC accept all IP packets on the VNet, regardless of destination
	// address.
	m.stack.SetPromiscuousMode(nicID, true)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	go func() {
		for {
			select {
			case <-m.rootCtx.Done():
				return
			case <-ch:
			}
			stats := m.stack.Stats()
			spew.Dump(stats)
		}
	}()

	return trace.Wrap(forwardBetweenOsAndVnet(m.rootCtx, m.tun, linkEndpoint))
}

// Close closes all connections, destroys the networking stack and closes the TUN device.
func (m *Manager) Close() error {
	m.rootCtxCancel()
	m.stack.Destroy()
	return trace.Wrap(m.tun.Close())
}

// ResolveA implements [dns.Resolver.ResolveA]
func (m *Manager) ResolveA(ctx context.Context, fqdn string) (dns.Result, error) {
	if ipv6, ok := m.cachedIPv6(fqdn); ok {
		return dns.Result{
			A: ipv4Suffix(ipv6).As4(),
		}, nil
	}

	appPublicAddr := strings.TrimSuffix(fqdn, ".")
	matchingProfile, ok, err := m.matchingProfile(appPublicAddr)
	if err != nil {
		return dns.Result{}, trace.Wrap(err)
	}
	if !ok {
		// No matching profile, forward the request.
		return dns.Result{}, nil
	}

	app, match, err := m.matchingAppForProfile(ctx, matchingProfile, appPublicAddr)
	if err != nil {
		return dns.Result{}, trace.Wrap(err)
	}
	if !match {
		// The app wasn't found, forward the request to the default nameservers.
		return dns.Result{}, nil
	}

	ipv4, _, err := m.assignIPsToApp(fqdn, app)
	if err != nil {
		return dns.Result{}, trace.Wrap(err)
	}

	return dns.Result{
		A: ipv4.As4(),
	}, nil
}

// ResolveAAAA implements [dns.Resolver.ResolveAAAA]
func (m *Manager) ResolveAAAA(ctx context.Context, fqdn string) (dns.Result, error) {
	if ip, ok := m.cachedIPv6(fqdn); ok {
		return dns.Result{
			AAAA: ip.As16(),
		}, nil
	}

	appPublicAddr := strings.TrimSuffix(fqdn, ".")
	matchingProfile, ok, err := m.matchingProfile(appPublicAddr)
	if err != nil {
		return dns.Result{}, trace.Wrap(err)
	}
	if !ok {
		// No matching profile, forward the request.
		return dns.Result{}, nil
	}

	app, match, err := m.matchingAppForProfile(ctx, matchingProfile, appPublicAddr)
	if err != nil {
		return dns.Result{}, trace.Wrap(err)
	}
	if !match {
		// The app wasn't found, forward the request to the default nameservers.
		return dns.Result{}, nil
	}

	_, ipv6, err := m.assignIPsToApp(fqdn, app)
	if err != nil {
		return dns.Result{}, trace.Wrap(err)
	}

	return dns.Result{
		AAAA: ipv6.As16(),
	}, nil
}

func (m *Manager) matchingProfile(appPublicAddr string) (string, bool, error) {
	profiles, err := m.tc.ClientStore.ListProfiles()
	if err != nil {
		return "", false, trace.Wrap(err, "listing user profiles")
	}
	for _, profile := range profiles {
		dnsZone := fmt.Sprintf(".%s", profile)
		if strings.HasSuffix(appPublicAddr, dnsZone) {
			return profile, true, nil
		}
		for _, customZone := range m.globalCustomDNSZones {
			if strings.HasSuffix(appPublicAddr, customZone) {
				return profile, true, nil
			}
		}
	}
	return "", false, nil
}

func (m *Manager) matchingAppForProfile(ctx context.Context, profileName, appPublicAddr string) (types.Application, bool, error) {
	// TODO(nklaassen): support leaf clusters
	clt, err := m.apiClient(ctx, profileName)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}
	appServers, err := apiclient.GetAllResources[types.AppServer](ctx, clt, &proto.ListResourcesRequest{
		ResourceType: types.KindAppServer,
		PredicateExpression: builder.Equals(
			builder.Identifier("resource.spec.public_addr"),
			builder.String(appPublicAddr),
		).String(),
	})
	if err != nil {
		return nil, false, trace.Wrap(err, "listing application servers")
	}
	for _, appServer := range appServers {
		app := appServer.GetApp()
		if app.GetPublicAddr() == appPublicAddr && app.IsTCP() {
			return app, true, nil
		}
	}
	return nil, false, nil
}

func (m *Manager) apiClient(ctx context.Context, profileName string) (*apiclient.Client, error) {
	// TODO(nklaassen): reuse api clients
	profile, err := m.tc.ClientStore.GetProfile(profileName)
	if err != nil {
		return nil, trace.Wrap(err, "loading user profile")
	}
	creds := apiclient.LoadProfile(os.Getenv("TELEPORT_HOME"), profileName)
	return apiclient.New(ctx, apiclient.Config{
		Addrs:       []string{profile.WebProxyAddr},
		Credentials: []apiclient.Credentials{creds},
		Context:     m.rootCtx,
	})
}

func (m *Manager) assignIPsToApp(fqdn string, app types.Application) (ipv4 tcpip.Address, ipv6 tcpip.Address, err error) {
	appHandler, err := newAppHandler(m.tc, app)
	if err != nil {
		return ipv4, ipv6, trace.Wrap(err)
	}

	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	ip := m.state.nextFreeIP
	m.state.nextFreeIP += 1

	ipv4 = tcpip.AddrFrom4([4]byte{byte(ip >> 24), byte(ip >> 16), byte(ip >> 8), byte(ip >> 0)})
	ipv6Bytes := m.baseIPv6Address.As16()
	copy(ipv6Bytes[12:16], ipv4.AsSlice())
	ipv6 = tcpip.AddrFrom16(ipv6Bytes)

	m.state.tcpHandlers[ipv4] = appHandler
	m.state.tcpHandlers[ipv6] = appHandler
	m.state.ips[fqdn] = ipv6

	return ipv4, ipv6, nil
}

func (m *Manager) cachedIPv6(fqdn string) (tcpip.Address, bool) {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	ipv6, ok := m.state.ips[fqdn]
	return ipv6, ok
}

func ipv4Suffix(ipv6 tcpip.Address) tcpip.Address {
	return tcpip.AddrFromSlice(ipv6.AsSlice()[12:16])
}

func (m *Manager) tcpHandler(addr tcpip.Address, port uint16) (tcpHandler, bool) {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	handler, ok := m.state.tcpHandlers[addr]
	return handler, ok
}

func (m *Manager) udpHandler(addr tcpip.Address, port uint16) (udpHandler, bool) {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()
	handler, ok := m.state.udpHandlers[addr]
	return handler, ok
}

func protocolAddress(addr tcpip.Address) (tcpip.ProtocolAddress, error) {
	addrWithPrefix := addr.WithPrefix()
	var protocol tcpip.NetworkProtocolNumber
	switch addrWithPrefix.PrefixLen {
	case 32:
		protocol = ipv4.ProtocolNumber
	case 128:
		protocol = ipv6.ProtocolNumber
	default:
		return tcpip.ProtocolAddress{}, trace.BadParameter("unhandled prefx len %d", addrWithPrefix.PrefixLen)
	}
	return tcpip.ProtocolAddress{
		AddressWithPrefix: addrWithPrefix,
		Protocol:          protocol,
	}, nil
}

func (m *Manager) handleTCP(req *tcp.ForwarderRequest) {
	ctx, cancel := context.WithCancel(m.rootCtx)
	defer cancel()
	id := req.ID()
	slog := m.slog.With("request_id", id)
	slog.Debug("Got TCP forward request.")
	defer slog.Debug("Finished TCP forward request.")

	// Add the address to the NIC so that the gvisor stack routes packets back
	// out to the host. Seems fine to call multiple times for same IP.
	protocolAddress, err := protocolAddress(id.LocalAddress)
	if err != nil {
		slog.With("error", err).Debug("Failed to construct protocol address.")
		return
	}
	if err := m.stack.AddProtocolAddress(nicID, protocolAddress, stack.AddressProperties{}); err != nil {
		slog.With("error", err).Debug("Failed to add protocol address to netstack.")
		return
	}

	handler, ok := m.tcpHandler(id.LocalAddress, id.LocalPort)
	if !ok {
		slog.Debug("No handler for address.", "addr", id.LocalAddress)
		req.Complete(true) // Send RST
		return
	}

	var wq waiter.Queue
	waitEntry, notifyCh := waiter.NewChannelEntry(waiter.EventHUp)
	wq.EventRegister(&waitEntry)
	defer wq.EventUnregister(&waitEntry)
	go func() {
		select {
		case <-notifyCh:
			slog.Debug("Got HUP, canceling context.")
			cancel()
		case <-ctx.Done():
		}
	}()

	completed := false
	connector := func() (io.ReadWriteCloser, error) {
		endpoint, err := req.CreateEndpoint(&wq)
		if err != nil {
			// This err doesn't actually implement [error]
			return nil, trace.Errorf("creating TCP endpoint: %s", err)
		}
		req.Complete(false)
		completed = true
		endpoint.SocketOptions().SetKeepAlive(true)
		conn := gonet.NewTCPConn(&wq, endpoint)
		return conn, nil
	}

	if err := handler.handleTCP(ctx, connector); err != nil {
		if errors.Is(err, context.Canceled) {
			slog.Debug("TCP connection handler returned early due to canceled context.")
		} else {
			slog.Debug("Error handling TCP connection.", "err", err)
		}
	}
	if !completed {
		// Handler did not consume the connector.
		req.Complete(true) // Send RST
	}
}

func (m *Manager) handleUDP(req *udp.ForwarderRequest) {
	go m.handleUDPConcurrent(req)
}

func (m *Manager) handleUDPConcurrent(req *udp.ForwarderRequest) {
	ctx, cancel := context.WithCancel(m.rootCtx)
	defer cancel()
	id := req.ID()
	slog := m.slog.With("request_id", id)
	slog.Debug("Got UDP forward request.")
	defer slog.Debug("Finished UDP forward request.")

	handler, ok := m.udpHandler(id.LocalAddress, id.LocalPort)
	if !ok {
		slog.Debug("No handler for address.", "addr", id.LocalAddress)
		return
	}

	// Add the address to the NIC so that the VNet routes packets back out
	// to the host. Seems fine to call multiple times for same IP.
	m.stack.AddProtocolAddress(nicID, tcpip.ProtocolAddress{
		AddressWithPrefix: id.LocalAddress.WithPrefix(),
		Protocol:          ipv4.ProtocolNumber, // TODO(nklaassen): Support IPv6
	}, stack.AddressProperties{})

	var wq waiter.Queue
	ep, err := req.CreateEndpoint(&wq)
	if err != nil {
		slog.Warn("Failed to create endpoint.", "err", err)
		return
	}

	conn := gonet.NewUDPConn(m.stack, &wq, ep)
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	if err := handler(ctx, conn); err != nil {
		slog.Debug("Error handling UDP conn.", "err", err)
	}
}

func createStack() (*stack.Stack, error) {
	netStack := stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol, ipv6.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol},
	})

	// Route everything to the one NIC.
	ipv4Subnet, err := tcpip.NewSubnet(tcpip.AddrFrom4([4]byte{}), tcpip.MaskFromBytes(make([]byte, 4)))
	if err != nil {
		return nil, trace.Wrap(err, "creating VNet IPv4 subnet")
	}
	ipv6Subnet, err := tcpip.NewSubnet(tcpip.AddrFrom16([16]byte{}), tcpip.MaskFromBytes(make([]byte, 16)))
	if err != nil {
		return nil, trace.Wrap(err, "creating VNet IPv6 subnet")
	}
	netStack.SetRouteTable([]tcpip.Route{
		{
			Destination: ipv4Subnet,
			NIC:         nicID,
		},
		{
			Destination: ipv6Subnet,
			NIC:         nicID,
		},
	})
	return netStack, nil
}

func forwardBetweenOsAndVnet(ctx context.Context, osTUN tun.Device, vnetEndpoint *channel.Endpoint) error {
	slog.Debug("Started forwarding packets between OS and VNet.")
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return forwardVnetEndpointToOsTUN(ctx, vnetEndpoint, osTUN) })
	g.Go(func() error { return forwardOsTUNToVnetEndpoint(ctx, osTUN, vnetEndpoint) })
	g.Go(func() error {
		<-ctx.Done()
		osTUN.Close()
		vnetEndpoint.Close()
		return nil
	})
	return trace.Wrap(g.Wait())
}

func forwardVnetEndpointToOsTUN(ctx context.Context, endpoint *channel.Endpoint, tun tun.Device) error {
	bufs := [][]byte{make([]byte, device.MessageTransportHeaderSize+mtu)}
	for {
		packet := endpoint.ReadContext(ctx)
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

func forwardOsTUNToVnetEndpoint(ctx context.Context, tun tun.Device, dstEndpoint *channel.Endpoint) error {
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
			dstEndpoint.InjectInbound(protocol, packet)
			packet.DecRef()
		}
	}
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
