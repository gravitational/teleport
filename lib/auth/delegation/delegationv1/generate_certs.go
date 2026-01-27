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

package delegationv1

import (
	"context"
	"time"

	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/internal"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
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

	// Read user from the backend to get the current roles and traits.
	user, err := s.userGetter.GetUser(ctx, session.GetSpec().GetUser(), false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Perform another best-effort check to see if the delegated identity still
	// has access to the required resources (i.e. that their roles haven't been
	// changed) so we can surface it early.
	if err := s.bestEffortCheckResourceAccess(
		ctx,
		user,
		session.GetSpec().GetResources(),
	); err != nil {
		return nil, trace.Wrap(err)
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
		switch user.GetType() {
		case types.DelegationUserTypeBot:
			if botName == user.GetBotName() {
				return nil
			}
		default:
			s.logger.ErrorContext(ctx,
				"Unsupported authorized user type",
				"user_type", user.GetType(),
				"delegation_session_id", session.GetMetadata().GetName(),
			)
		}
	}
	return ErrDelegationUnauthorized
}

func (s *SessionService) generateCertificates(
	ctx context.Context,
	authCtx *authz.Context,
	delegatingUser types.User,
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

	var resourceIDs []types.ResourceID
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
		resourceIDs = append(resourceIDs, id)
	}

	checker := services.NewAccessCheckerWithRoleSet(
		&services.AccessInfo{
			Roles:              delegatingUser.GetRoles(),
			Traits:             delegatingUser.GetTraits(),
			AllowedResourceIDs: resourceIDs,
			Username:           delegatingUser.GetName(),
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

	// TODO(boxofrad): Add the Delegation Session ID to the certificate.
	certReq := internal.CertRequest{
		SSHPublicKey: req.GetSshPublicKey(),
		TLSPublicKey: req.GetTlsPublicKey(),

		User:           delegatingUser,
		Traits:         delegatingUser.GetTraits(),
		Checker:        services.NewUnscopedSplitAccessChecker(checker),
		RouteToCluster: clusterName.GetClusterName(),
		TTL:            ttl,

		LoginIP: callerIdentity.LoginIP,
		PinIP:   false, // TODO(boxofrad): it might make sense to set this because bots don't "travel"

		DisallowReissue: false,
		Renewable:       true,
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
		appSession, err := s.appSessionCreator.CreateAppSession(ctx, internal.NewAppSessionRequest{
			NewWebSessionRequest: internal.NewWebSessionRequest{
				User:                 delegatingUser.GetName(),
				LoginIP:              callerIdentity.LoginIP,
				SessionTTL:           ttl,
				Traits:               delegatingUser.GetTraits(),
				Roles:                delegatingUser.GetRoles(),
				RequestedResourceIDs: resourceIDs,
				AttestWebSession:     true,
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

func (s *SessionService) getRoleSet(ctx context.Context, user types.User) (services.RoleSet, error) {
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
