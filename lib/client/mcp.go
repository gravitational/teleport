/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/utils"
)

// MCPServerDialerClient defines a subset of TeleportClient functions that are
// used by MCPServerDialer.
type MCPServerDialerClient interface {
	DialALPN(context.Context, tls.Certificate, alpncommon.Protocol) (net.Conn, error)
	ListApps(context.Context, *proto.ListResourcesRequest) ([]types.Application, error)
	IssueUserCertsWithMFA(context.Context, ReissueParams) (*KeyRing, error)
	ProfileStatus() (*ProfileStatus, error)
	GetSiteName() string
}

// MCPServerDialer is a wrapper of TeleportClient for handling MCP connections
// to proxy.
type MCPServerDialer struct {
	client  MCPServerDialerClient
	appName string

	mu     sync.Mutex
	app    types.Application
	cert   tls.Certificate
	clock  clockwork.Clock
	logger *slog.Logger
}

// NewMCPServerDialer creates a new MCPServerDialer.
func NewMCPServerDialer(client MCPServerDialerClient, appName string) *MCPServerDialer {
	return &MCPServerDialer{
		client:  client,
		appName: appName,
		clock:   clockwork.NewRealClock(),
		logger: slog.With(
			teleport.ComponentKey,
			teleport.Component(teleport.ComponentMCP, "dialer"),
		),
	}
}

// GetApp returns the types.Application for the associated MCP server.
func (d *MCPServerDialer) GetApp(ctx context.Context) (types.Application, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.getAppLocked(ctx)
}

// DialALPN dials Teleport Proxy to establish a TLS routing connection for the
// MCP server.
func (d *MCPServerDialer) DialALPN(ctx context.Context) (net.Conn, error) {
	app, err := d.getAppLocked(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := d.getCertLocked(ctx, app)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch types.GetMCPServerTransportType(app.GetURI()) {
	case types.MCPTransportHTTP:
		return d.client.DialALPN(ctx, cert, alpncommon.ProtocolHTTP)
	default:
		return d.client.DialALPN(ctx, cert, alpncommon.ProtocolMCP)
	}
}

// DialContext is a simple wrapper of DialALPN. This function is defined to be
// compatible with common context dialer interfaces.
func (d *MCPServerDialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	return d.DialALPN(ctx)
}

func (d *MCPServerDialer) getAppLocked(ctx context.Context) (types.Application, error) {
	if d.app != nil {
		return d.app, nil
	}

	apps, err := d.client.ListApps(ctx, &proto.ListResourcesRequest{
		ResourceType:        types.KindAppServer,
		Namespace:           apidefaults.Namespace,
		PredicateExpression: fmt.Sprintf("name == %q", strings.TrimSpace(d.appName)),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch len(apps) {
	case 0:
		return nil, trace.NotFound("no MCP servers found")
	case 1:
	default:
		d.logger.WarnContext(ctx, "multiple appServers found, using the first one")
	}
	if !apps[0].IsMCP() {
		return nil, trace.BadParameter("app %q is not a MCP server", d.appName)
	}

	d.app = apps[0]
	d.logger.InfoContext(ctx, "Successfully fetched app",
		"name", d.app.GetName(),
		"transport", types.GetMCPServerTransportType(d.app.GetURI()),
	)
	return d.app, nil
}

func (d *MCPServerDialer) getCertLocked(ctx context.Context, mcpServer types.Application) (tls.Certificate, error) {
	if err := utils.VerifyTLSCertLeafExpiry(d.cert, d.clock); err == nil {
		return d.cert, nil
	}

	d.logger.InfoContext(ctx, "Reissuing certificate", "name", mcpServer.GetName())
	profile, err := d.client.ProfileStatus()
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	appCertParams := ReissueParams{
		RouteToCluster: d.client.GetSiteName(),
		RouteToApp: proto.RouteToApp{
			Name:        mcpServer.GetName(),
			PublicAddr:  mcpServer.GetPublicAddr(),
			ClusterName: d.client.GetSiteName(),
			URI:         mcpServer.GetURI(),
		},
		AccessRequests: profile.ActiveRequests,
	}

	// Do NOT write the keyring to avoid race condition when AI clients run
	// multiple tsh at the same time.
	keyRing, err := d.client.IssueUserCertsWithMFA(ctx, appCertParams)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	cert, err := keyRing.AppTLSCert(mcpServer.GetName())
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	d.logger.InfoContext(ctx, "Successfully issued certificate", "name", mcpServer.GetName())
	d.cert = cert
	return d.cert, nil
}
