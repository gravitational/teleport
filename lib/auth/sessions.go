/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// NewWebSessionRequest defines a request to create a new user
// web session
type NewWebSessionRequest struct {
	// User specifies the user this session is bound to
	User string
	// LoginIP is an observed IP of the client, it will be embedded into certificates.
	LoginIP string
	// Roles optionally lists additional user roles
	Roles []string
	// Traits optionally lists role traits
	Traits map[string][]string
	// SessionTTL optionally specifies the session time-to-live.
	// If left unspecified, the default certificate duration is used.
	SessionTTL time.Duration
	// LoginTime is the time that this user recently logged in.
	LoginTime time.Time
	// AccessRequests contains the UUIDs of the access requests currently in use.
	AccessRequests []string
	// RequestedResourceIDs optionally lists requested resources
	RequestedResourceIDs []types.ResourceID
	// AttestWebSession optionally attests the web session to meet private key policy requirements.
	// This should only be set to true for web sessions that are purely in the purview of the Proxy
	// and Auth services. Users should never have direct access to attested web sessions.
	AttestWebSession bool
	// PrivateKey is a specific private key to use when generating the web sessions' certificates.
	// This should be provided when extending an attested web session in order to maintain the
	// session attested status.
	PrivateKey *keys.PrivateKey
}

// CheckAndSetDefaults validates the request and sets defaults.
func (r *NewWebSessionRequest) CheckAndSetDefaults() error {
	if r.User == "" {
		return trace.BadParameter("user name required")
	}
	if len(r.Roles) == 0 {
		return trace.BadParameter("roles required")
	}
	if len(r.Traits) == 0 {
		return trace.BadParameter("traits required")
	}
	if r.SessionTTL == 0 {
		r.SessionTTL = apidefaults.CertDuration
	}
	return nil
}

func (a *Server) CreateWebSessionFromReq(ctx context.Context, req NewWebSessionRequest) (types.WebSession, error) {
	session, err := a.newWebSession(ctx, req, nil /* opts */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = a.upsertWebSession(ctx, session)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

// newWebSessionOpts are WebSession creation options exclusive to Auth.
// These options complement [types.NewWebSessionRequest].
// See [Server.newWebSession].
type newWebSessionOpts struct {
	// deviceExtensions are the device extensions to apply to the session.
	// Only present on renewals, the original extensions are applied by
	// [Server.AugmentWebSessionCertificates].
	deviceExtensions *tlsca.DeviceExtensions
}

// newWebSession creates and returns a new web session for the specified request
func (a *Server) newWebSession(
	ctx context.Context,
	req NewWebSessionRequest,
	opts *newWebSessionOpts,
) (types.WebSession, error) {
	userState, err := a.GetUserOrLoginState(ctx, req.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if req.LoginIP == "" {
		// TODO(antonam): consider turning this into error after all use cases are covered (before v14.0 testplan)
		log.Debug("Creating new web session without login IP specified.")
	}
	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checker, err := services.NewAccessChecker(&services.AccessInfo{
		Roles:              req.Roles,
		Traits:             req.Traits,
		AllowedResourceIDs: req.RequestedResourceIDs,
	}, clusterName.GetClusterName(), a)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	netCfg, err := a.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.PrivateKey == nil {
		req.PrivateKey, err = native.GeneratePrivateKey()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	sessionTTL := req.SessionTTL
	if sessionTTL == 0 {
		sessionTTL = checker.AdjustSessionTTL(apidefaults.CertDuration)
	}

	if req.AttestWebSession {
		// Upsert web session attestation data so that this key's certs
		// will be marked with the web session private key policy.
		webAttData, err := services.NewWebSessionAttestationData(req.PrivateKey.Public())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err = a.UpsertKeyAttestationData(ctx, webAttData, sessionTTL); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	certReq := certRequest{
		user:           userState,
		loginIP:        req.LoginIP,
		ttl:            sessionTTL,
		publicKey:      req.PrivateKey.MarshalSSHPublicKey(),
		checker:        checker,
		traits:         req.Traits,
		activeRequests: services.RequestIDs{AccessRequests: req.AccessRequests},
	}
	var hasDeviceExtensions bool
	if opts != nil && opts.deviceExtensions != nil {
		// Apply extensions to request.
		certReq.deviceExtensions = DeviceExtensions(*opts.deviceExtensions)
		hasDeviceExtensions = true
	}

	certs, err := a.generateUserCert(ctx, certReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	token, err := utils.CryptoRandomHex(defaults.SessionTokenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	bearerToken, err := utils.CryptoRandomHex(defaults.SessionTokenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	bearerTokenTTL := min(sessionTTL, defaults.BearerTokenTTL)

	startTime := a.clock.Now()
	if !req.LoginTime.IsZero() {
		startTime = req.LoginTime
	}

	sessionSpec := types.WebSessionSpecV2{
		User:                req.User,
		Priv:                req.PrivateKey.PrivateKeyPEM(),
		Pub:                 certs.SSH,
		TLSCert:             certs.TLS,
		Expires:             startTime.UTC().Add(sessionTTL),
		BearerToken:         bearerToken,
		BearerTokenExpires:  startTime.UTC().Add(bearerTokenTTL),
		LoginTime:           req.LoginTime,
		IdleTimeout:         types.Duration(netCfg.GetWebIdleTimeout()),
		HasDeviceExtensions: hasDeviceExtensions,
	}
	UserLoginCount.Inc()

	sess, err := types.NewWebSession(token, types.KindWebSession, sessionSpec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return sess, nil
}

func (a *Server) upsertWebSession(ctx context.Context, session types.WebSession) error {
	if err := a.WebSessions().Upsert(ctx, session); err != nil {
		return trace.Wrap(err)
	}
	token, err := types.NewWebToken(session.GetBearerTokenExpiryTime(), types.WebTokenSpecV3{
		User:  session.GetUser(),
		Token: session.GetBearerToken(),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if err := a.WebTokens().Upsert(ctx, token); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CreateAppSession creates and inserts a services.WebSession into the
// backend with the identity of the caller used to generate the certificate.
// The certificate is used for all access requests, which is where access
// control is enforced.
func (a *Server) CreateAppSession(ctx context.Context, req *proto.CreateAppSessionRequest, user services.UserState, identity tlsca.Identity, checker services.AccessChecker) (types.WebSession, error) {
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
	ttl := checker.AdjustSessionTTL(identity.Expires.Sub(a.clock.Now()))

	// Encode user traits in the app access certificate. This will allow to
	// pass user traits when talking to app servers in leaf clusters.
	_, traits, err := services.ExtractFromIdentity(ctx, a, identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var verifiedMFADeviceID string
	if req.MFAResponse != nil {
		requiredExt := &mfav1.ChallengeExtensions{Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION}
		mfaData, err := a.ValidateMFAAuthResponse(ctx, req.GetMFAResponse(), req.Username, requiredExt)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		verifiedMFADeviceID = mfaData.Device.Id
	}

	// Create certificate for this session.
	privateKey, publicKey, err := native.GenerateKeyPair()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certs, err := a.generateUserCert(ctx, certRequest{
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
		mfaVerified:      verifiedMFADeviceID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create services.WebSession for this session.
	sessionID, err := utils.CryptoRandomHex(defaults.SessionTokenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	bearer, err := utils.CryptoRandomHex(defaults.SessionTokenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	session, err := types.NewWebSession(sessionID, types.KindAppSession, types.WebSessionSpecV2{
		User:        req.Username,
		Priv:        privateKey,
		Pub:         certs.SSH,
		TLSCert:     certs.TLS,
		LoginTime:   a.clock.Now().UTC(),
		Expires:     a.clock.Now().UTC().Add(ttl),
		BearerToken: bearer,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = a.UpsertAppSession(ctx, session); err != nil {
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
func (a *Server) generateAppToken(ctx context.Context, username string, roles []string, traits map[string][]string, uri string, expires time.Time) (string, error) {
	// Get the clusters CA.
	clusterName, err := a.GetDomainName()
	if err != nil {
		return "", trace.Wrap(err)
	}
	ca, err := a.GetCertAuthority(ctx, types.CertAuthID{
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
	signer, err := a.GetKeyStore().GetJWTSigner(ctx, ca)
	if err != nil {
		return "", trace.Wrap(err)
	}
	privateKey, err := services.GetJWTSigner(signer, ca.GetClusterName(), a.clock)
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

func (a *Server) CreateSessionCert(user services.UserState, sessionTTL time.Duration, publicKey []byte, compatibility, routeToCluster, kubernetesCluster, loginIP string, attestationReq *keys.AttestationStatement) ([]byte, []byte, error) {
	// It's safe to extract the access info directly from services.User because
	// this occurs during the initial login before the first certs have been
	// generated, so there's no possibility of any active access requests.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	userState, err := a.GetUserOrLoginState(ctx, user.GetName())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	accessInfo := services.AccessInfoFromUserState(userState)
	clusterName, err := a.GetClusterName()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	checker, err := services.NewAccessChecker(accessInfo, clusterName.GetClusterName(), a)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	certs, err := a.generateUserCert(ctx, certRequest{
		user:                 userState,
		ttl:                  sessionTTL,
		publicKey:            publicKey,
		compatibility:        compatibility,
		checker:              checker,
		traits:               userState.GetTraits(),
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

func (a *Server) CreateSnowflakeSession(ctx context.Context, req types.CreateSnowflakeSessionRequest,
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
	ttl := checker.AdjustSessionTTL(identity.Expires.Sub(a.clock.Now()))

	// Create services.WebSession for this session.
	sessionID, err := utils.CryptoRandomHex(defaults.SessionTokenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	session, err := types.NewWebSession(sessionID, types.KindSnowflakeSession, types.WebSessionSpecV2{
		User:               req.Username,
		Expires:            a.clock.Now().Add(ttl),
		BearerToken:        req.SessionToken,
		BearerTokenExpires: a.clock.Now().Add(req.TokenTTL),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = a.UpsertSnowflakeSession(ctx, session); err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf("Generated Snowflake web session for %v with TTL %v.", req.Username, ttl)

	return session, nil
}

func (a *Server) CreateSAMLIdPSession(ctx context.Context, req types.CreateSAMLIdPSessionRequest,
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
	if err = a.UpsertSAMLIdPSession(ctx, session); err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf("Generated SAML IdP web session for %v.", req.Username)

	return session, nil
}
