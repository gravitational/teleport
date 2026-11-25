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

// routeResolveTimeout bounds how long resolving a route may take.
const routeResolveTimeout = 10 * time.Second

// routeCandidatesCacheTTL is how long the resolver caches an address's
// candidate app servers, which are costly to find on a large cluster.
const routeCandidatesCacheTTL = agentRouterCacheTTL

// routeResolverConfig configures a routeResolver.
type routeResolverConfig struct {
	clusterName      string
	authorizerOpts   authz.AuthorizerOpts
	clusterGetter    reversetunnelclient.ClusterGetter
	proxyPublicAddrs []utils.NetAddr
	logger           *slog.Logger
}

func (c *routeResolverConfig) checkAndSetDefaults() error {
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

// routeResolver resolves a connection's route: the app its SNI names that its
// certificate may access.
type routeResolver struct {
	clusterName   string
	authorizer    authz.ScopedAuthorizer
	appAuthorizer appauthz.Authorizer
	clusterGetter reversetunnelclient.ClusterGetter
	proxyDNSNames []string
	logger        *slog.Logger
	candidates    *utils.FnCache
}

// newRouteResolver returns a resolver for cfg.
func newRouteResolver(cfg routeResolverConfig) (*routeResolver, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	opts := cfg.authorizerOpts
	// Only roles can require a trusted device, as on the Application Service.
	opts.DeviceAuthorization.DisableGlobalMode = true
	authorizer, err := authz.NewScopedAuthorizer(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	candidates, err := newCandidatesCache()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &routeResolver{
		clusterName:   cfg.clusterName,
		authorizer:    authorizer,
		appAuthorizer: appauthz.Authorizer{Logger: cfg.logger, AuthPreferenceGetter: cfg.authorizerOpts.AccessPoint},
		clusterGetter: cfg.clusterGetter,
		proxyDNSNames: ProxyDNSNames(cfg.proxyPublicAddrs, cfg.clusterName),
		logger:        cfg.logger,
		candidates:    candidates,
	}, nil
}

// newCandidatesCache returns the cache of each address's candidate app servers.
// It doesn't keep a failed lookup.
func newCandidatesCache() (*utils.FnCache, error) {
	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:             routeCandidatesCacheTTL,
		CleanupInterval: routeCandidatesCacheTTL,
		ReloadOnErr:     true,
	})
	return cache, trace.Wrap(err)
}

// candidatesKey identifies an address's candidate app servers in a cluster.
type candidatesKey struct {
	clusterName string
	sni         string
}

// candidatesFor returns sni's candidate app servers in cluster, from the cache.
// Callers must not modify them.
func (r *routeResolver) candidatesFor(ctx context.Context, cluster reversetunnelclient.Cluster, sni string) ([]types.AppServer, error) {
	// Keyed by the normalized name, so spellings of one address share an entry.
	host, ok := normalizeHostname(sni)
	if !ok {
		return nil, nil
	}
	key := candidatesKey{clusterName: cluster.GetName(), sni: host}
	servers, err := r.candidates.Get(ctx, key, func(ctx context.Context) ([]types.AppServer, error) {
		// The cache loads with its own context, which has no deadline.
		ctx, cancel := context.WithTimeout(ctx, routeResolveTimeout)
		defer cancel()
		return candidatesForSNI(ctx, cluster, r.proxyDNSNames, host)
	})
	return servers, trace.Wrap(err)
}

// resolvedApp is a resolved route: the app server, the identity to forward to
// it, and the session context for the connection's controls.
type resolvedApp struct {
	server     types.AppServer
	identity   authz.LocalUser
	sessionCtx *authz.ScopedSessionContext
}

// resolve returns the first of sni's candidate app servers whose app cert may
// access. The candidates' order is fixed, so a certificate always gets the same
// app.
func (r *routeResolver) resolve(ctx context.Context, cert *x509.Certificate, sni string) (*resolvedApp, error) {
	cluster, err := r.clusterGetter.Cluster(ctx, r.clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers, err := r.candidatesFor(ctx, cluster, sni)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var failed []types.Application
	var errs []error
	for _, server := range servers {
		app := server.GetApp()
		// Skip copies of an app that already failed, since they'd fail too.
		if slices.ContainsFunc(failed, app.IsEqual) {
			continue
		}
		resolved, err := r.authorize(ctx, cert, app)
		if err != nil {
			r.logger.DebugContext(ctx, "SNI-route App resolution denied a candidate app",
				"app", app.GetName(), "scope", app.GetScope(), "host_id", server.GetHostID(), "error", err)
			failed = append(failed, app)
			errs = append(errs, err)
			continue
		}
		r.logger.DebugContext(ctx, "Resolved an App connection's route",
			"app", app.GetName(), "scope", app.GetScope(), "host_id", server.GetHostID())
		resolved.server = server
		return resolved, nil
	}
	return nil, accessError(errs)
}

// authorize checks that cert's identity, routed to app, may access app, and
// returns that identity with its session context.
func (r *routeResolver) authorize(ctx context.Context, cert *x509.Certificate, app types.Application) (*resolvedApp, error) {
	user, err := newForwardedIdentity(cert, r.clusterName, app)
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

// accessError returns the candidates' error that tells the client most:
// per-session MFA, then a trusted device, then an unsupported app type, then
// the first error. With no candidates, it's access denied. Each error comes
// from an app the client may discover, so none reveals another app.
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
