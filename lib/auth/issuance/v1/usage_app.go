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

	cn, err := s.authServer.GetClusterName(ctx)
	if err != nil {
		return trace.Wrap(err, "resolving cluster name")
	}

	// CreateAppSessionFromReq performs some authorization internally, but NOT explicit app-level RBAC.
	// The explicit app-level authorization (CheckAccessToApp) happens at connection time in the App Service.
	ws, err := s.authServer.CreateAppSessionFromReq(ctx, sessionreq.NewAppSessionRequest{
		NewWebSessionRequest: sessionreq.NewWebSessionRequest{
			User:       user.GetName(),
			LoginIP:    currentIdentity.LoginIP,
			SessionTTL: ttl,
			// No Traits, Roles, AccessRequests, or RequestedResourceAccessIDs:
			// scoped bots have none, and CreateAppSessionFromReq rejects
			// resource access IDs for scope-pinned identities.

			// AttestWebSession must be true so that clusters enforcing a
			// hardware private key policy (e.g. hardware_key_touch) do not
			// reject this session. The session's private key is generated
			// and held exclusively by the Auth server — marking it as a
			// web_session attestation tells GenerateUserCerts that no
			// physical hardware key is needed to satisfy the policy.
			// Without this, CreateAppSessionFromReq would fail with a
			// PrivateKeyPolicyError on any cluster with a non-"none" policy.

			AttestWebSession: true,
		},

		AppName:     usageApp.GetName(),
		PublicAddr:  usageApp.GetPublicAddr(),
		ClusterName: cn.GetClusterName(),
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
	certReq.AppClusterName = cn.GetClusterName()
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
		return trace.BadParameter("tls_public_key: is required for app usage")
	}

	return nil
}
