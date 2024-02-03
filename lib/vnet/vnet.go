package vnet

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/davecgh/go-spew/spew"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/vnet/dns"
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
)

const (
	// TODO: Find optimal MTU.
	mtu              = 1500
	nicID            = 1
	privateDNSSuffix = ".teleport.private."
)

var (
	dnsAddr = tcpip.AddrFrom4([4]byte{100, 127, 100, 127})
)

func Run(ctx context.Context, tc *client.TeleportClient) error {
	manager, err := newManager(ctx, tc)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := manager.run(); err != nil {
		return trace.Wrap(err)
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

type manager struct {
	tc            *client.TeleportClient
	stack         *stack.Stack
	rootCtx       context.Context
	rootCtxCancel context.CancelFunc
	dnsServer     *dns.Server
	slog          *slog.Logger
	state         state
	mu            sync.RWMutex
}

func newManager(ctx context.Context, tc *client.TeleportClient) (m *manager, err error) {
	stack, err := createStack()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	slog := slog.With(trace.Component, "VNet")
	dnsServer := dns.NewServer(slog)
	ctx, cancel := context.WithCancel(ctx)
	return &manager{
		tc:            tc,
		stack:         stack,
		rootCtx:       ctx,
		rootCtxCancel: cancel,
		slog:          slog,
		dnsServer:     dnsServer,
	}, nil
}

func (m *manager) run() error {
	defer m.rootCtxCancel()

	tun, tunName, err := createTunDevice()
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		<-m.rootCtx.Done()
		if err := tun.Close(); err != nil {
			m.slog.Debug("Closing TUN device.", "err", err)
		}
	}()
	m.slog.Info("Created TUN device.", "dev", tunName)

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

	if err := setupHostIPRoutes(tunName); err != nil {
		return trace.Wrap(err, "setting up host IP routes")
	}

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

	return trace.Wrap(forwardBetweenOsAndVnet(m.rootCtx, tun, linkEndpoint))
}

func (m *manager) tcpHandler(addr tcpip.Address) (tcpHandler, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	handler, ok := m.state.tcpHandlers[addr]
	return handler, ok
}

func (m *manager) udpHandler(addr tcpip.Address) (udpHandler, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	handler, ok := m.state.udpHandlers[addr]
	return handler, ok
}

func (m *manager) handleTCP(req *tcp.ForwarderRequest) {
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

func (m *manager) handleUDP(req *udp.ForwarderRequest) {
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

func (m *manager) refreshState(ctx context.Context) error {
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
		fqdn := appName + privateDNSSuffix
		table.AddRow([]string{appName, fqdn, addr.String()})
		tcpHandlers[addr] = proxyToApp(m.tc, appName, appPublicAddr)
		catalog.PushAddress(fqdn, addr)
		nextIp += 1
	}
	io.Copy(os.Stdout, table.AsBuffer())

	m.dnsServer.UpdateCatalog(catalog)

	udpHandlers := map[tcpip.Address]udpHandler{
		dnsAddr: m.dnsServer.HandleUDPConn,
	}
	fmt.Println("\nHosting DNS at", dnsAddr)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.apps = apps
	m.state.tcpHandlers = tcpHandlers
	m.state.udpHandlers = udpHandlers
	return nil
}

func createTunDevice() (tun.Device, string, error) {
	slog.Debug("Creating TUN device.")
	dev, err := tun.CreateTUN("utun", mtu)
	if err != nil {
		return nil, "", trace.Wrap(err, "creating TUN device")
	}
	name, err := dev.Name()
	if err != nil {
		return nil, "", trace.Wrap(err, "getting TUN device name")
	}
	return dev, name, nil
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

func forwardBetweenOsAndVnet(ctx context.Context, osTun tun.Device, vnetEndpoint *channel.Endpoint) error {
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return forwardVnetEndpointToOsTun(ctx, vnetEndpoint, osTun) })
	g.Go(func() error { return forwardOsTunToVnetEndpoint(ctx, osTun, vnetEndpoint) })
	return trace.Wrap(g.Wait())
}

func forwardVnetEndpointToOsTun(ctx context.Context, endpoint *channel.Endpoint, tun tun.Device) error {
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

func forwardOsTunToVnetEndpoint(ctx context.Context, tun tun.Device, dstEndpoint *channel.Endpoint) error {
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

// TODO: something better than this.
func setupHostIPRoutes(tunName string) error {
	const (
		ip   = "100.64.0.1"
		mask = "100.64.0.0/10"
	)
	fmt.Println("Setting IP address for the TUN device:")
	cmd := exec.Command("ifconfig", tunName, ip, ip, "up")
	fmt.Println("\t", cmd.Path, strings.Join(cmd.Args, " "))
	if err := cmd.Run(); err != nil {
		return trace.Wrap(err, "running ifconfig")
	}

	fmt.Println("Setting an IP route for the VNet:")
	cmd = exec.Command("route", "add", "-net", mask, "-interface", tunName)
	fmt.Println("\t", cmd.Path, strings.Join(cmd.Args, " "))
	if err := cmd.Run(); err != nil {
		return trace.Wrap(err, "running route add")
	}
	return nil
}

func getenvInt(envVar string) (int, error) {
	s := os.Getenv(envVar)
	if s == "" {
		return 0, trace.BadParameter(envVar + " is not set")
	}
	i, err := strconv.Atoi(s)
	return i, trace.Wrap(err)

}

func dropSudo() error {
	ouid, err := getenvInt("SUDO_UID")
	if err != nil {
		return trace.Wrap(err)
	}
	ogid, err := getenvInt("SUDO_GID")
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("Dropping sudo rights:")
	fmt.Println("\tsetgid", ogid)
	if err := syscall.Setgid(ogid); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("\tsetuid", ouid)
	if err := syscall.Setuid(ouid); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
