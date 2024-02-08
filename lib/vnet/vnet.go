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
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	"github.com/davecgh/go-spew/spew"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/vnet/dns"
)

const (
	nicID            = 1
	mtu              = 1500 // TODO: Find optimal MTU.
	defaultDNSSuffix = ".teleport.private."
)

var (
	defaultDNSAddress = tcpip.AddrFrom4([4]byte{100, 127, 100, 127})
)

// Run is a blocking call to create and start Teleport VNet.
func Run(ctx context.Context, tc *client.TeleportClient) error {
	tun, err := CreateAndSetupTUNDevice(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	manager, err := NewManager(ctx, &Config{
		Client:    tc,
		TUNDevice: tun,
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
	Client     *client.TeleportClient
	TUNDevice  tun.Device
	DNSAddress tcpip.Address
	DNSSuffix  string
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
	if c.DNSSuffix == "" {
		c.DNSSuffix = defaultDNSSuffix
	}
	return nil
}

type tcpConnector func() (io.ReadWriteCloser, error)
type tcpHandler func(context.Context, tcpConnector) error

type udpHandler func(context.Context, io.ReadWriteCloser) error

type state struct {
	// Canonical data
	apps []types.Application

	// Denormalized data optimized for indexing
	tcpHandlers map[tcpip.Address]tcpHandler
	udpHandlers map[tcpip.Address]udpHandler
	ips         map[string]tcpip.Address
}

// Manager holds configuration and state for the VNet.
type Manager struct {
	tc            *client.TeleportClient
	tun           tun.Device
	dnsSuffix     string
	dnsAddress    tcpip.Address
	stack         *stack.Stack
	rootCtx       context.Context
	rootCtxCancel context.CancelFunc
	dnsServer     *dns.Server
	slog          *slog.Logger
	state         state
	mu            sync.RWMutex
}

// NewManager creates a new VNet manager with the given configuration and root
// context. Call Run() on the returned manager to start the VNet.
func NewManager(ctx context.Context, cfg *Config) (m *Manager, err error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	stack, err := createStack()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	slog := slog.With(trace.Component, "VNet")
	dnsServer := dns.NewServer(slog)
	ctx, cancel := context.WithCancel(ctx)
	return &Manager{
		tc:            cfg.Client,
		tun:           cfg.TUNDevice,
		dnsSuffix:     cfg.DNSSuffix,
		dnsAddress:    cfg.DNSAddress,
		stack:         stack,
		rootCtx:       ctx,
		rootCtxCancel: cancel,
		slog:          slog,
		dnsServer:     dnsServer,
	}, nil
}

// Run starts the VNet.
// TODO: Accept ctx instead of saving rootCtx. Or maybe not? How do we stop vnet if we shouldn't be
// storing contexts as fields on structs?
func (m *Manager) Run() error {
	defer m.rootCtxCancel()

	const (
		// TODO: Figure out optimal values for these.
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

	if err := m.refreshState(m.rootCtx); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(forwardBetweenOsAndVnet(m.rootCtx, m.tun, linkEndpoint))
}

func (m *Manager) tcpHandler(addr tcpip.Address) (tcpHandler, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	handler, ok := m.state.tcpHandlers[addr]
	return handler, ok
}

func (m *Manager) udpHandler(addr tcpip.Address) (udpHandler, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	handler, ok := m.state.udpHandlers[addr]
	return handler, ok
}

func (m *Manager) handleTCP(req *tcp.ForwarderRequest) {
	ctx, cancel := context.WithCancel(m.rootCtx)
	defer cancel()
	slog := m.slog.With("request_id", req.ID())
	slog.Debug("Got TCP forward request.")
	defer slog.Debug("Finished TCP forward request.")

	// Add the address to the NIC so that the VNet routes packets back out
	// to the host. Seems fine to call multiple times for same IP.
	m.stack.AddProtocolAddress(nicID, tcpip.ProtocolAddress{
		AddressWithPrefix: req.ID().LocalAddress.WithPrefix(),
		Protocol:          ipv4.ProtocolNumber, // TODO: Support IPv6
	}, stack.AddressProperties{})

	handler, ok := m.tcpHandler(req.ID().LocalAddress)
	if !ok {
		slog.Debug("No handler for address.", "addr", req.ID().LocalAddress)
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
			slog.Debug("Got HUP, cancelling context.")
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

	if err := handler(ctx, connector); err != nil {
		slog.Debug("Error handling TCP connection.", "err", err)
	}
	if !completed {
		// Handler did not consume the connector.
		req.Complete(true) // Send RST
	}
}

func (m *Manager) handleUDP(req *udp.ForwarderRequest) {
	ctx, cancel := context.WithCancel(m.rootCtx)
	defer cancel()
	slog := m.slog.With("request_id", req.ID())
	slog.Debug("Got UDP forward request.")
	defer slog.Debug("Finished UDP forward request.")

	handler, ok := m.udpHandler(req.ID().LocalAddress)
	if !ok {
		slog.Debug("No handler for address.", "addr", req.ID().LocalAddress)
		return
	}

	// Add the address to the NIC so that the VNet routes packets back out
	// to the host. Seems fine to call multiple times for same IP.
	m.stack.AddProtocolAddress(nicID, tcpip.ProtocolAddress{
		AddressWithPrefix: req.ID().LocalAddress.WithPrefix(),
		Protocol:          ipv4.ProtocolNumber, // TODO: Support IPv6
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
		slog.Debug("Context cancelling, closing UDP conn.")
		conn.Close()
	}()
	if err := handler(ctx, conn); err != nil {
		slog.Debug("Error handling UDP conn.", "err", err)
	}
}

func (m *Manager) refreshState(ctx context.Context) error {
	apps, err := m.tc.ListApps(ctx, nil /*filters*/)
	if err != nil {
		return trace.Wrap(err)
	}

	tcpHandlers := make(map[tcpip.Address]tcpHandler, len(apps))
	var catalog dns.Catalog

	fmt.Println("\nSetting up VNet IPs for all apps:")
	table := asciitable.MakeTable([]string{"App", "DNS", "IP"})
	var nextIp uint32 = 100<<24 + 64<<16 + 0<<8 + 2
	for _, app := range apps {
		addr := tcpip.AddrFrom4([4]byte{byte(nextIp >> 24), byte(nextIp >> 16), byte(nextIp >> 8), byte(nextIp)})
		appName := app.GetName()
		appPublicAddr := app.GetPublicAddr()
		fqdn := appName + m.dnsSuffix
		table.AddRow([]string{appName, fqdn, addr.String()})
		tcpHandlers[addr] = proxyToApp(m.tc, appName, appPublicAddr)
		catalog.PushAddress(fqdn, addr)
		nextIp += 1
	}
	_, err = io.Copy(os.Stdout, table.AsBuffer())
	if err != nil {
		return trace.Wrap(err)
	}

	m.dnsServer.UpdateCatalog(catalog)

	udpHandlers := map[tcpip.Address]udpHandler{
		m.dnsAddress: m.dnsServer.HandleUDPConn,
	}
	fmt.Println("\nHosting DNS at", m.dnsAddress)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.apps = apps
	m.state.tcpHandlers = tcpHandlers
	m.state.udpHandlers = udpHandlers
	return nil
}

func createStack() (*stack.Stack, error) {
	netStack := stack.New(stack.Options{
		// TODO: IPv6
		NetworkProtocols: []stack.NetworkProtocolFactory{ipv4.NewProtocol},
		// TODO: Consider ICMP
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol},
	})

	// Route everything to the one NIC.
	// TODO: Support IPv6.
	ipv4Subnet, err := tcpip.NewSubnet(tcpip.AddrFrom4([4]byte{}), tcpip.MaskFromBytes(make([]byte, 4)))
	if err != nil {
		return nil, trace.Wrap(err, "creating VNet IPv4 subnet")
	}
	netStack.SetRouteTable([]tcpip.Route{{
		Destination: ipv4Subnet,
		NIC:         nicID,
	}})
	return netStack, nil
}

func forwardBetweenOsAndVnet(ctx context.Context, osTUN tun.Device, vnetEndpoint *channel.Endpoint) error {
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
		bufs[0] = bufs[0][:cap(bufs[0])]
		packet := endpoint.ReadContext(ctx)
		if packet.IsNil() {
			// Nil packet is returned when context is cancelled.
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
	}
}

func forwardOsTUNToVnetEndpoint(ctx context.Context, tun tun.Device, dstEndpoint *channel.Endpoint) error {
	const readOffset = device.MessageTransportHeaderSize
	buffers := make([][]byte, tun.BatchSize())
	for i := range buffers {
		buffers[i] = make([]byte, device.MessageTransportHeaderSize+mtu)
	}
	sizes := make([]int, len(buffers))
	for {
		for i := range buffers {
			buffers[i] = buffers[i][:cap(buffers[i])]
		}
		n, err := tun.Read(buffers, sizes, readOffset)
		if err != nil {
			return trace.Wrap(err, "reading packets from TUN")
		}
		for i := range sizes[:n] {
			buffers[i] = buffers[i][readOffset : readOffset+sizes[i]]
			packet := stack.NewPacketBuffer(stack.PacketBufferOptions{
				Payload: buffer.MakeWithData(buffers[i]),
			})
			dstEndpoint.InjectInbound(header.IPv4ProtocolNumber, packet)
			packet.DecRef()
		}
	}
}

func getenvInt(envVar string) (int, error) {
	s := os.Getenv(envVar)
	if s == "" {
		return 0, trace.BadParameter(envVar + " is not set")
	}
	i, err := strconv.Atoi(s)
	return i, trace.Wrap(err)

}
