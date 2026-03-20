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

package delegationv1

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/accessrequest"
	"github.com/gravitational/teleport/api/client/proto"
	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/internal/cert"
	sessionreq "github.com/gravitational/teleport/lib/auth/internal/session"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/set"
	"github.com/gravitational/teleport/lib/utils/slices"
)

// ErrDelegationUnauthorized is returned from the GenerateCerts handler when
// a given delegation either doesn't exist, has expired, or the user is not
// authorized to use it. We return the same error in all cases to avoid leaking
// information about which sessions exist.
var ErrDelegationUnauthorized = &trace.AccessDeniedError{
	Message: "Delegation session does not exist, has expired, or you are not authorized to use it",
}

// GenerateCerts generates TLS and/or SSH certificates, scoped to a delegation
// session.
//
// Note: this endpoint does not support remote/trusted clusters.
func (s *SessionService) GenerateCerts(
	ctx context.Context,
	req *delegationv1.GenerateCertsRequest,
) (*delegationv1.GenerateCertsResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	session, err := s.sessionReader.GetDelegationSession(ctx, req.GetDelegationSessionId())
	switch {
	case trace.IsNotFound(err):
		// Treat NotFound as an AccessDenied error to avoid leaking whether the
		// session ID was valid or not.
		return nil, ErrDelegationUnauthorized
	case err != nil:
		return nil, trace.Wrap(err)
	}

	// Explicitly check the expiry in case the backend hasn't purged expired
	// items yet.
	if session.GetMetadata().GetExpires().AsTime().Before(time.Now()) {
		return nil, ErrDelegationUnauthorized
	}

	// Check the bot is in the session's authorized users.
	if err := s.authorizeSession(ctx, authCtx, session); err != nil {
		return nil, err
	}

	// Validate the request.
	if req.GetSshPublicKey() == nil && req.GetTlsPublicKey() == nil {
		return nil, trace.BadParameter("at least one of ssh_public_key or tls_public_key is required")
	}
	if req.GetExpires() == nil {
		return nil, trace.BadParameter("expires: is required")
	}
	if !time.Now().Before(req.GetExpires().AsTime()) {
		return nil, trace.BadParameter("expires: must be in the future")
	}

	// Read user login state from the backend to get current roles, traits, and
	// any enriched identity from external providers (e.g. GitHub). Falls back
	// to the plain user if no login state exists.
	user, err := services.GetUserOrLoginState(ctx, s.userGetter, session.GetSpec().GetUser())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !wildcardPermissions(session.GetSpec().GetResources()) {
		// Perform a best-effort check to see if the delegated identity has
		// access to the required resources (i.e. that their roles haven't been
		// changed since the session was created) so we can surface it early.
		if err := s.bestEffortCheckResourceAccess(
			ctx,
			user,
			session.GetSpec().GetResources(),
		); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	certs, err := s.generateCertificates(
		ctx,
		authCtx,
		user,
		session,
		req,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return certs, nil
}

func (s *SessionService) authorizeSession(
	ctx context.Context,
	authCtx *authz.Context,
	session *delegationv1.DelegationSession,
) error {
	botName := authCtx.Identity.GetIdentity().BotName
	if botName == "" {
		return ErrDelegationUnauthorized
	}

	for _, user := range session.GetSpec().GetAuthorizedUsers() {
		switch user.GetKind() {
		case types.KindBot:
			if botName == user.GetBotName() {
				return nil
			}
		default:
			s.logger.ErrorContext(ctx,
				"Unsupported authorized user kind",
				"user_kind", user.GetKind(),
				"delegation_session_id", session.GetMetadata().GetName(),
			)
		}
	}
	return ErrDelegationUnauthorized
}

func (s *SessionService) generateCertificates(
	ctx context.Context,
	authCtx *authz.Context,
	delegatingUser services.UserState,
	session *delegationv1.DelegationSession,
	req *delegationv1.GenerateCertsRequest,
) (*delegationv1.GenerateCertsResponse, error) {
	clusterName, err := s.clusterNameGetter.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	roleSet, err := s.getRoleSet(ctx, delegatingUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var resourceIDs []types.ResourceAccessID

	if !wildcardPermissions(session.GetSpec().GetResources()) {
		for _, res := range session.GetSpec().GetResources() {
			// TODO(boxofrad): support kubernetes resources, which have constraints
			// that may need to "compile down" into many ResourceIDs.
			if res.GetKind() == types.KindKubernetesCluster {
				return nil, trace.NotImplemented("Support for delegating access to Kubernetes resources is not yet implemented")
			}
			id := types.ResourceID{
				ClusterName: clusterName.GetClusterName(),
				Kind:        res.GetKind(),
				Name:        res.GetName(),
			}
			resourceIDs = append(resourceIDs, types.ResourceAccessID{Id: id})
		}
	}

	checker := services.NewAccessCheckerWithRoleSet(
		&services.AccessInfo{
			Roles:                    delegatingUser.GetRoles(),
			Traits:                   delegatingUser.GetTraits(),
			AllowedResourceAccessIDs: resourceIDs,
			Username:                 delegatingUser.GetName(),
		},
		clusterName.GetClusterName(),
		roleSet,
	)

	ttl := min(
		time.Until(req.GetExpires().AsTime()),
		time.Until(session.GetMetadata().GetExpires().AsTime()),
		defaults.MaxRenewableCertTTL,
	)

	callerIdentity := authCtx.Identity.GetIdentity()

	certReq := cert.Request{
		SSHPublicKey: req.GetSshPublicKey(),
		TLSPublicKey: req.GetTlsPublicKey(),

		User:           delegatingUser,
		Traits:         delegatingUser.GetTraits(),
		CheckerContext: services.NewUnscopedSplitAccessCheckerContext(checker),
		RouteToCluster: clusterName.GetClusterName(),
		TTL:            ttl,

		LoginIP: callerIdentity.LoginIP,
		PinIP:   false, // TODO(boxofrad): it might make sense to set this because bots don't "travel"

		DisallowReissue: true,
		Renewable:       false,
		IncludeHostCA:   true,

		BotName:       callerIdentity.BotName,
		BotInstanceID: callerIdentity.BotInstanceID,
	}

	// Add the protocol-specific routing hints to the certificate.
	switch routing := req.Routing.(type) {
	case *delegationv1.GenerateCertsRequest_KubernetesCluster:
		certReq.KubernetesCluster = routing.KubernetesCluster
	case *delegationv1.GenerateCertsRequest_RouteToApp:
		route := routing.RouteToApp

		certReq.AppPublicAddr = route.GetPublicAddr()
		certReq.AppClusterName = route.GetClusterName()
		certReq.AppName = route.GetName()
		certReq.AppURI = route.GetUri()
		certReq.AppTargetPort = int(route.GetTargetPort())
		// TODO(boxofrad): Figure out what to with the AWSCredentialProcessCredentials field.
		certReq.AWSRoleARN = route.GetAwsRoleArn()
		certReq.AzureIdentity = route.GetAzureIdentity()
		certReq.GCPServiceAccount = route.GetGcpServiceAccount()

		// Start an application access session.
		appSession, err := s.appSessionCreator.CreateAppSession(ctx, sessionreq.NewAppSessionRequest{
			NewWebSessionRequest: sessionreq.NewWebSessionRequest{
				User:                       delegatingUser.GetName(),
				LoginIP:                    callerIdentity.LoginIP,
				SessionTTL:                 ttl,
				Traits:                     delegatingUser.GetTraits(),
				Roles:                      delegatingUser.GetRoles(),
				RequestedResourceAccessIDs: resourceIDs,
				AttestWebSession:           true,
			},
			PublicAddr:        certReq.AppPublicAddr,
			ClusterName:       certReq.AppClusterName,
			AWSRoleARN:        certReq.AWSRoleARN,
			AzureIdentity:     certReq.AzureIdentity,
			GCPServiceAccount: certReq.GCPServiceAccount,

			// TODO(boxofrad): Figure out what to do with MFAVerified and DeviceExtensions.
			AppName:       certReq.AppName,
			AppURI:        certReq.AppURI,
			AppTargetPort: certReq.AppTargetPort,
			BotName:       callerIdentity.BotName,
			BotInstanceID: certReq.BotInstanceID,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certReq.AppSessionID = appSession.GetName()
	case *delegationv1.GenerateCertsRequest_RouteToDatabase:
		route := routing.RouteToDatabase

		certReq.DBService = route.GetServiceName()
		certReq.DBProtocol = route.GetProtocol()
		certReq.DBUser = route.GetUsername()
		certReq.DBName = route.GetDatabase()
		certReq.DBRoles = route.GetRoles()
	}

	certs, err := s.certGenerator.Generate(ctx, certReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &delegationv1.GenerateCertsResponse{
		Ssh:    certs.SSH,
		Tls:    certs.TLS,
		SshCas: certs.SSHCACerts,
		TlsCas: certs.TLSCACerts,
	}, nil
}

func (s *SessionService) getRoleSet(ctx context.Context, user services.UserState) (services.RoleSet, error) {
	roleSet := make(services.RoleSet, len(user.GetRoles()))
	for idx, roleName := range user.GetRoles() {
		role, err := s.roleGetter.GetRole(ctx, roleName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		role, err = services.ApplyTraits(role, user.GetTraits())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roleSet[idx] = role
	}
	return roleSet, nil
}

func wildcardPermissions(specs []*delegationv1.DelegationResourceSpec) bool {
	for _, spec := range specs {
		if spec.GetKind() == types.Wildcard && spec.GetName() == types.Wildcard {
			return true
		}
	}
	return false
}

// bestEffortCheckResourceAccess makes a best-effort attempt to check a user's
// access to the given resources. It is not strictly required as the RBAC engine
// will check resource access at time-of-use, but enables us to surface permission
// errors as early as possible.
func (s *SessionService) bestEffortCheckResourceAccess(
	ctx context.Context,
	user services.UserState,
	resources []*delegationv1.DelegationResourceSpec,
) error {
	checker, err := services.NewAccessChecker(
		&services.AccessInfo{
			Roles:  user.GetRoles(),
			Traits: user.GetTraits(),
		},
		"",
		s.roleGetter,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	resourceNamesByKind := make(map[string]set.Set[string])
	for _, res := range resources {
		byKind, ok := resourceNamesByKind[res.GetKind()]
		if !ok {
			byKind = set.New[string]()
			resourceNamesByKind[res.GetKind()] = byKind
		}
		byKind.Add(res.GetName())
	}

	resourcesByKindName := make(map[string]map[string]types.ResourceWithLabels)
	for kind, resourceNames := range resourceNamesByKind {
		req := proto.ListResourcesRequest{
			PredicateExpression: strings.Join(
				slices.Map(
					resourceNames.Elements(),
					func(name string) string {
						return fmt.Sprintf(`resource.metadata.name == %q`, name)
					},
				),
				" || ",
			),
			Limit: int32(len(resourceNames)),
		}

		rsp, err := accessrequest.GetResourcesByKind(ctx, s.resourceLister, req, kind)
		if err != nil {
			return trace.Wrap(err)
		}

		byName := make(map[string]types.ResourceWithLabels)
		for _, res := range rsp {
			byName[res.GetName()] = res
		}
		resourcesByKindName[kind] = byName
	}

	unauthorizedResources := set.New[string]()
	for _, spec := range resources {
		id := fmt.Sprintf("%s/%s", spec.GetKind(), spec.GetName())

		byName, ok := resourcesByKindName[spec.GetKind()]
		if !ok {
			unauthorizedResources.Add(id)
			continue
		}

		res, ok := byName[spec.GetName()]
		if !ok {
			unauthorizedResources.Add(id)
			continue
		}

		if err := checker.CheckAccess(res, services.AccessState{MFAVerified: true}); err != nil {
			unauthorizedResources.Add(id)
			continue
		}
	}

	if unauthorizedResources.Len() != 0 {
		idStrings := unauthorizedResources.Elements()
		sort.Strings(idStrings)
		return trace.AccessDenied("User does not have permission to delegate access to all of the required resources, missing resources: [%s]", strings.Join(idStrings, ", "))
	}

	return nil
}
