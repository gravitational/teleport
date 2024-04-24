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
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"

	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLogger(utils.LoggingForCLI, slog.LevelDebug)
	os.Exit(m.Run())
}

type testPack struct {
	vnetIPv6Prefix tcpip.Address
	manager        *Manager

	testStack        *stack.Stack
	testLinkEndpoint *channel.Endpoint
	localAddress     tcpip.Address
}

func newTestPack(t *testing.T, ctx context.Context) *testPack {
	ctx, cancel := context.WithCancel(ctx)

	// Create two sides of an emulated TUN interface: writes to one can be read on the other, and vice versa.
	tun1, tun2 := newSplitTUN()

	// Create an isolated userspace networking stack that can be manipulated from test code. It will be
	// connected to the VNet over the TUN interface. This emulates the host networking stack.
	testStack, linkEndpoint, err := createStack()
	require.NoError(t, err)

	// Assign a local IP address to the test stack.
	localAddress, err := randomULAAddress()
	require.NoError(t, err)
	protocolAddr, err := protocolAddress(localAddress)
	require.NoError(t, err)
	tcpErr := testStack.AddProtocolAddress(nicID, protocolAddr, stack.AddressProperties{})
	require.Nil(t, tcpErr)

	// Route the VNet range to the TUN interface - this emulates the route that will be installed on the host.
	vnetIPv6Prefix, err := IPv6Prefix()
	require.NoError(t, err)
	subnet, err := tcpip.NewSubnet(vnetIPv6Prefix, tcpip.MaskFromBytes(net.CIDRMask(64, 128)))
	require.NoError(t, err)
	testStack.SetRouteTable([]tcpip.Route{{
		Destination: subnet,
		NIC:         nicID,
	}})

	go func() {
		err := forwardBetweenTunAndNetstack(ctx, tun1, linkEndpoint)
		if ignoreCancel(err) != nil {
			slog.With("error", err).DebugContext(ctx, "Forwarding to test netstack failed, canceling context.")
			cancel()
		}
	}()

	// Create the VNet and connect it to the other side of the TUN.
	manager, err := NewManager(ctx, &Config{
		TUNDevice:  tun2,
		IPv6Prefix: vnetIPv6Prefix,
	})
	require.NoError(t, err)

	// Run the VNet in the background.
	go func() {
		err := manager.Run()
		if ignoreCancel(err) != nil {
			slog.With("error", err).DebugContext(ctx, "Running VNet failed, canceling context.")
			cancel()
		}
	}()

	t.Cleanup(func() {
		cancel()
		manager.Destroy()
	})

	return &testPack{
		vnetIPv6Prefix:   vnetIPv6Prefix,
		manager:          manager,
		testStack:        testStack,
		testLinkEndpoint: linkEndpoint,
		localAddress:     localAddress,
	}
}

// dial dials the VNet address [addr] from the test virtual netstack.
func (p *testPack) dial(ctx context.Context, addr tcpip.Address) (*gonet.TCPConn, error) {
	conn, err := gonet.DialTCPWithBind(
		ctx,
		p.testStack,
		tcpip.FullAddress{
			NIC:      nicID,
			Addr:     p.localAddress,
			LinkAddr: p.testLinkEndpoint.LinkAddress(),
		},
		tcpip.FullAddress{
			NIC:      nicID,
			Addr:     addr,
			Port:     456,
			LinkAddr: p.manager.linkEndpoint.LinkAddress(),
		},
		ipv6.ProtocolNumber,
	)
	return conn, trace.Wrap(err)
}

// TestVnetEcho is a preliminary VNet test that manually stalls an echo handler on a specific IP, TCP dials
// it, and makes sure writes are echoed back to the TCP conn.
func TestVnetEcho(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := newTestPack(t, ctx)

	dialAddress, err := p.manager.assignTCPHandler(echoHandler{})
	require.NoError(t, err)

	conn, err := p.dial(ctx, dialAddress)
	require.NoError(t, err)
	defer conn.Close()

	testString := "Hello, World!\n"
	_, err = conn.Write([]byte(testString))
	require.NoError(t, err)
	defer func() { require.NoError(t, conn.Close()) }()

	scanner := bufio.NewScanner(conn)
	require.True(t, scanner.Scan(), "scan failed: %v", scanner.Err())
	line := scanner.Text()
	require.Equal(t, strings.TrimSuffix(testString, "\n"), line)
}

type echoHandler struct{}

func (echoHandler) handleTCP(ctx context.Context, connector tcpConnector) error {
	conn, err := connector()
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()
	_, err = io.Copy(conn, conn)
	return trace.Wrap(err)
}

func randomULAAddress() (tcpip.Address, error) {
	prefix, err := IPv6Prefix()
	if err != nil {
		return tcpip.Address{}, trace.Wrap(err)
	}
	bytes := prefix.As16()
	bytes[15] = 2
	return tcpip.AddrFrom16(bytes), nil
}

var fakeTUNClosedErr = errors.New("TUN closed")

type fakeTUN struct {
	name                            string
	readPacketsFrom, writePacketsTo chan []byte
	closed                          chan struct{}
	closeOnce                       func()
}

// newSplitTUN returns two fake TUN devices that are tied together: writes to one can be read on the other,
// and vice versa.
func newSplitTUN() (*fakeTUN, *fakeTUN) {
	closed := make(chan struct{})
	closeOnce := sync.OnceFunc(func() { close(closed) })
	ab := make(chan []byte)
	ba := make(chan []byte)
	return &fakeTUN{
			name:            "tun1",
			readPacketsFrom: ab,
			writePacketsTo:  ba,
			closed:          closed,
			closeOnce:       closeOnce,
		}, &fakeTUN{
			name:            "tun2",
			readPacketsFrom: ba,
			writePacketsTo:  ab,
			closed:          closed,
			closeOnce:       closeOnce,
		}
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
		return 0, fakeTUNClosedErr
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
		return 0, fakeTUNClosedErr
	case packet = <-f.readPacketsFrom:
	}
	sizes[0] = copy(bufs[0][offset:], packet)
	return 1, nil
}

func (f *fakeTUN) Close() error {
	f.closeOnce()
	return nil
}
