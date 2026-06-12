// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package desktop

import (
	"context"
	"net"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/utils"
)

// ConnectionConfig contains configuration needed to connect to Windows desktop service.
type ConnectionConfig struct {
	// Log emits log messages.
	Log *logrus.Entry
	// DesktopsGetter is responsible for getting desktops and desktop services.
	DesktopsGetter DesktopsGetter
	// Site represents a remote teleport site that can be accessed via
	// a teleport tunnel or directly by proxy.
	Site reversetunnelclient.RemoteSite
	// ClientSrcAddr is the original observed client address.
	ClientSrcAddr net.Addr
	// ClientDstAddr is the original client's destination address.
	ClientDstAddr net.Addr
	// ClusterName identifies the originating teleport cluster.
	ClusterName string
	// DesktopName is the target desktop name.
	DesktopName string
}

// DesktopsGetter is responsible for getting desktops and desktop services.
type DesktopsGetter interface {
	// GetWindowsDesktops returns windows desktop hosts.
	// TODO(gzdunek): Use ListWindowsDesktops that supports pagination.
	GetWindowsDesktops(ctx context.Context, filter types.WindowsDesktopFilter) ([]types.WindowsDesktop, error)
	// GetWindowsDesktopService returns a registered Windows desktop service by name.
	GetWindowsDesktopService(ctx context.Context, name string) (types.WindowsDesktopService, error)
}

// ConnectToWindowsService tries to make a connection to a Windows Desktop Service
// by trying each of the services provided. It returns an error if it could not connect
// to any of the services or if it encounters an error that is not a connection problem.
func ConnectToWindowsService(ctx context.Context, config *ConnectionConfig) (conn net.Conn, version string, err error) {
	// Pick a random Windows desktop service as our gateway.
	// When agent mode is implemented in the service, we'll have to filter out
	// the services in agent mode.
	//
	// In the future, we may want to do something smarter like latency-based
	// routing.
	winDesktops, err := config.DesktopsGetter.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{Name: config.DesktopName})
	if err != nil {
		return nil, "", trace.Wrap(err, "cannot get Windows desktops")
	}
	if len(winDesktops) == 0 {
		return nil, "", trace.NotFound("no Windows desktops were found")
	}

	validServiceIDs := make([]string, 0, len(winDesktops))
	for _, desktop := range winDesktops {
		if desktop.GetHostID() == "" {
			// desktops with empty host ids are invalid and should
			// only occur when migrating from an old version of teleport
			continue
		}
		validServiceIDs = append(validServiceIDs, desktop.GetHostID())
	}

	for _, id := range utils.ShuffleVisit(validServiceIDs) {
		conn, ver, err := tryConnect(ctx, id, config)
		if err == nil {
			return conn, ver, nil
		}
		config.Log.Warnf("failed to connect to windows_desktop_service %q: %v", id, err)

	}
	return nil, "", trace.Errorf("failed to connect to any windows_desktop_service")
}

func tryConnect(ctx context.Context, desktopServiceID string, config *ConnectionConfig) (conn net.Conn, version string, err error) {
	service, err := config.DesktopsGetter.GetWindowsDesktopService(ctx, desktopServiceID)
	if err != nil {
		config.Log.Errorf("Error finding service with id %s", desktopServiceID)
		return nil, "", trace.NotFound("could not find windows desktop service %s: %v", desktopServiceID, err)
	}

	conn, err = config.Site.DialTCP(reversetunnelclient.DialParams{
		From:                  config.ClientSrcAddr,
		To:                    &utils.NetAddr{AddrNetwork: "tcp", Addr: service.GetAddr()},
		ConnType:              types.WindowsDesktopTunnel,
		ServerID:              service.GetName() + "." + config.ClusterName,
		ProxyIDs:              service.GetProxyIDs(),
		OriginalClientDstAddr: config.ClientDstAddr,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	ver := service.GetTeleportVersion()
	config.Log.
		WithField("windows_service_version", ver).
		WithField("windows_service_uuid", service.GetName()).
		WithField("windows_service_addr", service.GetAddr()).
		Debug("Established windows_desktop_service connection")

	return conn, ver, nil
}
