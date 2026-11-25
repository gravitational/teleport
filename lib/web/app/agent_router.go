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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/componentfeatures"
	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
	appauthz "github.com/gravitational/teleport/lib/srv/app/authz"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// agentRouterCacheTTL is how long the router reuses a route's capable app
// servers rather than scanning the watcher on every request. An app server
// that registers waits up to this long for requests, and one that leaves costs
// a failed dial for as long.
const agentRouterCacheTTL = 5 * time.Second

// errNoCapableAgent means that no reachable agent serves the routed app with a
// forwarded identity.
var errNoCapableAgent = errors.New("no capable Application Service agent could be reached")

// errAgentsUnreachable is the errNoCapableAgent of a request whose capable
// agents the Proxy dialed but couldn't reach. Clients get the same error code
// as for the other causes, and the metrics tell them apart.
var errAgentsUnreachable = fmt.Errorf("%w over a reverse tunnel", errNoCapableAgent)

type agentRouterConfig struct {
	// The router lists app servers in the cluster that agents dial.
	agents *agentTransports
	logger *slog.Logger
}

func (c *agentRouterConfig) checkAndSetDefaults() error {
	if c.agents == nil {
		return trace.BadParameter("missing agent transports")
	}
	if c.logger == nil {
		c.logger = slog.Default()
	}
	return nil
}

// agentRouter sends each request to an agent that serves its routed app with a
// forwarded identity. It tries the agents in the rendezvous order of the
// request's recording group, so a group's requests stay on one agent while it
// answers.
type agentRouter struct {
	cfg agentRouterConfig
	// cache holds each route's capable app servers.
	cache *utils.FnCache
}

// newAgentForwarder returns the forwarder for requests whose context holds a
// forwarded identity and the client's address. Its cache of capable app servers
// stops with ctx.
func newAgentForwarder(ctx context.Context, cfg agentRouterConfig) (*reverseproxy.Forwarder, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:         agentRouterCacheTTL,
		Context:     ctx,
		ReloadOnErr: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	router := &agentRouter{cfg: cfg, cache: cache}
	fwd, err := newForwarder(router, writeAgentRouterError, cfg.logger)
	return fwd, trace.Wrap(err)
}

// RoundTrip sends req to the first capable agent that answers. It moves on to
// the next agent only when a dial fails because the agent can't be reached.
func (r *agentRouter) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := r.roundTrip(req)
	result := sharedForwardResult(err)
	r.cfg.agents.cfg.metrics.countRequest(routeSNI, result)
	// The forwarder logs the error too, but without the app or the client, and
	// at Error for a client that gave up.
	if err != nil && result != canceledResult {
		r.logFailure(req, result, err)
	}
	return resp, err
}

// logFailure logs a request with result that the shared transport couldn't
// forward because of err.
func (r *agentRouter) logFailure(req *http.Request, result string, err error) {
	attrs := []slog.Attr{slog.String("result", result)}
	if user, userErr := authz.UserFromContext(req.Context()); userErr == nil {
		identity := user.GetIdentity()
		attrs = append(attrs,
			slog.String("user", identity.Username),
			slog.String("app", identity.RouteToApp.Name),
			slog.String("cluster", identity.RouteToApp.ClusterName),
		)
	}
	// The error goes last, since its trace report spans many lines.
	attrs = append(attrs, slog.Any("error", err))
	r.cfg.logger.LogAttrs(req.Context(), slog.LevelWarn, "Failed to forward a request over the shared transport", attrs...)
}

func (r *agentRouter) roundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	// Failing these checks is a Proxy bug, since the Proxy sets the forwarded
	// identity.
	user, err := authz.UserFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "request has no forwarded identity")
	}
	route := user.GetIdentity().RouteToApp
	// The transports reach only this cluster's agents.
	if route.ClusterName != r.cfg.agents.cfg.clusterName {
		return nil, trace.BadParameter("request is routed to cluster %q", route.ClusterName)
	}

	candidates, err := r.capableAppServers(ctx, route)
	if err != nil {
		if isReverseTunnelDownError(err) {
			return nil, fmt.Errorf("%w: %w", errNoCapableAgent, err)
		}
		return nil, trace.Wrap(err)
	}
	if len(candidates) == 0 {
		return nil, trace.Wrap(errNoCapableAgent)
	}

	// A RoundTripper must not modify its request.
	req = req.Clone(ctx)
	// The transport dials its agent itself, so the host is only a placeholder.
	req.URL.Scheme = "https"
	req.URL.Host = constants.APIDomain
	removeAppSessionCookies(req)
	// The transport closes the body when a dial fails, which would stop the
	// next agent reading it. The server closes it when the handler returns.
	if req.Body != nil && req.Body != http.NoBody {
		req.Body = io.NopCloser(req.Body)
	}

	var lastErr error
	for _, server := range candidates {
		resp, err := r.cfg.agents.roundTrip(req, server)
		if err == nil {
			return resp, nil
		}
		// A proxy peering stream reports a broken connection as a connection
		// problem too, after the agent may have acted on the request. Only a
		// failed dial means it received none of it.
		if !errors.As(err, new(agentDialError)) || !isReverseTunnelDownError(err) {
			return nil, trace.Wrap(err, "forwarding to agent %q", server.GetHostID())
		}
		r.cfg.logger.DebugContext(ctx, "Failed to connect to Application Service agent",
			"host_id", server.GetHostID(), "app", route.Name, "error", err)
		lastErr = err
	}
	return nil, fmt.Errorf("%w: %w", errAgentsUnreachable, lastErr)
}

// capableAppServers returns the route's app servers whose agents can serve it
// with a forwarded identity, in the rendezvous order of the route's recording
// group. It fails with appauthz.ErrUnsupportedApplicationType if this Proxy
// can't forward the app, such as after it became an app type that needs an
// app certificate.
func (r *agentRouter) capableAppServers(ctx context.Context, route tlsca.RouteToApp) ([]types.AppServer, error) {
	key := routeKey{clusterName: route.ClusterName, name: route.Name, publicAddr: route.PublicAddr}
	servers, err := r.cache.Get(ctx, key, func(ctx context.Context) ([]types.AppServer, error) {
		// resolve doesn't use the resolver's cache.
		resolver := newAppServerResolver(r.cfg.agents.cfg.clusterGetter, route, route.SessionID, nil)
		_, servers, err := resolver.resolve(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		registered := len(servers)
		// This Proxy's own rule for which apps the forwarded path serves. The
		// rule changes between releases, so this keeps an agent on another
		// version from drawing the Proxy into forwarding an app it excludes.
		servers = slices.DeleteFunc(servers, func(server types.AppServer) bool {
			return !componentfeatures.SupportsAppForwardedIdentityV1(server.GetApp())
		})
		if registered > 0 && len(servers) == 0 {
			return nil, trace.Wrap(appauthz.ErrUnsupportedApplicationType)
		}
		// An agent advertises the feature for an app only when it accepts
		// forwarded identities and its own version of the rule includes the
		// app.
		return slices.DeleteFunc(servers, func(server types.AppServer) bool {
			features := componentfeatures.GetEffectiveServerFeatures(server)
			return !componentfeatures.InAllSets(componentfeatures.FeatureAppForwardedIdentityV1, features)
		}), nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// The cached listing is in the order of whichever request loaded it.
	return orderCandidates(servers, route.SessionID), nil
}

// writeAgentRouterError answers a request that the router couldn't forward.
// The forwarder has logged the error already.
func writeAgentRouterError(w http.ResponseWriter, req *http.Request, err error) {
	if errors.Is(err, errNoCapableAgent) {
		w.Header().Set(common.TeleportAppErrorCodeHeader, string(common.AppErrorCodeNoCapableReplica))
		http.Error(w, "No Application Service instance can currently serve this application without a certificate for the application. Retry, or contact your Teleport administrator.",
			http.StatusServiceUnavailable)
		return
	}
	if errors.Is(err, appauthz.ErrUnsupportedApplicationType) {
		appauthz.WriteForwardedError(w, req, err)
		return
	}
	reverseproxy.DefaultHandler.ServeHTTP(w, req, err)
}
