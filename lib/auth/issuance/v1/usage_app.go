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

package issuancev1

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"

	issuancev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/issuance/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/internal/cert"
	sessionreq "github.com/gravitational/teleport/lib/auth/internal/session"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/tlsca"
)

// applyUsageApp will create a new scoped session with the application and
// append to the cert.Request the relevant information.
func (s *Service) applyUsageApp(
	ctx context.Context,
	user types.User,
	botScope string,
	currentIdentity tlsca.Identity,
	usageApp *issuancev1pb.UsageApp,
	ttl time.Duration,
	certReq *cert.Request,
) error {
	// Ensure the current identity has scope to perform this operation.
	pinnedScope := currentIdentity.ScopePin.GetScope()
	if !scopes.ResourceScope(usageApp.GetScope()).IsSubjectToScopeOfEffect(pinnedScope) {
		return trace.AccessDenied("app scope %q is not equivalent to or descendant of pinned scope %q", usageApp.GetScope(), pinnedScope)
	}

	// Fetch the application to ensure it exists and that its scope matches.
	// This is defense-in-depth and UX: it prevents issuing a cert+session
	// that can never be used, and surfaces a clear error at issuance time
	// rather than a confusing transport-layer failure later.
	var app types.Application
	for server, err := range s.authServer.RangeApplicationServersWithName(ctx, usageApp.GetName()) {
		if err != nil {
			return trace.Wrap(err)
		}
		// It requires the exact scope match:
		// If the UsageApp refers to /staging::abc, it should not match /stating/team-1::abc.
		if server.GetScope() != usageApp.GetScope() {
			continue
		}
		app = server.GetApp()
		break
	}
	if app == nil {
		return trace.NotFound("application %q in scope %q not found", usageApp.GetName(), usageApp.GetScope())
	}

	// Reject Identity Center account apps — they have special lifecycle
	// management and should not be accessed through scoped bot certificates.
	if app.GetSubKind() == types.KindIdentityCenterAccount {
		return trace.BadParameter("Identity Center account applications cannot be accessed via scoped bot certificates")
	}

	// Create a new session.
	ws, err := s.authServer.CreateAppSessionFromReq(ctx, sessionreq.NewAppSessionRequest{
		NewWebSessionRequest: sessionreq.NewWebSessionRequest{
			User:       user.GetName(),
			LoginIP:    currentIdentity.LoginIP,
			SessionTTL: ttl,
			// No Traits, Roles, AccessRequests, or RequestedResourceAccessIDs:
			// scoped bots have none, and CreateAppSessionFromReq rejects
			// resource access IDs for scope-pinned identities.
			AttestWebSession: false,
		},

		AppName:     usageApp.GetName(),
		PublicAddr:  usageApp.GetPublicAddr(),
		ClusterName: usageApp.GetClusterName(),
		TargetScope: usageApp.GetScope(),

		// Identity drives the scoped access checker context.
		Identity:      currentIdentity,
		BotName:       currentIdentity.BotName,
		BotInstanceID: currentIdentity.BotInstanceID,
		BotScope:      botScope,
	})
	if err != nil {
		return trace.Wrap(err, "creating app session")
	}

	// Annotate the certificate with the session information.
	certReq.Usage = []string{teleport.UsageAppsOnly}
	certReq.AppSessionID = ws.GetName()
	certReq.AppName = usageApp.GetName()
	certReq.AppPublicAddr = usageApp.GetPublicAddr()
	certReq.AppClusterName = usageApp.GetClusterName()
	certReq.TargetScope = usageApp.GetScope()

	return nil
}

func validateUsageApp(req *issuancev1pb.IssueScopedBotCertsRequest) error {
	app := req.GetApp()
	if app.GetName() == "" {
		return trace.BadParameter("app.name: is required")
	}
	if err := scopes.StrongValidate(app.GetScope()); err != nil {
		return trace.Wrap(err, "app.scope")
	}
	if len(req.GetTlsPublicKey()) == 0 {
		return trace.BadParameter("tls_public_key is required for app usage")
	}

	return nil
}
