/*
Copyright 2021 Gravitational, Inc.

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
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// CreateAppSession creates and inserts a services.WebSession into the
// backend with the identity of the caller used to generate the certificate.
// The certificate is used for all access requests, which is where access
// control is enforced.
func (s *Server) CreateAppSession(ctx context.Context, req types.CreateAppSessionRequest, user types.User, identity tlsca.Identity, checker services.AccessChecker) (types.WebSession, error) {
	if !modules.GetModules().Features().App {
		return nil, trace.AccessDenied(
			"this Teleport cluster is not licensed for application access, please contact the cluster administrator")
	}

	// Don't let the app session go longer than the identity expiration,
	// which matches the parent web session TTL as well.
	//
	// When using web-based app access, the browser will send a cookie with
	// sessionID which will be used to fetch services.WebSession which
	// contains a certificate whose life matches the life of the session
	// that will be used to establish the connection.
	ttl := checker.AdjustSessionTTL(identity.Expires.Sub(s.clock.Now()))

	// Encode user traits in the app access certificate. This will allow to
	// pass user traits when talking to app servers in leaf clusters.
	_, traits, err := services.ExtractFromIdentity(s, identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create certificate for this session.
	privateKey, publicKey, err := native.GenerateKeyPair()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certs, err := s.generateUserCert(certRequest{
		user:           user,
		loginIP:        identity.LoginIP,
		publicKey:      publicKey,
		checker:        checker,
		ttl:            ttl,
		traits:         traits,
		activeRequests: services.RequestIDs{AccessRequests: identity.ActiveRequests},
		// Only allow this certificate to be used for applications.
		usage: []string{teleport.UsageAppsOnly},
		// Add in the application routing information.
		appSessionID:      uuid.New().String(),
		appPublicAddr:     req.PublicAddr,
		appClusterName:    req.ClusterName,
		awsRoleARN:        req.AWSRoleARN,
		azureIdentity:     req.AzureIdentity,
		gcpServiceAccount: req.GCPServiceAccount,
		// Since we are generating the keys and certs directly on the Auth Server,
		// we need to skip attestation.
		skipAttestation: true,
		// Pass along device extensions from the user.
		deviceExtensions: DeviceExtensions(identity.DeviceExtensions),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create services.WebSession for this session.
	sessionID, err := utils.CryptoRandomHex(SessionTokenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	bearer, err := utils.CryptoRandomHex(SessionTokenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	session, err := types.NewWebSession(sessionID, types.KindAppSession, types.WebSessionSpecV2{
		User:        req.Username,
		Priv:        privateKey,
		Pub:         certs.SSH,
		TLSCert:     certs.TLS,
		Expires:     s.clock.Now().Add(ttl),
		BearerToken: bearer,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = s.UpsertAppSession(ctx, session); err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf("Generated application web session for %v with TTL %v.", req.Username, ttl)
	UserLoginCount.Inc()
	return session, nil
}

// WaitForAppSession will block until the requested application session shows up in the
// cache or a timeout occurs.
func WaitForAppSession(ctx context.Context, sessionID, user string, ap ReadProxyAccessPoint) error {
	req := waitForWebSessionReq{
		newWatcherFn: ap.NewWatcher,
		getSessionFn: func(ctx context.Context, sessionID string) (types.WebSession, error) {
			return ap.GetAppSession(ctx, types.GetAppSessionRequest{SessionID: sessionID})
		},
	}
	return trace.Wrap(waitForWebSession(ctx, sessionID, user, types.KindAppSession, req))
}

// WaitForSnowflakeSession waits until the requested Snowflake session shows up int the cache
// or a timeout occurs.
func WaitForSnowflakeSession(ctx context.Context, sessionID, user string, ap SnowflakeSessionWatcher) error {
	req := waitForWebSessionReq{
		newWatcherFn: ap.NewWatcher,
		getSessionFn: func(ctx context.Context, sessionID string) (types.WebSession, error) {
			return ap.GetSnowflakeSession(ctx, types.GetSnowflakeSessionRequest{SessionID: sessionID})
		},
	}
	return trace.Wrap(waitForWebSession(ctx, sessionID, user, types.KindSnowflakeSession, req))
}

// waitForWebSessionReq is a request to wait for web session to be populated in the application cache.
type waitForWebSessionReq struct {
	// newWatcherFn is a function that returns new event watcher.
	newWatcherFn func(ctx context.Context, watch types.Watch) (types.Watcher, error)
	// getSessionFn is a function that returns web session by given ID.
	getSessionFn func(ctx context.Context, sessionID string) (types.WebSession, error)
}

// waitForWebSession is an implementation for web session wait functions.
func waitForWebSession(ctx context.Context, sessionID, user string, evenSubKind string, req waitForWebSessionReq) error {
	_, err := req.getSessionFn(ctx, sessionID)
	if err == nil {
		return nil
	}
	logger := log.WithField("session", sessionID)
	if !trace.IsNotFound(err) {
		logger.WithError(err).Debug("Failed to query web session.")
	}
	// Establish a watch on application session.
	watcher, err := req.newWatcherFn(ctx, types.Watch{
		Name: teleport.ComponentAppProxy,
		Kinds: []types.WatchKind{
			{
				Kind:    types.KindWebSession,
				SubKind: evenSubKind,
				Filter:  (&types.WebSessionFilter{User: user}).IntoMap(),
			},
		},
		MetricComponent: teleport.ComponentAppProxy,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()
	matchEvent := func(event types.Event) (types.Resource, error) {
		if event.Type == types.OpPut &&
			event.Resource.GetKind() == types.KindWebSession &&
			event.Resource.GetSubKind() == evenSubKind &&
			event.Resource.GetName() == sessionID {
			return event.Resource, nil
		}
		return nil, trace.CompareFailed("no match")
	}
	_, err = local.WaitForEvent(ctx, watcher, local.EventMatcherFunc(matchEvent), clockwork.NewRealClock())
	if err != nil {
		logger.WithError(err).Warn("Failed to wait for web session.")
		// See again if we maybe missed the event but the session was actually created.
		if _, err := req.getSessionFn(ctx, sessionID); err == nil {
			return nil
		}
	}
	return trace.Wrap(err)
}

// generateAppToken generates an JWT token that will be passed along with every
// application request.
func (s *Server) generateAppToken(ctx context.Context, username string, roles []string, traits map[string][]string, uri string, expires time.Time) (string, error) {
	// Get the clusters CA.
	clusterName, err := s.GetDomainName()
	if err != nil {
		return "", trace.Wrap(err)
	}
	ca, err := s.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.JWTSigner,
		DomainName: clusterName,
	}, true)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Filter out empty traits so the resulting JWT doesn't have a bunch of
	// entries with nil values.
	filteredTraits := map[string][]string{}
	for trait, values := range traits {
		if len(values) > 0 {
			filteredTraits[trait] = values
		}
	}

	// Extract the JWT signing key and sign the claims.
	signer, err := s.GetKeyStore().GetJWTSigner(ctx, ca)
	if err != nil {
		return "", trace.Wrap(err)
	}
	privateKey, err := services.GetJWTSigner(signer, ca.GetClusterName(), s.clock)
	if err != nil {
		return "", trace.Wrap(err)
	}
	token, err := privateKey.Sign(jwt.SignParams{
		Username: username,
		Roles:    roles,
		Traits:   filteredTraits,
		URI:      uri,
		Expires:  expires,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	return token, nil
}

func (s *Server) CreateWebSessionFromReq(ctx context.Context, req types.NewWebSessionRequest) (types.WebSession, error) {
	session, err := s.NewWebSession(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.upsertWebSession(ctx, req.User, session)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

func (s *Server) CreateSessionCert(user types.User, sessionTTL time.Duration, publicKey []byte, compatibility, routeToCluster, kubernetesCluster, loginIP string, attestationReq *keys.AttestationStatement) ([]byte, []byte, error) {
	// It's safe to extract the access info directly from services.User because
	// this occurs during the initial login before the first certs have been
	// generated, so there's no possibility of any active access requests.
	accessInfo := services.AccessInfoFromUser(user)
	clusterName, err := s.GetClusterName()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	checker, err := services.NewAccessChecker(accessInfo, clusterName.GetClusterName(), s)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	certs, err := s.generateUserCert(certRequest{
		user:                 user,
		ttl:                  sessionTTL,
		publicKey:            publicKey,
		compatibility:        compatibility,
		checker:              checker,
		traits:               user.GetTraits(),
		routeToCluster:       routeToCluster,
		kubernetesCluster:    kubernetesCluster,
		attestationStatement: attestationReq,
		loginIP:              loginIP,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return certs.SSH, certs.TLS, nil
}

func (s *Server) CreateSnowflakeSession(ctx context.Context, req types.CreateSnowflakeSessionRequest,
	identity tlsca.Identity, checker services.AccessChecker,
) (types.WebSession, error) {
	if !modules.GetModules().Features().DB {
		return nil, trace.AccessDenied(
			"this Teleport cluster is not licensed for database access, please contact the cluster administrator")
	}

	// Don't let the app session go longer than the identity expiration,
	// which matches the parent web session TTL as well.
	//
	// When using web-based app access, the browser will send a cookie with
	// sessionID which will be used to fetch services.WebSession which
	// contains a certificate whose life matches the life of the session
	// that will be used to establish the connection.
	ttl := checker.AdjustSessionTTL(identity.Expires.Sub(s.clock.Now()))

	// Create services.WebSession for this session.
	sessionID, err := utils.CryptoRandomHex(SessionTokenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	session, err := types.NewWebSession(sessionID, types.KindSnowflakeSession, types.WebSessionSpecV2{
		User:               req.Username,
		Expires:            s.clock.Now().Add(ttl),
		BearerToken:        req.SessionToken,
		BearerTokenExpires: s.clock.Now().Add(req.TokenTTL),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = s.UpsertSnowflakeSession(ctx, session); err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf("Generated Snowflake web session for %v with TTL %v.", req.Username, ttl)

	return session, nil
}

func (s *Server) CreateSAMLIdPSession(ctx context.Context, req types.CreateSAMLIdPSessionRequest,
	identity tlsca.Identity, checker services.AccessChecker,
) (types.WebSession, error) {
	// TODO(mdwn): implement a module.Features() check.

	if req.SAMLSession == nil {
		return nil, trace.BadParameter("required SAML session is not populated")
	}

	// Create services.WebSession for this session.
	session, err := types.NewWebSession(req.SessionID, types.KindSAMLIdPSession, types.WebSessionSpecV2{
		User:        req.Username,
		Expires:     req.SAMLSession.ExpireTime,
		SAMLSession: req.SAMLSession,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = s.UpsertSAMLIdPSession(ctx, session); err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf("Generated SAML IdP web session for %v.", req.Username)

	return session, nil
}
