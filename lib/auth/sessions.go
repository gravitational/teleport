/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"context"
	"math/rand"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// CreateAppSession creates an application session. Application sessions
// are only created if the calling identity has access to the application requested.
func (s *AuthServer) CreateAppSession(ctx context.Context, req services.CreateAppSessionRequest, user services.User, checker services.AccessChecker) (services.AppSession, error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Fetch the application the caller is requesting.
	app, server, err := s.getApp(ctx, req.PublicAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check if the caller has access to the requested application.
	if err := checker.CheckAccessToApp(server.GetNamespace(), app); err != nil {
		log.Warnf("Access to %v denied: %v.", req.PublicAddr, err)
		// TODO(russjones): Hook audit log here.

		return nil, trace.AccessDenied("access denied")
	}

	// Synchronize expiration of JWT token and application session.
	ttl := checker.AdjustSessionTTL(defaults.CertDuration)
	expires := s.clock.Now().Add(ttl)

	// Create a new application session.
	session, err := services.NewAppSession(expires, services.AppSessionSpecV3{
		PublicAddr: req.PublicAddr,
		Username:   user.GetName(),
		Roles:      user.GetRoles(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.UpsertAppSession(ctx, session); err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf("Generated application session for %v for %v with TTL %v.", req.PublicAddr, user.GetName(), ttl)

	return session, nil
}

// getApp looks for an application registered for the requested public address
// in the cluster and returns it. In the situation multiple applications match,
// a random selection is returned. This is done on purpose to support HA to
// allow multiple application proxy nodes to be run and if one is down, at
// least the application can be accessible on the other.
//
// In the future this function should be updated to keep state on application
// servers that are down and to not route requests to that server.
func (s *AuthServer) getApp(ctx context.Context, publicAddr string) (*services.App, services.Server, error) {
	var am []*services.App
	var sm []services.Server

	servers, err := s.GetAppServers(ctx, defaults.Namespace)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	for _, server := range servers {
		for _, app := range server.GetApps() {
			if app.PublicAddr == publicAddr {
				am = append(am, app)
				sm = append(sm, server)
			}
		}
	}

	if len(am) == 0 {
		return nil, nil, trace.NotFound("%q not found", publicAddr)
	}
	index := rand.Intn(len(am))
	return am[index], sm[index], nil
}
