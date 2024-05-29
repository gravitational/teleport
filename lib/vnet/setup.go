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
	"log/slog"
	"os"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"golang.zx2c4.com/wireguard/tun"
)

// Run is a blocking call to create and start Teleport VNet.
func Run(ctx context.Context, appProvider AppProvider) error {
	ipv6Prefix, err := NewIPv6Prefix()
	if err != nil {
		return trace.Wrap(err)
	}

	dnsIPv6 := ipv6WithSuffix(ipv6Prefix, []byte{2})

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	tunCh, adminCommandErrCh := CreateAndSetupTUNDevice(ctx, ipv6Prefix.String(), dnsIPv6.String())

	var tun TUNDevice
	select {
	case err := <-adminCommandErrCh:
		return trace.Wrap(err)
	case tun = <-tunCh:
	}

	appResolver, err := NewTCPAppResolver(appProvider)
	if err != nil {
		return trace.Wrap(err)
	}

	manager, err := NewManager(&Config{
		TUNDevice:          tun,
		IPv6Prefix:         ipv6Prefix,
		DNSIPv6:            dnsIPv6,
		TCPHandlerResolver: appResolver,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	allErrors := make(chan error, 2)
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		// Make sure to cancel the context if manager.Run terminates for any reason.
		defer cancel()
		err := trace.Wrap(manager.Run(ctx), "running VNet manager")
		allErrors <- err
		return err
	})
	g.Go(func() error {
		var adminCommandErr error
		select {
		case adminCommandErr = <-adminCommandErrCh:
			// The admin command exited before the context was canceled, cancel everything and exit.
			cancel()
		case <-ctx.Done():
			// The context has been canceled, the admin command should now exit.
			adminCommandErr = <-adminCommandErrCh
		}
		adminCommandErr = trace.Wrap(adminCommandErr, "running admin subcommand")
		allErrors <- adminCommandErr
		return adminCommandErr
	})
	// Deliberately ignoring the error from g.Wait() to return an aggregate of all errors.
	_ = g.Wait()
	close(allErrors)
	return trace.NewAggregateFromChannel(allErrors, context.Background())
}

// AdminSubcommand is the tsh subcommand that should run as root that will create and setup a TUN device and
// pass the file descriptor for that device over the unix socket found at socketPath.
//
// It also handles host OS configuration that must run as root, and stays alive to keep the host configuration
// up to date. It will stay running until the socket at [socketPath] is deleting or encountering an
// unrecoverable error.
func AdminSubcommand(ctx context.Context, socketPath, ipv6Prefix, dnsAddr string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	tunCh, errCh := createAndSetupTUNDeviceAsRoot(ctx, ipv6Prefix, dnsAddr)
	var tun tun.Device
	select {
	case tun = <-tunCh:
	case err := <-errCh:
		return trace.Wrap(err, "performing admin setup")
	}
	tunName, err := tun.Name()
	if err != nil {
		return trace.Wrap(err, "getting TUN name")
	}
	if err := sendTUNNameAndFd(socketPath, tunName, tun.File().Fd()); err != nil {
		return trace.Wrap(err, "sending TUN over socket")
	}

	// Stay alive until we get an error on errCh, indicating that the osConfig loop exited.
	// If the socket is deleted, indicating that the parent process exited, cancel the context and then wait
	// for the osConfig loop to exit and send an err on errCh.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if _, err := os.Stat(socketPath); err != nil {
				slog.DebugContext(ctx, "failed to stat socket path, assuming parent exited")
				cancel()
				return trace.Wrap(<-errCh)
			}
		case err = <-errCh:
			return trace.Wrap(err)
		}
	}
}

// CreateAndSetupTUNDevice creates a virtual network device and configures the host OS to use that device for
// VNet connections.
//
// If not already running as root, it will spawn a root process to handle the TUN creation and host
// configuration.
//
// After the TUN device is created, it will be sent on the result channel. Any error will be sent on the err
// channel. Always select on both the result channel and the err channel when waiting for a result.
//
// This will keep running until [ctx] is canceled or an unrecoverable error is encountered, in order to keep
// the host OS configuration up to date.
func CreateAndSetupTUNDevice(ctx context.Context, ipv6Prefix, dnsAddr string) (<-chan tun.Device, <-chan error) {
	if os.Getuid() == 0 {
		// We can get here if the user runs `tsh vnet` as root, but it is not in the expected path when
		// started as a regular user. Typically we expect `tsh vnet` to be run as a non-root user, and for
		// AdminSubcommand to directly call createAndSetupTUNDeviceAsRoot.
		return createAndSetupTUNDeviceAsRoot(ctx, ipv6Prefix, dnsAddr)
	} else {
		return createAndSetupTUNDeviceWithoutRoot(ctx, ipv6Prefix, dnsAddr)
	}
}

// createAndSetupTUNDeviceAsRoot creates a virtual network device and configures the host OS to use that device for
// VNet connections.
//
// After the TUN device is created, it will be sent on the result channel. Any error will be sent on the err
// channel. Always select on both the result channel and the err channel when waiting for a result.
//
// This will keep running until [ctx] is canceled or an unrecoverable error is encountered, in order to keep
// the host OS configuration up to date.
func createAndSetupTUNDeviceAsRoot(ctx context.Context, ipv6Prefix, dnsAddr string) (<-chan tun.Device, <-chan error) {
	tunCh := make(chan tun.Device, 1)
	errCh := make(chan error, 2)

	tun, tunName, err := createTUNDevice(ctx)
	if err != nil {
		errCh <- trace.Wrap(err, "creating TUN device")
		return tunCh, errCh
	}
	tunCh <- tun

	osConfigurator, err := newOSConfigurator(tunName, ipv6Prefix, dnsAddr)
	if err != nil {
		errCh <- trace.Wrap(err, "creating OS configurator")
		return tunCh, errCh
	}

	go func() {
		defer func() {
			// Shutting down, deconfigure OS.
			errCh <- trace.Wrap(osConfigurator.deconfigureOS())
		}()

		if err := osConfigurator.updateOSConfiguration(ctx); err != nil {
			errCh <- trace.Wrap(err, "applying initial OS configuration")
			return
		}

		// Re-configure the host OS every 10 seconds. This will pick up any newly logged-in clusters by
		// reading profiles from TELEPORT_HOME.
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := osConfigurator.updateOSConfiguration(ctx); err != nil {
					errCh <- trace.Wrap(err, "updating OS configuration")
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return tunCh, errCh
}

func createTUNDevice(ctx context.Context) (tun.Device, string, error) {
	slog.DebugContext(ctx, "Creating TUN device.")
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
