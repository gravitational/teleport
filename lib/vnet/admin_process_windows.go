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
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.zx2c4.com/wireguard/tun"
)

type windowsAdminProcessConfig struct {
	// clientApplicationServiceAddr is the local TCP address of the client
	// application gRPC service.
	clientApplicationServiceAddr string
}

// runWindowsAdminProcess must run as administrator. It creates and sets up a TUN
// device, runs the VNet networking stack, and handles OS configuration. It will
// continue to run until [ctx] is canceled or encountering an unrecoverable
// error.
func runWindowsAdminProcess(ctx context.Context, cfg *windowsAdminProcessConfig) error {
	log.InfoContext(ctx, "Running VNet admin process")

	clt, err := newClientApplicationServiceClient(ctx, cfg.clientApplicationServiceAddr)
	if err != nil {
		return trace.Wrap(err, "creating user process client")
	}
	defer clt.close()

	if err := authenticateUserProcess(ctx, clt); err != nil {
		return trace.Wrap(err, "authenticating user process")
	}

	device, err := tun.CreateTUN("TeleportVNet", mtu)
	if err != nil {
		return trace.Wrap(err, "creating TUN device")
	}
	defer device.Close()
	tunName, err := device.Name()
	if err != nil {
		return trace.Wrap(err, "getting TUN device name")
	}
	log.InfoContext(ctx, "Created TUN interface", "tun", tunName)

	networkStackConfig, err := newWindowsNetworkStackConfig(device, clt)
	if err != nil {
		return trace.Wrap(err, "creating network stack config")
	}
	networkStack, err := newNetworkStack(networkStackConfig)
	if err != nil {
		return trace.Wrap(err, "creating network stack")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error)
	go func() {
		errCh <- trace.Wrap(networkStack.run(ctx), "running network stack")
	}()
loop:
	for {
		select {
		case <-time.After(time.Second):
			if err := clt.Ping(ctx); err != nil {
				log.InfoContext(ctx, "Failed to ping client application, it may have exited, shutting down", "error", err)
				break loop
			}
		case <-ctx.Done():
			log.InfoContext(ctx, "Context canceled, shutting down", "error", err)
			break loop
		}
	}
	// Cancel the context and wait for networkStack.run to terminate.
	cancel()
	err = <-errCh
	return trace.Wrap(err, "running VNet network stack")
}

func newWindowsNetworkStackConfig(tun tunDevice, clt *clientApplicationServiceClient) (*networkStackConfig, error) {
	appProvider := newRemoteAppProvider(clt)
	appResolver := newTCPAppResolver(appProvider, clockwork.NewRealClock())
	ipv6Prefix, err := NewIPv6Prefix()
	if err != nil {
		return nil, trace.Wrap(err, "creating new IPv6 prefix")
	}
	dnsIPv6 := ipv6WithSuffix(ipv6Prefix, []byte{2})
	return &networkStackConfig{
		tunDevice:          tun,
		ipv6Prefix:         ipv6Prefix,
		dnsIPv6:            dnsIPv6,
		tcpHandlerResolver: appResolver,
	}, nil
}

func authenticateUserProcess(ctx context.Context, clt *clientApplicationServiceClient) error {
	// TODO(nklaassen): implement process authentication.
	return nil
}
