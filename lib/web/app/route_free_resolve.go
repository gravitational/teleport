/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package app

import (
	"context"
	"crypto/x509"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	appauthz "github.com/gravitational/teleport/lib/srv/app/authz"
	"github.com/gravitational/teleport/lib/utils"
)

// routeFreeResolveTimeout bounds how long resolution waits on reads that miss the
// Proxy's caches, such as a stale app server watcher's refresh. The ALPN router
// sets no deadline once the handshake is done.
const routeFreeResolveTimeout = 10 * time.Second

// routeFreeResolverConfig holds what resolving a connection to an app takes.
// NewRouteFreeHandler fills it from RouteFreeConfig.
type routeFreeResolverConfig struct {
	clusterName      string
	authorizerOpts   authz.AuthorizerOpts
	clusterGetter    reversetunnelclient.ClusterGetter
	proxyPublicAddrs []utils.NetAddr
	logger           *slog.Logger
}

func (c *routeFreeResolverConfig) checkAndSetDefaults() error {
	if c.clusterName == "" {
		return trace.BadParameter("missing cluster name")
	}
	if c.clusterGetter == nil {
		return trace.BadParameter("missing cluster getter")
	}
	if c.logger == nil {
		c.logger = slog.Default()
	}
	return nil
}

// routeFreeResolver resolves a connection to the app it may reach.
type routeFreeResolver struct {
	clusterName   string
	authorizer    authz.ScopedAuthorizer
	appAuthorizer appauthz.Authorizer
	clusterGetter reversetunnelclient.ClusterGetter
	proxyDNSNames []string
	logger        *slog.Logger
}

// newRouteFreeResolver returns a resolver for cfg's cluster, or an error when cfg
// can't resolve connections in it. The authorizer it builds holds a cache that
// every candidate app's authorization then shares, so a Proxy builds one
// resolver and keeps it.
func newRouteFreeResolver(cfg routeFreeResolverConfig) (*routeFreeResolver, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	opts := cfg.authorizerOpts
	opts.DeviceAuthorization.DisableGlobalMode = true
	authorizer, err := authz.NewScopedAuthorizer(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &routeFreeResolver{
		clusterName:   cfg.clusterName,
		authorizer:    authorizer,
		appAuthorizer: appauthz.Authorizer{Logger: cfg.logger, AuthPreferenceGetter: cfg.authorizerOpts.AccessPoint},
		clusterGetter: cfg.clusterGetter,
		proxyDNSNames: ProxyDNSNames(cfg.proxyPublicAddrs, cfg.clusterName),
		logger:        cfg.logger,
	}, nil
}

// resolvedApp is what resolving a connection found: the app server it reaches,
// the identity the Proxy forwards to it, and what the connection controls of
// the role that granted access need.
type resolvedApp struct {
	server     types.AppServer
	identity   authz.LocalUser
	sessionCtx *authz.ScopedSessionContext
}

// resolve returns the first of sni's candidate app servers with an app that
// cert's identity may access, so the same certificate reaches the same app on
// every connection.
func (r *routeFreeResolver) resolve(ctx context.Context, cert *x509.Certificate, sni string) (*resolvedApp, error) {
	cluster, err := r.clusterGetter.Cluster(ctx, r.clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers, err := candidatesForSNI(ctx, cluster, r.proxyDNSNames, sni)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var failed []types.Application
	var errs []error
	for _, server := range servers {
		app := server.GetApp()
		// Agents usually hold equal copies of an app, and authorizing a copy
		// costs far more than comparing two. A copy equal to one that already
		// failed would fail too, whichever of its fields authorization reads.
		if slices.ContainsFunc(failed, app.IsEqual) {
			continue
		}
		resolved, err := r.authorize(ctx, cert, app)
		if err != nil {
			r.logger.DebugContext(ctx, "Route-free App resolution denied a candidate app",
				"app", app.GetName(), "scope", app.GetScope(), "host_id", server.GetHostID(), "error", err)
			failed = append(failed, app)
			errs = append(errs, err)
			continue
		}
		r.logger.DebugContext(ctx, "Resolved a route-free App connection",
			"app", app.GetName(), "scope", app.GetScope(), "host_id", server.GetHostID())
		resolved.server = server
		return resolved, nil
	}
	return nil, accessError(errs)
}

// authorize checks that the identity cert would carry to app may access it, as
// the Application Service checks each request, and returns that identity with
// the session controls of the role that granted access.
func (r *routeFreeResolver) authorize(ctx context.Context, cert *x509.Certificate, app types.Application) (*resolvedApp, error) {
	user, err := newRoutedIdentity(cert, r.clusterName, app)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ctx = authz.ContextWithUser(ctx, user)
	authCtx, err := r.authorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := r.appAuthorizer.AuthorizeForwarded(ctx, authCtx, app)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &resolvedApp{
		identity:   user,
		sessionCtx: &authz.ScopedSessionContext{Context: authCtx, SessionControls: checker},
	}, nil
}

// accessError picks, from the candidates' errors, the one that tells a client
// most: per-session MFA, then a trusted device, then an app type this path
// can't serve. Otherwise it's the first error, or access denied when no app
// answers to the address. Discovery is checked first, so these come only from
// apps the client may discover.
func accessError(errs []error) error {
	for _, target := range []error{services.ErrSessionMFARequired, services.ErrTrustedDeviceRequired, appauthz.ErrUnsupportedApplicationType} {
		for _, err := range errs {
			if errors.Is(err, target) {
				return err
			}
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return trace.AccessDenied("no app answers to this address")
}
