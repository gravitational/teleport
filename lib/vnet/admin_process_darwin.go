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
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"golang.zx2c4.com/wireguard/tun"

	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/vnet/daemon"
)

// RunDarwinAdminProcess must run as root. It creates and sets up a TUN device
// and passes the file descriptor for that device over the unix socket found at
// config.socketPath.
//
// It also handles host OS configuration that must run as root, and stays alive
// to keep the host configuration up to date. It will stay running until the
// socket at config.socketPath is deleted, ctx is canceled, or until
// encountering an unrecoverable error.
func RunDarwinAdminProcess(ctx context.Context, config daemon.Config) error {
	log.InfoContext(ctx, "Running VNet admin process")
	if err := config.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "checking daemon process config")
	}

	serviceCreds, err := readCredentials(config.ServiceCredentialPath)
	if err != nil {
		return trace.Wrap(err, "reading service IPC credentials")
	}
	clt, err := newClientApplicationServiceClient(ctx, serviceCreds, config.ClientApplicationServiceAddr)
	if err != nil {
		return trace.Wrap(err, "creating user process client")
	}
	defer clt.close()

	tun, tunName, err := createTUNDevice(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer tun.Close()

	networkStackConfig, err := newNetworkStackConfig(ctx, tun, clt)
	if err != nil {
		return trace.Wrap(err, "creating network stack config")
	}
	networkStack, err := newNetworkStack(networkStackConfig)
	if err != nil {
		return trace.Wrap(err, "creating network stack")
	}

	if err := clt.ReportNetworkStackInfo(ctx, &vnetv1.NetworkStackInfo{
		InterfaceName: tunName,
		Ipv6Prefix:    networkStackConfig.ipv6Prefix.String(),
	}); err != nil {
		return trace.Wrap(err, "reporting network stack info to client application")
	}

	osConfigProvider, err := newRemoteOSConfigProvider(
		clt,
		tunName,
		networkStackConfig.ipv6Prefix.String(),
		networkStackConfig.dnsIPv6.String(),
	)
	if err != nil {
		return trace.Wrap(err, "creating OS config provider")
	}
	osConfigurator := newOSConfigurator(osConfigProvider)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if err := networkStack.run(ctx); err != nil {
			return trace.Wrap(err, "running network stack")
		}
		return errors.New("network stack terminated")
	})
	g.Go(func() error {
		if err := osConfigurator.runOSConfigurationLoop(ctx); err != nil {
			return trace.Wrap(err, "running OS configuration loop")
		}
		return errors.New("OS configuration loop terminated")
	})
	g.Go(func() error {
		tick := time.Tick(time.Second)
		for {
			select {
			case <-tick:
				if err := clt.Ping(ctx); err != nil {
					return trace.Wrap(err, "failed to ping client application, it may have exited, shutting down")
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})
	return trace.Wrap(g.Wait(), "running VNet admin process")
}

func createTUNDevice(ctx context.Context) (tun.Device, string, error) {
	log.DebugContext(ctx, "Creating TUN device.")
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
