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
	"os"
	"time"

	"github.com/gravitational/trace"
	"golang.zx2c4.com/wireguard/tun"

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
	if err := config.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err, "checking daemon process config")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	tunName, err := createAndSendTUNDevice(ctx, config.SocketPath)
	if err != nil {
		return trace.Wrap(err)
	}

	osConfigProvider, err := newProfileOSConfigProvider(tunName, config.IPv6Prefix, config.DNSAddr, config.HomePath, config.ClientCred)
	if err != nil {
		return trace.Wrap(err, "creating profileOSConfigProvider")
	}
	osConfigurator := newOSConfigurator(osConfigProvider)

	errCh := make(chan error)
	go func() {
		errCh <- trace.Wrap(osConfigurator.runOSConfigurationLoop(ctx))
	}()

	// Stay alive until we get an error on errCh, indicating that the osConfig loop exited.
	// If the socket is deleted, indicating that the unprivileged process exited, cancel the context
	// and then wait for the osConfig loop to exit and send an err on errCh.
	ticker := time.NewTicker(daemon.CheckUnprivilegedProcessInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if _, err := os.Stat(config.SocketPath); err != nil {
				log.DebugContext(ctx, "failed to stat socket path, assuming parent exited")
				cancel()
				return trace.Wrap(<-errCh)
			}
		case err := <-errCh:
			return trace.Wrap(err)
		}
	}
}

// createAndSendTUNDevice creates a virtual network TUN device and sends the open file descriptor on
// socketPath. It returns the name of the TUN device or an error.
func createAndSendTUNDevice(ctx context.Context, socketPath string) (string, error) {
	tun, tunName, err := createTUNDevice(ctx)
	if err != nil {
		return "", trace.Wrap(err, "creating TUN device")
	}

	defer func() {
		// We can safely close the TUN device in the admin process after it has been sent on the socket.
		if err := tun.Close(); err != nil {
			log.WarnContext(ctx, "Failed to close TUN device.", "error", trace.Wrap(err))
		}
	}()

	if err := sendTUNNameAndFd(socketPath, tunName, tun.File()); err != nil {
		return "", trace.Wrap(err, "sending TUN over socket")
	}
	return tunName, nil
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
