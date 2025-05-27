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
	"crypto"
	"crypto/rsa"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	dtauthz "github.com/gravitational/teleport/lib/devicetrust/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
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
	// LoginUserAgent is the user agent of the client's browser, as captured by
	// the Proxy.
	LoginUserAgent string
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
	// SSHPrivateKey is a specific private key to use when generating the web
	// sessions' SSH certificates.
	// This should be provided when extending an attested web session in order
	// to maintain the session attested status.
	SSHPrivateKey *keys.PrivateKey
	// TLSPrivateKey is a specific private key to use when generating the web
	// sessions' SSH certificates.
	// This should be provided when extending an attested web session in order
	// to maintain the session attested status.
	TLSPrivateKey *keys.PrivateKey
	// CreateDeviceWebToken informs Auth to issue a DeviceWebToken when creating
	// this session.
	// A DeviceWebToken must only be issued for users that have been authenticated
	// in the same RPC.
	// May only be set internally by Auth (and Auth-related logic), not allowed
	// for external requests.
	CreateDeviceWebToken bool
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
	session, _, err := a.newWebSession(ctx, req, nil /* opts */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = a.upsertWebSession(ctx, session)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Issue and assign the DeviceWebToken, but never persist it with the
	// session.
	if req.CreateDeviceWebToken {
		if err := a.augmentSessionForDeviceTrust(ctx, session, req.LoginIP, req.LoginUserAgent); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return session, nil
}

func (a *Server) augmentSessionForDeviceTrust(
	ctx context.Context,
	session types.WebSession,
	loginIP, userAgent string,
) error {
	// IP and user agent are mandatory for device web authentication.
	if loginIP == "" || userAgent == "" {
		return nil
	}

	// Create the device trust DeviceWebToken.
	// We only get a token if the server is enabled for Device Trust and the user
	// has a suitable trusted device.
	webToken, err := a.createDeviceWebToken(ctx, &devicepb.DeviceWebToken{
		WebSessionId:     session.GetName(),
		BrowserUserAgent: userAgent,
		BrowserIp:        loginIP,
		User:             session.GetUser(),
	})
	switch {
	case err != nil:
		a.logger.WarnContext(ctx, "Failed to create DeviceWebToken for user", "error", err)
	case webToken != nil: // May be nil even if err==nil.
		session.SetDeviceWebToken(&types.DeviceWebToken{
			Id:    webToken.Id,
			Token: webToken.Token,
		})
	}

	return nil
}

func (a *Server) calculateTrustedDeviceMode(
	ctx context.Context,
	getRoles func() ([]types.Role, error),
) (types.TrustedDeviceRequirement, error) {
	const unspecified = types.TrustedDeviceRequirement_TRUSTED_DEVICE_REQUIREMENT_UNSPECIFIED

	// Don't evaluate for OSS.
	if !modules.GetModules().IsEnterpriseBuild() {
		return unspecified, nil
	}

	ap, err := a.GetAuthPreference(ctx)
	if err != nil {
		return unspecified, trace.Wrap(err)
	}

	requirement, err := dtauthz.CalculateTrustedDeviceRequirement(ap.GetDeviceTrust(), getRoles)
	if err != nil {
		return unspecified, trace.Wrap(err)
	}
	return requirement, nil
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
) (types.WebSession, services.AccessChecker, error) {
	userState, err := a.GetUserOrLoginState(ctx, req.User)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if req.LoginIP == "" {
		// TODO(antonam): consider turning this into error after all use cases are covered (before v14.0 testplan)
		a.logger.DebugContext(ctx, "Creating new web session without login IP specified")
	}

	clusterName, err := a.GetClusterName(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	checker, err := services.NewAccessChecker(&services.AccessInfo{
		Roles:              req.Roles,
		Traits:             req.Traits,
		AllowedResourceIDs: req.RequestedResourceIDs,
	}, clusterName.GetClusterName(), a)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	idleTimeout, err := a.getWebIdleTimeout(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	var sshKey, tlsKey crypto.Signer
	if req.SSHPrivateKey != nil || req.TLSPrivateKey != nil {
		if req.SSHPrivateKey == nil || req.TLSPrivateKey == nil {
			return nil, nil, trace.BadParameter("invalid to set only one of SSHPrivateKey or TLSPrivateKey (this is a bug)")
		}
		sshKey, tlsKey = req.SSHPrivateKey.Signer, req.TLSPrivateKey.Signer
	} else {
		sshKey, tlsKey, err = cryptosuites.GenerateUserSSHAndTLSKey(ctx, cryptosuites.GetCurrentSuiteFromAuthPreference(a))
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if _, isRSA := sshKey.Public().(*rsa.PublicKey); isRSA {
			// Start precomputing RSA keys if we ever generate one.
			// [cryptosuites.PrecomputeRSAKeys] is idempotent.
			// Doing this lazily easily handles changing signature algorithm
			// suites and won't start precomputing keys if they are never needed
			// (a major benefit in tests).
			cryptosuites.PrecomputeRSAKeys()
		}
	}

	sessionTTL := req.SessionTTL
	if sessionTTL == 0 {
		sessionTTL = checker.AdjustSessionTTL(apidefaults.CertDuration)
	}

	if req.AttestWebSession {
		for _, pubKey := range []crypto.PublicKey{sshKey.Public(), tlsKey.Public()} {
			// Upsert web session attestation data so that this key's certs
			// will be marked with the web session private key policy.
			webAttData, err := services.NewWebSessionAttestationData(pubKey)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
			if err = a.UpsertKeyAttestationData(ctx, webAttData, sessionTTL); err != nil {
				return nil, nil, trace.Wrap(err)
			}
		}
	}

	sshPub, err := ssh.NewPublicKey(sshKey.Public())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	sshAuthorizedKey := ssh.MarshalAuthorizedKey(sshPub)

	tlsPublicKeyPEM, err := keys.MarshalPublicKey(tlsKey.Public())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	certReq := certRequest{
		user:           userState,
		loginIP:        req.LoginIP,
		ttl:            sessionTTL,
		sshPublicKey:   sshAuthorizedKey,
		tlsPublicKey:   tlsPublicKeyPEM,
		checker:        checker,
		traits:         req.Traits,
		activeRequests: req.AccessRequests,
	}
	var hasDeviceExtensions bool
	if opts != nil && opts.deviceExtensions != nil {
		// Apply extensions to request.
		certReq.deviceExtensions = DeviceExtensions(*opts.deviceExtensions)
		hasDeviceExtensions = true
	}

	certs, err := a.generateUserCert(ctx, certReq)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	token, err := utils.CryptoRandomHex(defaults.SessionTokenBytes)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	bearerToken, err := utils.CryptoRandomHex(defaults.SessionTokenBytes)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	bearerTokenTTL := min(sessionTTL, defaults.BearerTokenTTL)

	startTime := a.clock.Now()
	if !req.LoginTime.IsZero() {
		startTime = req.LoginTime
	}

	sshPrivateKeyPEM, err := keys.MarshalPrivateKey(sshKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	tlsPrivateKeyPEM, err := keys.MarshalPrivateKey(tlsKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	sessionSpec := types.WebSessionSpecV2{
		User:                req.User,
		Priv:                sshPrivateKeyPEM,
		TLSPriv:             tlsPrivateKeyPEM,
		Pub:                 certs.SSH,
		TLSCert:             certs.TLS,
		Expires:             startTime.UTC().Add(sessionTTL),
		BearerToken:         bearerToken,
		BearerTokenExpires:  startTime.UTC().Add(bearerTokenTTL),
		LoginTime:           req.LoginTime,
		IdleTimeout:         types.Duration(idleTimeout),
		HasDeviceExtensions: hasDeviceExtensions,
	}
	UserLoginCount.Inc()

	sess, err := types.NewWebSession(token, types.KindWebSession, sessionSpec)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if tdr, err := a.calculateTrustedDeviceMode(ctx, func() ([]types.Role, error) {
		return checker.Roles(), nil
	}); err != nil {
		a.logger.WarnContext(ctx, "Failed to calculate trusted device mode for session", "error", err)
	} else {
		sess.SetTrustedDeviceRequirement(tdr)

		if tdr != types.TrustedDeviceRequirement_TRUSTED_DEVICE_REQUIREMENT_UNSPECIFIED {
			a.logger.DebugContext(ctx, "Calculated trusted device requirement for session",
				"user", req.User,
				"trusted_device_requirement", tdr,
			)
		}
	}

	return sess, checker, nil
}

func (a *Server) getWebIdleTimeout(ctx context.Context) (time.Duration, error) {
	netCfg, err := a.GetReadOnlyClusterNetworkingConfig(ctx)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return netCfg.GetWebIdleTimeout(), nil
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

// NewAppSessionRequest defines a request to create a new user app session.
type NewAppSessionRequest struct {
	NewWebSessionRequest

	// PublicAddr is the public address the application.
	PublicAddr string
	// ClusterName is cluster within which the application is running.
	ClusterName string
	// AWSRoleARN is AWS role the user wants to assume.
	AWSRoleARN string
	// AzureIdentity is Azure identity the user wants to assume.
	AzureIdentity string
	// GCPServiceAccount is the GCP service account the user wants to assume.
	GCPServiceAccount string
	// MFAVerified is the UUID of an MFA device used to verify this request.
	MFAVerified string
	// DeviceExtensions holds device-aware user certificate extensions.
	DeviceExtensions DeviceExtensions
	// AppName is the name of the app.
	AppName string
	// AppURI is the URI of the app. This is the internal endpoint where the application is running and isn't user-facing.
	AppURI string
	// AppTargetPort signifies that the session is made to a specific port of a multi-port TCP app.
	AppTargetPort int
	// Identity is the identity of the user.
	Identity tlsca.Identity
	// ClientAddr is a client (user's) address.
	ClientAddr string

	// BotName is the name of the bot that is creating this session.
	// Empty if not a bot.
	BotName string
	// BotInstanceID is the ID of the bot instance that is creating this session.
	// Empty if not a bot.
	BotInstanceID string
}

// CreateAppSession creates and inserts a services.WebSession into the
// backend with the identity of the caller used to generate the certificate.
// The certificate is used for all access requests, which is where access
// control is enforced.
func (a *Server) CreateAppSession(ctx context.Context, req *proto.CreateAppSessionRequest, identity tlsca.Identity, checker services.AccessChecker) (types.WebSession, error) {
	if !modules.GetModules().Features().GetEntitlement(entitlements.App).Enabled {
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
	roles, traits, err := services.ExtractFromIdentity(ctx, a, identity)
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

	sess, err := a.CreateAppSessionFromReq(ctx, NewAppSessionRequest{
		NewWebSessionRequest: NewWebSessionRequest{
			User:           req.Username,
			LoginIP:        identity.LoginIP,
			SessionTTL:     ttl,
			Roles:          roles,
			Traits:         traits,
			AccessRequests: identity.ActiveRequests,
			// If the user's current identity is attested as a "web_session", its secrets are only
			// available to the Proxy and Auth roles, meaning this request is coming from the Proxy
			// service on behalf of the user's Web Session. We can safely attest this child app session
			// as a "web_session" as a result.
			AttestWebSession: identity.PrivateKeyPolicy == keys.PrivateKeyPolicyWebSession,
		},
		PublicAddr:        req.PublicAddr,
		ClusterName:       req.ClusterName,
		AWSRoleARN:        req.AWSRoleARN,
		AzureIdentity:     req.AzureIdentity,
		GCPServiceAccount: req.GCPServiceAccount,
		MFAVerified:       verifiedMFADeviceID,
		AppName:           req.AppName,
		AppURI:            req.URI,
		DeviceExtensions:  DeviceExtensions(identity.DeviceExtensions),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sess, nil
}

func (a *Server) CreateAppSessionFromReq(ctx context.Context, req NewAppSessionRequest) (types.WebSession, error) {
	if !modules.GetModules().Features().GetEntitlement(entitlements.App).Enabled {
		return nil, trace.AccessDenied(
			"this Teleport cluster is not licensed for application access, please contact the cluster administrator")
	}

	if req.CreateDeviceWebToken {
		return nil, trace.BadParameter("parameter CreateDeviceWebToken disallowed for App Sessions")
	}

	user, err := a.GetUserOrLoginState(ctx, req.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterName, err := a.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	checker, err := services.NewAccessChecker(&services.AccessInfo{
		Username:           req.User,
		Roles:              req.Roles,
		Traits:             req.Traits,
		AllowedResourceIDs: req.RequestedResourceIDs,
	}, clusterName.GetClusterName(), a)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create services.WebSession for this session.
	sessionID, err := utils.CryptoRandomHex(defaults.SessionTokenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create certificate for this session.
	priv, err := cryptosuites.GenerateKey(ctx, cryptosuites.GetCurrentSuiteFromAuthPreference(a), cryptosuites.UserTLS)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.AttestWebSession {
		// Upsert web session attestation data so that this key's certs
		// will be marked with the web session private key policy.
		webAttData, err := services.NewWebSessionAttestationData(priv.Public())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err = a.UpsertKeyAttestationData(ctx, webAttData, req.SessionTTL); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	privateKeyPEM, err := keys.MarshalPrivateKey(priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsPublicKey, err := keys.MarshalPublicKey(priv.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certs, err := a.generateUserCert(ctx, certRequest{
		user:           user,
		loginIP:        req.LoginIP,
		tlsPublicKey:   tlsPublicKey,
		checker:        checker,
		ttl:            req.SessionTTL,
		traits:         req.Traits,
		activeRequests: req.AccessRequests,
		// Set the app session ID in the certificate - used in auditing from the App Service.
		appSessionID: sessionID,
		// Only allow this certificate to be used for applications.
		usage:             []string{teleport.UsageAppsOnly},
		appPublicAddr:     req.PublicAddr,
		appClusterName:    req.ClusterName,
		appTargetPort:     req.AppTargetPort,
		awsRoleARN:        req.AWSRoleARN,
		azureIdentity:     req.AzureIdentity,
		gcpServiceAccount: req.GCPServiceAccount,
		// Pass along device extensions from the user.
		deviceExtensions: req.DeviceExtensions,
		mfaVerified:      req.MFAVerified,
		// Pass along bot details to ensure audit logs are correct.
		botName:       req.BotName,
		botInstanceID: req.BotInstanceID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	bearer, err := utils.CryptoRandomHex(defaults.SessionTokenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	session, err := types.NewWebSession(sessionID, types.KindAppSession, types.WebSessionSpecV2{
		User:        req.User,
		TLSPriv:     privateKeyPEM,
		TLSCert:     certs.TLS,
		LoginTime:   a.clock.Now().UTC(),
		Expires:     a.clock.Now().UTC().Add(req.SessionTTL),
		BearerToken: bearer,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = a.UpsertAppSession(ctx, session); err != nil {
		return nil, trace.Wrap(err)
	}
	a.logger.DebugContext(ctx, "Generated application web session", "user", req.User, "ttl", req.SessionTTL)
	UserLoginCount.Inc()

	// Extract the identity of the user from the certificate, this will include metadata from any actively assumed access requests.
	certificate, err := tlsca.ParseCertificatePEM(session.GetTLSCert())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity, err := tlsca.FromSubject(certificate.Subject, certificate.NotAfter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userMetadata := identity.GetUserMetadata()
	userMetadata.User = session.GetUser()
	userMetadata.AWSRoleARN = req.AWSRoleARN

	err = a.emitter.EmitAuditEvent(a.closeCtx, &apievents.AppSessionStart{
		Metadata: apievents.Metadata{
			Type:        events.AppSessionStartEvent,
			Code:        events.AppSessionStartCode,
			ClusterName: req.ClusterName,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerVersion:   teleport.Version,
			ServerID:        a.ServerID,
			ServerNamespace: apidefaults.Namespace,
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID:        session.GetName(),
			WithMFA:          req.MFAVerified,
			PrivateKeyPolicy: string(req.Identity.PrivateKeyPolicy),
		},
		UserMetadata: userMetadata,
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: req.ClientAddr,
		},
		PublicAddr: req.PublicAddr,
		AppMetadata: apievents.AppMetadata{
			AppURI:        req.AppURI,
			AppPublicAddr: req.PublicAddr,
			AppName:       req.AppName,
			AppTargetPort: uint32(req.AppTargetPort),
		},
	})
	if err != nil {
		a.logger.WarnContext(ctx, "Failed to emit app session start event", "error", err)
	}

	return session, nil
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

// SessionCertsRequest is a request for new user session certs.
type SessionCertsRequest struct {
	UserState               services.UserState
	SessionTTL              time.Duration
	SSHPubKey               []byte
	TLSPubKey               []byte
	SSHAttestationStatement *hardwarekey.AttestationStatement
	TLSAttestationStatement *hardwarekey.AttestationStatement
	Compatibility           string
	RouteToCluster          string
	KubernetesCluster       string
	LoginIP                 string
}

// CreateSessionCerts returns new user certs. The user must already be
// authenticated.
func (a *Server) CreateSessionCerts(ctx context.Context, req *SessionCertsRequest) ([]byte, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// It's safe to extract the access info directly from services.User because
	// this occurs during the initial login before the first certs have been
	// generated, so there's no possibility of any active access requests.
	accessInfo := services.AccessInfoFromUserState(req.UserState)
	clusterName, err := a.GetClusterName(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	checker, err := services.NewAccessChecker(accessInfo, clusterName.GetClusterName(), a)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	certs, err := a.generateUserCert(ctx, certRequest{
		user:                             req.UserState,
		ttl:                              req.SessionTTL,
		sshPublicKey:                     req.SSHPubKey,
		tlsPublicKey:                     req.TLSPubKey,
		sshPublicKeyAttestationStatement: req.SSHAttestationStatement,
		tlsPublicKeyAttestationStatement: req.TLSAttestationStatement,
		compatibility:                    req.Compatibility,
		checker:                          checker,
		traits:                           req.UserState.GetTraits(),
		routeToCluster:                   req.RouteToCluster,
		kubernetesCluster:                req.KubernetesCluster,
		loginIP:                          req.LoginIP,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return certs.SSH, certs.TLS, nil
}

func (a *Server) CreateSnowflakeSession(ctx context.Context, req types.CreateSnowflakeSessionRequest,
	identity tlsca.Identity, checker services.AccessChecker,
) (types.WebSession, error) {
	if !modules.GetModules().Features().GetEntitlement(entitlements.DB).Enabled {
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
	a.logger.DebugContext(ctx, "Generated Snowflake web session", "user", req.Username, "ttl", ttl)

	return session, nil
}
