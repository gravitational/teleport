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
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	appcommon "github.com/gravitational/teleport/lib/srv/app/common"
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
	client MCPServerDialerClient
	appSQN scopes.QualifiedName

	mu           sync.Mutex
	app          types.Application
	appFetchedAt time.Time
	cert         tls.Certificate
	clock        clockwork.Clock
	logger       *slog.Logger
}

// mcpAppCacheTTL is how long a fetched app definition is reused between
// dials. The app service resolves the upstream anew for every session chunk,
// so re-fetching on the same horizon keeps the cached resource URI in step
// with where the app service forwards requests on a long-lived connection.
// Stored OAuth credentials are checked against that URI before every request
// and must not outlive a replaced upstream. The check is best-effort: the app
// service resolves the upstream on its own, so a URI replaced within this
// window can still receive the old token.
const mcpAppCacheTTL = appcommon.MaxSessionChunkDuration

// NewMCPServerDialer creates a new MCPServerDialer.
func NewMCPServerDialer(client MCPServerDialerClient, appSQN scopes.QualifiedName) *MCPServerDialer {
	return &MCPServerDialer{
		client: client,
		appSQN: appSQN,
		clock:  clockwork.NewRealClock(),
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
// MCP server. The app URI must match the first successful lookup; restart the
// connection to use a changed URI.
func (d *MCPServerDialer) DialALPN(ctx context.Context) (net.Conn, error) {
	d.mu.Lock()
	// A new connection is where the app service may have re-resolved the
	// upstream, such as after a restart or failover, so look the app up
	// instead of trusting the cache.
	app, err := d.fetchAppLocked(ctx)
	if err != nil {
		d.mu.Unlock()
		return nil, trace.Wrap(err)
	}
	cert, err := d.getCertLocked(ctx, app)
	if err != nil {
		d.mu.Unlock()
		return nil, trace.Wrap(err)
	}
	protocol := alpncommon.ProtocolMCP
	if types.GetMCPServerTransportType(app.GetURI()) == types.MCPTransportHTTP {
		protocol = alpncommon.ProtocolHTTP
	}
	d.mu.Unlock()

	// The app and certificate caches must be serialized, but independent
	// network connections should still be allowed to dial concurrently.
	return d.client.DialALPN(ctx, cert, protocol)
}

// DialContext is a simple wrapper of DialALPN. This function is defined to be
// compatible with common context dialer interfaces.
func (d *MCPServerDialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	return d.DialALPN(ctx)
}

// getAppLocked returns the cached app, fetching it again once it is older
// than mcpAppCacheTTL.
func (d *MCPServerDialer) getAppLocked(ctx context.Context) (types.Application, error) {
	if d.app != nil && d.clock.Since(d.appFetchedAt) < mcpAppCacheTTL {
		return d.app, nil
	}
	return d.fetchAppLocked(ctx)
}

// fetchAppLocked looks the app up and replaces the cached copy.
func (d *MCPServerDialer) fetchAppLocked(ctx context.Context) (types.Application, error) {
	appName := strings.TrimSpace(d.appSQN.Name)
	var predicate strings.Builder
	predicate.WriteString("name == ")
	predicate.WriteString(strconv.Quote(appName))
	if d.appSQN.Scope != "" {
		predicate.WriteString(" && resource.scope == ")
		predicate.WriteString(strconv.Quote(d.appSQN.Scope))
	}
	apps, err := d.client.ListApps(ctx, &proto.ListResourcesRequest{
		ResourceType:        types.KindAppServer,
		Namespace:           apidefaults.Namespace,
		PredicateExpression: predicate.String(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, app := range apps {
		if app.GetScope() != d.appSQN.Scope {
			continue
		}
		if !app.IsMCP() {
			return nil, trace.BadParameter("app %q is not a MCP server", d.appSQN)
		}
		if d.app != nil && d.app.GetURI() != app.GetURI() {
			return nil, trace.CompareFailed("MCP server %q URI changed, restart the connection", d.appSQN)
		}
		d.app = app
		d.appFetchedAt = d.clock.Now()
		d.logger.InfoContext(ctx, "Successfully fetched app",
			"name", d.app.GetName(),
			"scope", d.app.GetScope(),
			"transport", types.GetMCPServerTransportType(d.app.GetURI()),
		)
		return d.app, nil
	}
	return nil, trace.NotFound("MCP server %q not found", d.appSQN)
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
			Scope:       mcpServer.GetScope(),
			PublicAddr:  mcpServer.GetPublicAddr(),
			ClusterName: d.client.GetSiteName(),
			URI:         mcpServer.GetURI(),
		},
		AccessRequests: profile.ActiveRequests,
		// The local proxy requester avoids the one-minute MFA certificate cap.
		RequesterName: proto.UserCertsRequest_TSH_APP_LOCAL_PROXY,
	}

	// Do NOT write the keyring to avoid race condition when AI clients run
	// multiple tsh at the same time.
	keyRing, err := d.client.IssueUserCertsWithMFA(ctx, appCertParams)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	cert, err := keyRing.AppTLSCert(scopes.QualifiedName{Name: appCertParams.RouteToApp.Name, Scope: appCertParams.RouteToApp.Scope})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	d.logger.InfoContext(ctx, "Successfully issued certificate", "name", mcpServer.GetName())
	d.cert = cert
	return d.cert, nil
}
