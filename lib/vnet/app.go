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
	"crypto/tls"
	"crypto/x509"
	"io"
	"log/slog"
	"net"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
)

type tcpAppHandler struct {
	tc  *client.TeleportClient
	app types.Application
}

func (h *tcpAppHandler) handleTCP(ctx context.Context, connector tcpConnector) error {
	appName := h.app.GetName()
	cert, err := appCert(ctx, h.tc, appName, h.app.GetPublicAddr())
	if err != nil {
		return trace.Wrap(err, "getting cert for app %s", appName)
	}
	appConn, err := dialApp(ctx, h.tc, cert)
	if err != nil {
		return trace.Wrap(err, "dialing app %s", appName)
	}
	conn, err := connector()
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(utils.ProxyConn(ctx, conn, appConn))
}

type httpAppHandler struct {
	tc  *client.TeleportClient
	app types.Application
}

func (h *httpAppHandler) handleTCP(ctx context.Context, connector tcpConnector) error {
	appName := h.app.GetName()
	cert, err := appCert(ctx, h.tc, appName, h.app.GetPublicAddr())
	if err != nil {
		return trace.Wrap(err, "getting cert for app %s", appName)
	}

	downstreamConn, err := connector()
	if err != nil {
		return trace.Wrap(err)
	}
	bufReader := bufio.NewReader(downstreamConn)
	header, err := bufReader.Peek(6)
	if err != nil {
		return trace.Wrap(err, "peeking tcp stream")
	}
	downstreamConn = peekedConn{WriteCloser: downstreamConn, Reader: bufReader}

	var upstreamConn io.ReadWriteCloser
	if isTLSClientHello(header) {
		var dialer net.Dialer
		upstreamConn, err = dialer.DialContext(ctx, "tcp", h.tc.WebProxyAddr)
		if err != nil {
			return trace.Wrap(err, "dialing proxy")
		}
	} else {
		upstreamConn, err = dialApp(ctx, h.tc, cert)
		if err != nil {
			return trace.Wrap(err, "dialing app %s", appName)
		}
	}

	return trace.Wrap(utils.ProxyConn(ctx, downstreamConn, upstreamConn))
}

type peekedConn struct {
	io.WriteCloser
	io.Reader
}

func isTLSClientHello(header []byte) bool {
	switch {
	case len(header) < 6:
		return false
	case header[0] != 0x16:
		// not a handshake
		return false
	case header[1] != 3:
		// not TLS 1.x
		return false
	case header[5] != 1:
		// not Client Hello
		return false
	}
	return true
}

func newAppHandler(tc *client.TeleportClient, app types.Application) tcpHandler {
	if app.IsTCP() {
		return &tcpAppHandler{
			tc:  tc,
			app: app,
		}
	}
	return &httpAppHandler{
		tc:  tc,
		app: app,
	}
}

func dialApp(ctx context.Context, tc *client.TeleportClient, cert *tls.Certificate) (*tls.Conn, error) {
	alpnDialerConfig := apiclient.ALPNDialerConfig{
		ALPNConnUpgradeRequired: tc.TLSRoutingConnUpgradeRequired,
		TLSConfig: &tls.Config{
			NextProtos:   common.ProtocolsToString([]common.Protocol{common.ProtocolTCP}),
			Certificates: []tls.Certificate{*cert},
		},
		GetClusterCAs: func(context.Context) (*x509.CertPool, error) { return tc.LocalAgent().ClientCertPool(tc.SiteName) },
	}
	tlsConn, err := apiclient.DialALPN(ctx, tc.WebProxyAddr, alpnDialerConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tlsConn, nil
}

func appCert(ctx context.Context, tc *client.TeleportClient, appName, appPublicAddr string) (*tls.Certificate, error) {
	slog.Debug("Getting cert for app", slog.String("app", appName))
	key, err := tc.LocalAgent().GetKey(tc.SiteName, client.WithAppCerts{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// TODO: check cert expiry
	cert, ok := key.AppTLSCerts[appName]
	if !ok {
		if err := appLogin(ctx, tc, appName, appPublicAddr); err != nil {
			return nil, trace.Wrap(err)
		}
		key, err := tc.LocalAgent().GetKey(tc.SiteName, client.WithAppCerts{})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cert, ok = key.AppTLSCerts[appName]
		if !ok {
			return nil, trace.Errorf("unable to log in to app %q", appName)
		}
	}
	tlsCert, err := key.TLSCertificate(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &tlsCert, nil
}

func appLogin(ctx context.Context, tc *client.TeleportClient, appName, appPublicAddr string) error {
	slog.Debug("Logging in to app", slog.String("app", appName))
	currentProfile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}
	request := types.CreateAppSessionRequest{
		Username:    tc.Username,
		PublicAddr:  appPublicAddr,
		ClusterName: tc.SiteName,
	}
	webSession, err := tc.CreateAppSession(ctx, request)
	if err != nil {
		return trace.Wrap(err)
	}

	certReissueParams := client.ReissueParams{
		RouteToCluster: currentProfile.Cluster,
		RouteToApp: proto.RouteToApp{
			Name:        appName,
			SessionID:   webSession.GetName(),
			PublicAddr:  appPublicAddr,
			ClusterName: tc.SiteName,
		},
		AccessRequests: currentProfile.ActiveRequests.AccessRequests,
	}
	err = tc.ReissueUserCerts(ctx, client.CertCacheKeep, certReissueParams)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
