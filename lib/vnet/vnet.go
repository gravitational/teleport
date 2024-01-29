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
	"syscall"

	"github.com/davecgh/go-spew/spew"
	"github.com/gravitational/teleport/lib/client"
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
	"gvisor.dev/gvisor/pkg/waiter"
)

const (
	// TODO: Find optimal MTU.
	mtu = 1500
)

func Run(ctx context.Context, tc *client.TeleportClient) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	osTun, tunName, err := createTunDevice()
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		<-ctx.Done()
		if err := osTun.Close(); err != nil {
			slog.Debug("Closing TUN device", "err", err)
		}
	}()
	fmt.Println("Created TUN device", tunName)

	if err := setupHostIPRoutes(tunName); err != nil {
		return trace.Wrap(err)
	}

	// TODO: Figure out why dropping sudo causes this error when dialing proxy:
	//         tls: failed to verify certificate: SecPolicyCreateSSL error: 0
	// if err := dropSudo(); err != nil {
	// 	return trace.Wrap(err)
	// }

	handlers, err := buildHandlers(ctx, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	netStack, vnetEndpoint, err := createVnet(ctx, handlers)
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		<-ctx.Done()
		vnetEndpoint.Close()
		netStack.Close()
	}()

	statsC := make(chan os.Signal, 1)
	signal.Notify(statsC, syscall.SIGUSR1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-statsC:
				stats := netStack.Stats()
				spew.Dump(stats)
			}

		}
	}()

	if err := forwardBetweenOsAndVnet(ctx, osTun, vnetEndpoint); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func createTunDevice() (tun.Device, string, error) {
	slog.Debug("Creating TUN device")
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

type tcpHandler func(ctx context.Context, conn io.ReadWriteCloser) error

type Handlers struct {
	tcp map[tcpip.Address]tcpHandler
}

func (h *Handlers) getTcpHandler(addr tcpip.Address) (tcpHandler, bool) {
	handler, ok := h.tcp[addr]
	return handler, ok
}

func buildHandlers(ctx context.Context, tc *client.TeleportClient) (*Handlers, error) {
	tcpHandlers, err := buildTcpAppHandlers(ctx, tc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Handlers{
		tcp: tcpHandlers,
	}, nil
}

func createVnet(ctx context.Context, handlers *Handlers) (*stack.Stack, *channel.Endpoint, error) {
	// TODO: DNS
	netStack := stack.New(stack.Options{
		// TODO: IPv6
		NetworkProtocols: []stack.NetworkProtocolFactory{ipv4.NewProtocol},
		// TODO: Consider UDP, ICMP
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol},
	})

	const (
		linkAddr = ""
		nicID    = 1
	)
	linkEndpoint := channel.New(512, uint32(mtu), linkAddr)
	if err := netStack.CreateNIC(nicID, linkEndpoint); err != nil {
		return nil, nil, trace.Errorf("creating VNet NIC: %s", err)
	}
	// Make the NIC accept all IP packets on the VNet, regardless of destination
	// address.
	netStack.SetPromiscuousMode(nicID, true)

	// Route everything to the one NIC.
	// TODO: Support IPv6.
	ipv4Subnet, err := tcpip.NewSubnet(tcpip.AddrFrom4([4]byte{}), tcpip.MaskFromBytes(make([]byte, 4)))
	if err != nil {
		return nil, nil, trace.Wrap(err, "creating VNet IPv4 subnet")
	}
	netStack.SetRouteTable([]tcpip.Route{{
		Destination: ipv4Subnet,
		NIC:         nicID,
	}})

	const (
		// TODO: Figure out optimal values for these.
		tcpReceiveBufferSize          = 0 // 0 means a default will be used.
		maxInFlightConnectionAttempts = 1024
	)
	tcpForwarder := tcp.NewForwarder(netStack, tcpReceiveBufferSize, maxInFlightConnectionAttempts, func(req *tcp.ForwarderRequest) {
		slog.Debug("Got TCP forward request", "id", req.ID())

		// Add the address to the NIC so that the VNet routes packets back out
		// to the host.
		// TODO: add only known app addresses instead of doing this on each
		// incoming TCP SYN
		netStack.AddProtocolAddress(nicID, tcpip.ProtocolAddress{
			AddressWithPrefix: req.ID().LocalAddress.WithPrefix(),
			Protocol:          ipv4.ProtocolNumber, // TODO: Support IPv6
		}, stack.AddressProperties{})

		handler, ok := handlers.getTcpHandler(req.ID().LocalAddress)
		if !ok {
			slog.Debug("No handler")
			req.Complete(true)
			return
		}

		var wq waiter.Queue
		endpoint, err := req.CreateEndpoint(&wq)
		if err != nil {
			slog.Debug("Failed to create endpoint.", "err", err)
			req.Complete(true)
			return
		}
		req.Complete(false)
		endpoint.SocketOptions().SetKeepAlive(true)
		conn := gonet.NewTCPConn(&wq, endpoint)
		defer conn.Close()

		waitEntry, notifyCh := waiter.NewChannelEntry(waiter.EventHUp)
		wq.EventRegister(&waitEntry)
		defer wq.EventUnregister(&waitEntry)
		done := make(chan struct{})
		defer close(done)
		_, cancel := context.WithCancel(context.Background())
		go func() {
			select {
			case <-notifyCh:
				slog.Debug("Got HUP")
			case <-done:
			}
			cancel()
		}()

		if err := handler(ctx, conn); err != nil {
			slog.Debug("Error handling conn", "err", err)
		}
	})
	netStack.SetTransportProtocolHandler(tcp.ProtocolNumber, tcpForwarder.HandlePacket)
	return netStack, linkEndpoint, nil
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
	fmt.Println("Running the following commands to set up the TUN device:")
	cmd := exec.Command("ifconfig", tunName, ip, ip, "up")
	fmt.Println("\t", cmd.Path, strings.Join(cmd.Args, " "))
	if err := cmd.Run(); err != nil {
		return trace.Wrap(err, "running ifconfig")
	}
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
