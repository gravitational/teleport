package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	jose "github.com/go-jose/go-jose/v3"
	josejwt "github.com/go-jose/go-jose/v3/jwt"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/zitadel/oidc/v3/pkg/client"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"golang.org/x/oauth2"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	typescommon "github.com/gravitational/teleport/api/types/common"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/e/lib/auth/entraid"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client/sso"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/loginrule"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// authGracePeriod accounts for the time it takes for an OIDC provider to
// make a request to the Teleport callback URL after authenticating the
// user and setting the 'auth_time' claim.
const authGracePeriod = time.Minute

type OIDCAuthService struct {
	auth           *auth.Server
	emitter        apievents.Emitter
	client         *http.Client
	getSigner      JWTSignerFactory
	licenseChecker LicenseChecker

	mu  sync.Mutex
	rps map[rpKey]relyingParty
}

type OIDCAuthServiceConfig struct {
	Auth           *auth.Server
	Emitter        apievents.Emitter
	Client         *http.Client
	SignerFactory  JWTSignerFactory
	LicenseChecker LicenseChecker
}

func (cfg *OIDCAuthServiceConfig) CheckAndSetDefaults() error {
	if cfg.Auth == nil {
		return trace.BadParameter("auth.Server not provided")
	}
	if cfg.Emitter == nil {
		cfg.Emitter = events.NewDiscardEmitter()
	}
	if cfg.Client == nil {
		clt, err := defaults.HTTPClient()
		if err != nil {
			return trace.Wrap(err)
		}

		cfg.Client = clt
	}
	if cfg.SignerFactory == nil {
		cfg.SignerFactory = DefaultJWTSignerFactory(cfg.Auth)
	}
	if cfg.LicenseChecker == nil {
		return trace.BadParameter("license checker not provided")
	}
	return nil
}

// oidcRoundTripper wrapps the [http.RoundTripper] provided to the
// [rp.RelyingParty] to prevent reading response bodies that exceed
// [maxDataSize].
//
// TODO(tross): remove this when https://github.com/zitadel/oidc/issues/738
// is resolved.
type oidcRoundTripper struct {
	rt http.RoundTripper
}

func (l *oidcRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := l.rt.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	return &http.Response{
		Status:           resp.Status,
		StatusCode:       resp.StatusCode,
		Proto:            resp.Proto,
		ProtoMajor:       resp.ProtoMajor,
		ProtoMinor:       resp.ProtoMinor,
		Header:           resp.Header,
		Body:             newLimitReadCloser(resp.Body, resp.Body),
		ContentLength:    resp.ContentLength,
		TransferEncoding: resp.TransferEncoding,
		Close:            resp.Close,
		Uncompressed:     resp.Uncompressed,
		Trailer:          resp.Trailer,
		Request:          resp.Request,
		TLS:              resp.TLS,
	}, nil
}

func (l *oidcRoundTripper) CloseIdleConnections() {
	type closeIdler interface {
		CloseIdleConnections()
	}
	if tr, ok := l.rt.(closeIdler); ok {
		tr.CloseIdleConnections()
	}
}

const maxDataSize = 1024 * 1024

// limitReadCloser is an [io.ReadCloser] that limits reading
// less than [maxDataSize] from the reader.
type limitReadCloser struct {
	n      int64
	reader io.Reader
	closer io.Closer
}

func newLimitReadCloser(reader io.Reader, closer io.Closer) *limitReadCloser {
	return &limitReadCloser{
		n:      maxDataSize + 1,
		reader: reader,
		closer: closer,
	}
}

func (r *limitReadCloser) Read(p []byte) (int, error) {
	if r.n <= 0 {
		// discard rest of the body to free up connection
		io.Copy(io.Discard, r.reader)
		return 0, trace.Errorf("response exceeds maximum size of %d bytes", maxDataSize)
	}

	if int64(len(p)) > r.n {
		p = p[0:r.n]
	}
	n, err := r.reader.Read(p)
	r.n -= int64(n)
	return n, err
}

func (r *limitReadCloser) Close() error {
	return r.closer.Close()
}

func NewOIDCAuthService(cfg *OIDCAuthServiceConfig) (*OIDCAuthService, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, err
	}

	return &OIDCAuthService{
		auth:    cfg.Auth,
		emitter: cfg.Emitter,
		client: &http.Client{
			Transport:     &oidcRoundTripper{rt: cfg.Client.Transport},
			CheckRedirect: cfg.Client.CheckRedirect,
			Jar:           cfg.Client.Jar,
			Timeout:       cfg.Client.Timeout,
		},
		rps:            map[rpKey]relyingParty{},
		getSigner:      cfg.SignerFactory,
		licenseChecker: cfg.LicenseChecker,
	}, nil
}

// relyingParty associates an [rp.RelyingParty] with
// a [types.OIDCConnector].
type relyingParty struct {
	rp.RelyingParty
	discoveryConfig oidc.DiscoveryConfiguration
	conn            types.OIDCConnector
}

// rpKey is an internal key for a relying party.
type rpKey struct {
	name  string
	proxy string
	// The settings for an MFA connector differ from the
	// base connector, so we need to differentiate between them.
	forMFA bool
}

// ErrOIDCNoRoles results from not mapping any roles from OIDC claims.
var ErrOIDCNoRoles = &trace.AccessDeniedError{Message: "No roles mapped from claims. The mappings may contain typos."}

// CreateOIDCAuthRequest creates an OIDC AuthnRequest.
func (oas *OIDCAuthService) CreateOIDCAuthRequest(ctx context.Context, req types.OIDCAuthRequest) (*types.OIDCAuthRequest, error) {
	return oas.createOIDCAuthRequest(ctx, req, false /*forMFA*/)
}

// CreateOIDCAuthRequestForMFA creates an OIDC AuthnRequest for MFA.
func (oas *OIDCAuthService) CreateOIDCAuthRequestForMFA(ctx context.Context, req types.OIDCAuthRequest) (*types.OIDCAuthRequest, error) {
	return oas.createOIDCAuthRequest(ctx, req, true /*forMFA*/)
}

func (oas *OIDCAuthService) getRelyingParty(ctx context.Context, connector types.OIDCConnector, proxyAddress string, forMFA bool) (relyingParty, error) {
	oas.mu.Lock()
	defer oas.mu.Unlock()

	key := rpKey{connector.GetName(), proxyAddress, forMFA}
	cachedRP, ok := oas.rps[key]
	if ok &&
		slices.Equal(connector.GetRedirectURLs(), cachedRP.conn.GetRedirectURLs()) &&
		connector.GetClientID() == cachedRP.conn.GetClientID() &&
		connector.GetClientSecret() == cachedRP.conn.GetClientSecret() &&
		slices.Equal(connector.GetScope(), cachedRP.conn.GetScope()) &&
		connector.GetIssuerURL() == cachedRP.conn.GetIssuerURL() &&
		connector.GetAllowUnverifiedEmail() == cachedRP.conn.GetAllowUnverifiedEmail() &&
		connector.GetUsernameClaim() == cachedRP.conn.GetUsernameClaim() {
		return cachedRP, nil
	}

	delete(oas.rps, key)

	// The discover url MUST retain any query parameters of the issuer url
	// for some providers to function properly, e.g. IBM to function,
	// so https://example.com/id?a=b has to become
	// https://example.com/id/.well-known/openid-configuration?a=b.
	u, err := url.Parse(connector.GetIssuerURL())
	if err != nil {
		return relyingParty{}, trace.Wrap(err)
	}
	u.Fragment = ""
	u.Path = strings.TrimSuffix(u.Path, "/") + oidc.DiscoveryEndpoint

	redirectURI, err := services.GetRedirectURL(connector, proxyAddress)
	if err != nil {
		return relyingParty{}, trace.Wrap(err)
	}

	// NewRelyingPartyOIDC calls the discovery endpoint, but appears to discard much of the response
	// It only hangs on to configured endpoints and supported ID Token signing algs
	// Let's call it here and hang on to the full response
	// TODO(rhammonds): We could go ahead and validate the provider's discovery advertisement against our own
	// connector configuration to catch any incompatibilites
	discoveryConfig, err := client.Discover(ctx, connector.GetIssuerURL(), oas.client, u.String())
	if err != nil {
		return relyingParty{}, trace.Wrap(err, "Error invoking discovery URL %q", u.String())
	}

	rpOIDC, err := rp.NewRelyingPartyOIDC(
		ctx,
		connector.GetIssuerURL(),
		connector.GetClientID(),
		connector.GetClientSecret(),
		redirectURI,
		apiutils.Deduplicate(append([]string{"openid", "email"}, connector.GetScope()...)),
		rp.WithCustomDiscoveryUrl(u.String()),
		rp.WithLogger(slog.With(teleport.ComponentKey, "OIDC")),
		rp.WithHTTPClient(oas.client),
	)
	if err != nil {
		return relyingParty{}, trace.Wrap(err)
	}

	newRp := relyingParty{RelyingParty: rpOIDC, conn: connector, discoveryConfig: *discoveryConfig}
	oas.rps[key] = newRp
	return newRp, nil
}

// JWTSignerFactory provides a JWT key to be used by OIDCAuthService to sign JWTs.
type JWTSignerFactory func(ctx context.Context) (jose.Signer, string, error)

// DefaultJWTSignerFactory provides a default implementation for retrieving a
// JWT signer from the 'oidc_idp' certificate authority.
func DefaultJWTSignerFactory(auth *auth.Server) JWTSignerFactory {
	return func(ctx context.Context) (jose.Signer, string, error) {
		return getJWTSignerFromCertAuthority(ctx, auth)
	}
}

func getJWTSignerFromCertAuthority(ctx context.Context, auth *auth.Server) (jose.Signer, string, error) {
	clusterName, err := auth.GetClusterName(ctx)
	if err != nil {
		return nil, "", trace.Wrap(err, "Failed to obtain cluster name")
	}

	ca, err := auth.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.OIDCIdPCA,
		DomainName: clusterName.GetClusterName(),
	}, true /*loadKeys*/)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	signer, err := auth.GetKeyStore().GetJWTSigner(ctx, ca)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	alg, err := jwt.AlgorithmForPublicKey(signer.Public())
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	key, err := jwt.SigningKeyFromPrivateKey(signer)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	kid, err := jwt.KeyID(signer.Public())
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	joseSigner, err := jose.NewSigner(key, (&jose.SignerOptions{}).WithHeader("kid", kid))
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return joseSigner, string(alg), nil
}

func (oas *OIDCAuthService) createOIDCAuthRequest(ctx context.Context, req types.OIDCAuthRequest, forMFA bool) (*types.OIDCAuthRequest, error) {
	if oas.licenseChecker.IsDisabled() {
		return nil, ErrLicenseExpired
	}

	var connector types.OIDCConnector
	if req.SSOTestFlow {
		if req.ConnectorSpec == nil {
			return nil, trace.BadParameter("ConnectorSpec cannot be nil when SSOTestFlow is true")
		}

		if req.ConnectorID == "" {
			return nil, trace.BadParameter("ConnectorID cannot be empty")
		}

		var err error
		connector, err = types.NewOIDCConnector(req.ConnectorID, *req.ConnectorSpec)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		var err error
		connector, err = oas.auth.GetOIDCConnector(ctx, req.ConnectorID, true)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if forMFA {
		if err := connector.WithMFASettings(); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// see [auth.CreateGithubAuthRequest]
	if !req.CreateWebSession {
		ceremonyType := sso.CeremonyTypeLogin
		if req.SSOTestFlow {
			ceremonyType = sso.CeremonyTypeTest
		} else if forMFA {
			ceremonyType = sso.CeremonyTypeMFA
		}

		if err := sso.ValidateClientRedirect(req.ClientRedirectURL, ceremonyType, connector.GetClientRedirectSettings()); err != nil {
			return nil, trace.Wrap(err, auth.InvalidClientRedirectErrorMessage)
		}
	}

	relyingParty, err := oas.getRelyingParty(ctx, connector, req.ProxyAddress, forMFA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	stateToken, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authCodeOpts := []oauth2.AuthCodeOption{
		oauth2.AccessTypeOnline,
		oauth2.SetAuthURLParam("prompt", connector.GetPrompt()),
	}
	// do not add code_challenge params if PKCE is disabled for this connector
	if connector.IsPKCEEnabled() {
		logger.DebugContext(ctx, "PKCE enabled, appending code challenge")

		if req.PkceVerifier == "" {
			return nil, trace.BadParameter("pkce code verifier must not be empty")
		}
		codeChallenge := oauth2.S256ChallengeFromVerifier(req.PkceVerifier)
		authCodeOpts = append(authCodeOpts,
			oauth2.SetAuthURLParam("code_challenge", codeChallenge),
			oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		)
	}
	req.StateToken = stateToken

	if req.LoginHint != "" {
		authCodeOpts = append(authCodeOpts, oauth2.SetAuthURLParam("login_hint", req.LoginHint))
	}

	acURL := rp.AuthURL(
		req.StateToken,
		relyingParty,
		func() []oauth2.AuthCodeOption {
			return authCodeOpts
		},
	)
	redirectURL, err := url.Parse(acURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	redirectQuery := redirectURL.Query()

	// if the connector has an Authentication Context Class Reference (ACR) value set,
	// update redirect url and add it as a query value.
	acrValue := connector.GetACR()
	if acrValue != "" {
		redirectQuery.Set("acr_values", acrValue)
	}

	// set max_age to tell the provider the user must be reauthenticated
	// after a certain amount of time if necessary.
	// https://openid.net/specs/openid-connect-core-1_0.html#AuthRequest
	if maxAge, ok := connector.GetMaxAge(); ok {
		maxAgeSeconds := int64(maxAge / time.Second)
		redirectQuery.Set("max_age", strconv.FormatInt(maxAgeSeconds, 10))
	}

	// Only do JWT-Secured Authorization Requests (JAR) when configured for the connector.
	if connector.GetRequestObjectMode() == constants.OIDCRequestObjectModeSigned {
		// RFC 9101 - https://www.rfc-editor.org/rfc/rfc9101.html#name-request-object-2
		// "It [request object] MUST contain all the parameters (including extension parameters) used to
		// process the OAuth 2.0 [RFC6749] authorization request except the request and
		// request_uri parameters that are defined in this document."
		//
		// As the RFC states, our authorization request must contain only 'client_id' AND 'request' XOR 'request_uri' parameters.
		// (client_id goes into both the URL *and* JWT)
		signer, alg, err := oas.getSigner(ctx)
		if err != nil {
			return nil, err
		}

		if !relyingParty.discoveryConfig.RequestParameterSupported {
			return nil, trace.Errorf("connector is configured for request_object_mode '%s' but IdP does not support request objects", connector.GetRequestObjectMode())
		}

		if !slices.Contains(relyingParty.discoveryConfig.RequestObjectSigningAlgValuesSupported, alg) {
			return nil, trace.Errorf("Teleport signs request objects using signature algorithm '%s' which is not supported by the IdP", alg)
		}

		requestParameters := map[string]any{}
		for key, val := range redirectQuery {
			if len(val) == 1 {
				requestParameters[key] = val[0]
			} else {
				requestParameters[key] = val
			}
			// Clear each query param as we set them as claims
			// in the request object
			delete(redirectQuery, key)
		}

		// Intentionally omit standard JWT claims. They're not explicitly
		// required per RFC 9101, and it has been observed that the inclusion of
		// certain claims, like "issuer", will cause some IdPs to reject our request objects.
		signedRequestToken, err := josejwt.Signed(signer).Claims(requestParameters).CompactSerialize()
		if err != nil {
			return nil, trace.Wrap(err, "Failed to create request object JWT")
		}
		// and add back only the client_id and request_object
		// parameters per the RFC
		redirectQuery.Add("request", signedRequestToken)
		redirectQuery.Add("client_id", connector.GetClientID())
	}

	redirectURL.RawQuery = redirectQuery.Encode()
	req.RedirectURL = redirectURL.String()
	if req.RedirectURL == "" {
		return nil, trace.BadParameter("response redirect URL is empty")
	}

	if err := oas.auth.Services.CreateOIDCAuthRequest(ctx, req, defaults.OIDCAuthRequestTTL); err != nil {
		return nil, trace.Wrap(err)
	}
	return &req, nil
}

// ValidateOIDCAuthCallback is called by the proxy to check OIDC query parameters
// returned by OIDC Provider, if everything checks out, auth server
// will respond with OIDCAuthResponse, otherwise it will return error
func (oas *OIDCAuthService) ValidateOIDCAuthCallback(ctx context.Context, q url.Values) (*authclient.OIDCAuthResponse, error) {
	if oas.licenseChecker.IsDisabled() {
		return nil, ErrLicenseExpired
	}

	event := &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
		},
		Method: events.LoginMethodOIDC,
	}

	diagCtx := auth.NewSSODiagContext(types.KindOIDC, oas.auth)

	resp, loginIP, err := oas.validateOIDCAuthCallback(ctx, diagCtx, q)
	diagCtx.Info.Error = trace.UserMessage(err)
	diagCtx.WriteToBackend(ctx)
	event.AppliedLoginRules = diagCtx.Info.AppliedLoginRules
	if claims := diagCtx.Info.OIDCClaims; claims != nil {
		if attributes, err := apievents.EncodeMap(claims); err != nil {
			event.Status.UserMessage = fmt.Sprintf("Failed to encode identity attributes: %v", err.Error())
			logger.DebugContext(ctx, "Failed to encode identity attributes", "error", err)
		} else {
			event.IdentityAttributes = attributes
		}
	}
	if err != nil {
		event.Code = events.UserSSOLoginFailureCode
		if diagCtx.Info.TestFlow {
			event.Code = events.UserSSOTestFlowLoginFailureCode
		}
		event.Status.Success = false
		event.Status.Error = trace.Unwrap(err).Error()
		event.Status.UserMessage = err.Error()

		if err := oas.emitter.EmitAuditEvent(ctx, event); err != nil {
			logger.WarnContext(ctx, "Failed to emit OIDC login failed event", "error", err)
		}

		return nil, trace.Wrap(err)
	}

	event.ConnectionMetadata = apievents.ConnectionMetadata{RemoteAddr: loginIP}
	event.Code = events.UserSSOLoginCode
	if diagCtx.Info.TestFlow {
		event.Code = events.UserSSOTestFlowLoginCode
	}
	event.User = resp.Username
	event.Status.Success = true

	if err := oas.emitter.EmitAuditEvent(ctx, event); err != nil {
		logger.WarnContext(ctx, "Failed to emit OIDC login event", "error", err)
	}

	return resp, nil
}

func validateOIDCAuthCallbackWeb(authClient *auth.ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (any, error) {
	var req *authclient.ValidateOIDCAuthCallbackReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := authClient.ValidateOIDCAuthCallback(r.Context(), req.Query)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	raw := authclient.OIDCAuthRawResponse{
		Username:      response.Username,
		Identity:      response.Identity,
		Cert:          response.Cert,
		TLSCert:       response.TLSCert,
		Req:           response.Req,
		MFAToken:      response.MFAToken,
		ClientOptions: response.ClientOptions,
	}
	if response.Session != nil {
		rawSession, err := services.MarshalWebSession(response.Session, services.WithVersion(version))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		raw.Session = rawSession
	}
	raw.HostSigners = make([]json.RawMessage, len(response.HostSigners))
	for i, ca := range response.HostSigners {
		data, err := services.MarshalCertAuthority(ca, services.WithVersion(version))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		raw.HostSigners[i] = data
	}
	return &raw, nil
}

func (oas *OIDCAuthService) validateOIDCAuthCallback(ctx context.Context, diagCtx *auth.SSODiagContext, q url.Values) (*authclient.OIDCAuthResponse, string, error) {
	if errParam := q.Get("error"); errParam != "" {
		// try to find request so the error gets logged against it.
		state := q.Get("state")
		if state != "" {
			diagCtx.RequestID = state
			req, err := oas.auth.GetOIDCAuthRequest(ctx, state)
			if err == nil {
				diagCtx.Info.TestFlow = req.SSOTestFlow
			}
		}

		// optional parameter: error_description
		errDesc := q.Get("error_description")
		oidcErr := trace.OAuth2(string(oidc.InvalidRequest), errParam, q)
		return nil, "", trace.WithUserMessage(oidcErr, "OIDC provider returned error: %v [%v]", errDesc, errParam)
	}

	code := q.Get("code")
	if code == "" {
		oidcErr := trace.OAuth2(string(oidc.InvalidRequest), "code query param must be set", q)
		return nil, "", trace.WithUserMessage(oidcErr, "Invalid parameters received from OIDC provider.")
	}

	stateToken := q.Get("state")
	if stateToken == "" {
		oidcErr := trace.OAuth2(string(oidc.InvalidRequest), "missing state query param", q)
		return nil, "", trace.WithUserMessage(oidcErr, "Invalid parameters received from OIDC provider.")
	}
	diagCtx.RequestID = stateToken

	req, err := oas.auth.GetOIDCAuthRequest(ctx, stateToken)
	if err != nil {
		return nil, "", trace.Wrap(err, "Failed to get OIDC Auth Request.")
	}
	diagCtx.Info.TestFlow = req.SSOTestFlow

	// Check if this is an MFA request.
	mfaSession, err := oas.auth.GetSSOMFASession(ctx, stateToken)
	if err != nil && !trace.IsNotFound(err) {
		return nil, "", trace.Wrap(err)
	}

	var connector types.OIDCConnector
	if req.SSOTestFlow {
		if req.ConnectorSpec == nil {
			return nil, "", trace.BadParameter("ConnectorSpec cannot be nil when SSOTestFlow is true")
		}

		if req.ConnectorID == "" {
			return nil, "", trace.BadParameter("ConnectorID cannot be empty")
		}

		connector, err = types.NewOIDCConnector(req.ConnectorID, *req.ConnectorSpec)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
	} else {
		connector, err = oas.auth.GetOIDCConnector(ctx, req.ConnectorID, true)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
	}

	if mfaSession != nil {
		if err := connector.WithMFASettings(); err != nil {
			return nil, "", trace.Wrap(err)
		}
	}

	idToken, err := oas.retrieveIDTokenClaims(ctx, connector, code, req, q.Get("code_verifier"), mfaSession != nil)
	if err != nil {
		return nil, req.ClientLoginIP, trace.Wrap(err)
	}

	diagCtx.Info.OIDCClaims = idToken.IDTokenClaims.Claims

	logger.DebugContext(ctx, "Retrieved OIDC claims", "claims", idToken.IDTokenClaims.Claims)

	// check auth_time claim if max_age was passed in the request
	if maxAge, ok := connector.GetMaxAge(); ok {
		authTime := idToken.IDTokenClaims.GetAuthTime()
		if authTime.IsZero() {
			oidcErr := trace.OAuth2(string(oidc.InvalidRequest), "missing claim auth_time", q)
			return nil, req.ClientLoginIP, trace.Wrap(oidcErr, "Invalid parameters received from OIDC provider.")
		}

		maxAge = max(maxAge, authGracePeriod)
		if time.Since(authTime) > maxAge {
			oidcErr := trace.OAuth2(string(oidc.InvalidRequest), "user needs to reauthenticate", q)
			return nil, req.ClientLoginIP, trace.Wrap(oidcErr, "Reauthentication is required.")
		}
	}

	// Email verification, if required by the connector, is not enforced if the email_verified
	// claim is not present. If it is present, then it must have a truthy value otherwise the
	// authentication attempt is rejected.
	verifiedClaim, verifiedClaimProvided := idToken.IDTokenClaims.Claims["email_verified"]
	if !connector.GetAllowUnverifiedEmail() && verifiedClaimProvided && verifiedClaim != "" && !bool(idToken.IDTokenClaims.EmailVerified) {
		return nil, req.ClientLoginIP, trace.AccessDenied("email not verified by OIDC provider")
	}

	// if we are sending acr values, make sure we also validate them
	if acrValue := connector.GetACR(); acrValue != "" {
		if err := validateACRValues(ctx, acrValue, connector.GetProvider(), idToken); err != nil {
			return nil, req.ClientLoginIP, trace.Wrap(err, "OIDC ACR validation failure.")
		}
		logger.DebugContext(ctx, "OIDC ACR values successfully validated.", "acr_value", acrValue)
	}

	diagCtx.Info.OIDCIdentity = &types.OIDCIdentity{
		ID:        idToken.IDTokenClaims.Subject,
		Name:      idToken.IDTokenClaims.Name,
		Email:     idToken.IDTokenClaims.Email,
		ExpiresAt: idToken.IDTokenClaims.GetExpiration(),
	}
	logger.DebugContext(ctx, "Using retrieved OIDC user expiry",
		"user", idToken.IDTokenClaims.Email,
		"expiry", idToken.IDTokenClaims.GetExpiration(),
	)

	if len(connector.GetClaimsToRoles()) == 0 {
		oidcErr := trace.BadParameter("no claims to roles mapping, check connector documentation")
		return nil, req.ClientLoginIP, trace.WithUserMessage(oidcErr, "Claims-to-roles mapping is empty, SSO user will never have any roles.")
	}
	logger.DebugContext(ctx, "Applying OIDC claims to roles mappings.", "claims_to_roles_count", len(connector.GetClaimsToRoles()))
	diagCtx.Info.OIDCClaimsToRoles = connector.GetClaimsToRoles()

	// Update the MFA session with a token. Return it to the user to complete the MFA check.
	if mfaSession != nil {
		username := idToken.IDTokenClaims.Email
		if usernameClaim := connector.GetUsernameClaim(); usernameClaim != "" {
			u, ok := idToken.IDTokenClaims.Claims[usernameClaim].(string)
			if !ok {
				return nil, req.ClientLoginIP, trace.BadParameter("The configured username_claim of %q was not received from the IdP. Please update the username_claim in connector %q.", usernameClaim, connector.GetName())
			}
			username = u
		}
		// validate the mfaSession now that we have the full request details.
		switch {
		case mfaSession.ConnectorID != req.ConnectorID:
			return nil, req.ClientLoginIP, trace.AccessDenied("invalid OIDC MFA session, wrong provider %q", mfaSession.ConnectorID)
		case mfaSession.ConnectorType != constants.OIDC:
			return nil, req.ClientLoginIP, trace.AccessDenied("invalid OIDC MFA session, wrong sso type %q", mfaSession.ConnectorType)
		case mfaSession.Username != username:
			slog.WarnContext(ctx, "User attempted to validate an SSO MFA session belonging to a different user, denied", "user", username)
			return nil, req.ClientLoginIP, trace.AccessDenied("invalid OIDC MFA session")
		}

		token, err := oas.auth.UpsertSSOMFASessionWithToken(ctx, mfaSession)
		if err != nil {
			return nil, req.ClientLoginIP, trace.Wrap(err)
		}

		resp := &authclient.OIDCAuthResponse{
			Req: OIDCAuthRequestFromProto(req),
			Identity: types.ExternalIdentity{
				ConnectorID: req.ConnectorID,
				Username:    username,
			},
			Username: username,
		}

		resp.MFAToken = token
		return resp, req.ClientLoginIP, trace.Wrap(err)
	}

	// Calculate (figure out name, roles, traits, session TTL) of user and
	// create the user in the backend.
	params, err := oas.calculateOIDCUser(ctx, diagCtx, connector, idToken, req)
	if err != nil {
		return nil, req.ClientLoginIP, trace.Wrap(err, "Failed to calculate user attributes.")
	}

	diagCtx.Info.CreateUserParams = &types.CreateUserParams{
		ConnectorName: params.ConnectorName,
		Username:      params.Username,
		KubeGroups:    params.KubeGroups,
		KubeUsers:     params.KubeUsers,
		Roles:         params.Roles,
		Traits:        params.Traits,
		SessionTTL:    types.Duration(params.SessionTTL),
	}

	user, err := oas.createOIDCUser(ctx, params, req.SSOTestFlow)
	if err != nil {
		return nil, req.ClientLoginIP, trace.Wrap(err, "Failed to create user from provided parameters.")
	}

	if err := oas.auth.CallLoginHooks(ctx, user); err != nil {
		return nil, req.ClientLoginIP, trace.Wrap(err)
	}

	userState, err := oas.auth.GetUserOrLoginState(ctx, user.GetName())
	if err != nil {
		return nil, req.ClientLoginIP, trace.Wrap(err)
	}

	// Auth was successful, return session, certificate, etc. to caller.
	resp := &authclient.OIDCAuthResponse{
		Req: OIDCAuthRequestFromProto(req),
		Identity: types.ExternalIdentity{
			ConnectorID: params.ConnectorName,
			Username:    params.Username,
		},
		Username: userState.GetName(),
	}

	// In test flow skip signing and creating web sessions.
	if req.SSOTestFlow {
		diagCtx.Info.Success = true
		return resp, req.ClientLoginIP, nil
	}

	if !req.CheckUser {
		return resp, req.ClientLoginIP, nil
	}

	// If the request is coming from a browser, create a web session.
	if req.CreateWebSession {
		session, err := oas.auth.CreateWebSessionFromReq(ctx, auth.NewWebSessionRequest{
			User:                 userState.GetName(),
			Roles:                userState.GetRoles(),
			Traits:               userState.GetTraits(),
			SessionTTL:           params.SessionTTL,
			LoginTime:            oas.auth.GetClock().Now().UTC(),
			LoginIP:              req.ClientLoginIP,
			LoginUserAgent:       req.ClientUserAgent,
			AttestWebSession:     true,
			CreateDeviceWebToken: true,
			Scope:                req.Scope,
		})
		if err != nil {
			return nil, req.ClientLoginIP, trace.Wrap(err, "Failed to create web session.")
		}
		resp.Session = session
	}

	// If a public key was provided, sign it and return a certificate.
	if len(req.SshPublicKey) > 0 || len(req.TlsPublicKey) > 0 {
		sshCert, tlsCert, err := oas.auth.CreateSessionCerts(ctx, &auth.SessionCertsRequest{
			UserState:               userState,
			SessionTTL:              params.SessionTTL,
			SSHPubKey:               req.SshPublicKey,
			TLSPubKey:               req.TlsPublicKey,
			Compatibility:           req.Compatibility,
			RouteToCluster:          req.RouteToCluster,
			KubernetesCluster:       req.KubernetesCluster,
			LoginIP:                 req.ClientLoginIP,
			SSHAttestationStatement: hardwarekey.AttestationStatementFromProto(req.SshAttestationStatement),
			TLSAttestationStatement: hardwarekey.AttestationStatementFromProto(req.TlsAttestationStatement),
			Scope:                   req.Scope,
		})
		if err != nil {
			return nil, req.ClientLoginIP, trace.Wrap(err, "Failed to create session certificate.")
		}

		clusterName, err := oas.auth.GetClusterName(ctx)
		if err != nil {
			return nil, req.ClientLoginIP, trace.Wrap(err, "Failed to obtain cluster name.")
		}
		resp.Cert = sshCert
		resp.TLSCert = tlsCert

		// Return the host CA for this cluster only.
		authority, err := oas.auth.GetCertAuthority(ctx, types.CertAuthID{
			Type:       types.HostCA,
			DomainName: clusterName.GetClusterName(),
		}, false)
		if err != nil {
			return nil, req.ClientLoginIP, trace.Wrap(err, "Failed to obtain cluster's host CA.")
		}
		resp.HostSigners = append(resp.HostSigners, authority)
	}

	if o, err := oas.auth.ClientOptionsForLogin(userState); err == nil {
		resp.ClientOptions = o
	} else {
		logger.WarnContext(ctx, "Failed to calculate client options for OIDC login", "username", user.GetName(), "error", err)
	}

	return resp, req.ClientLoginIP, nil
}

// wrappedRP wraps an [rp.RelyingParty] so that a custom [http.Client]
// can be provided in order to make it possible to access the status codes
// of HTTP requests. Only the first request made by the client is recorded.
//
// This is used for requests to the user info endpoint in order to determine
// whether it is safe to proceed with just the information from the IDToken
// claims, or if the authentication attempt should be aborted prematurely.
type wrappedRP struct {
	rp.RelyingParty
	statusCode atomic.Int32
}

func (w *wrappedRP) HttpClient() *http.Client {
	clt := w.RelyingParty.HttpClient()

	return &http.Client{
		Transport:     &respCodeRoundTripper{rt: clt.Transport, statusCode: &w.statusCode},
		CheckRedirect: clt.CheckRedirect,
		Jar:           clt.Jar,
		Timeout:       clt.Timeout,
	}
}

// respCodeRoundTripper wraps an [http.RoundTripper] to record the status code
// of the first response issued.
type respCodeRoundTripper struct {
	rt         http.RoundTripper
	statusCode *atomic.Int32
}

func (r *respCodeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := r.rt.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	r.statusCode.CompareAndSwap(int32(0), int32(resp.StatusCode))
	return resp, nil
}

func (oas *OIDCAuthService) retrieveIDTokenClaims(ctx context.Context, connector types.OIDCConnector, code string, req *types.OIDCAuthRequest, codeVerifier string, forMFA bool) (*oidc.Tokens[*oidc.IDTokenClaims], error) {
	relyingParty, err := oas.getRelyingParty(ctx, connector, req.ProxyAddress, forMFA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var opts []rp.CodeExchangeOpt
	// For web sessions, the code verifier is automatically pulled from query params. For non web sessions,
	// we must pull the verifier directly from the request.
	if !req.CreateWebSession {
		codeVerifier = req.PkceVerifier
	}
	// if PKCE is enabled, we must use the code verifier in the exchange. If the original request
	// included a code challenge, even if PKCE is disabled, the code exchange will fail as it is expecting
	// a code verifier included.
	if connector.IsPKCEEnabled() {
		logger.DebugContext(ctx, "PKCE enabled, using code verifier in code exchange")
		opts = append(opts, rp.WithCodeVerifier(codeVerifier))
	}

	idToken, err := rp.CodeExchange[*oidc.IDTokenClaims](ctx, code, relyingParty, opts...)
	if err != nil {
		// different error message for Google Workspace as likely cause is different.
		if isGoogleWorkspaceConnector(connector) {
			return nil, trace.Wrap(err, "Failed to extract OIDC claims. Check your Google Workspace plan and enabled APIs. See: https://goteleport.com/docs/admin-guides/access-controls/sso/google-workspace/#review-your-google-workspace-edition")
		}

		return nil, trace.Wrap(err, "Failed to extract OIDC claims. This may indicate need to set 'provider' flag in connector definition. See: https://goteleport.com/docs/admin-guides/access-controls/sso/#provider-specific-workarounds")
	}

	logger.DebugContext(ctx, "Retrieved OIDC ID Token claims", "claims", idToken.IDTokenClaims.Claims)

	var unsafeUserInfoLookup bool
	for _, u := range []string{relyingParty.Issuer(), relyingParty.UserinfoEndpoint()} {
		parsed, err := url.Parse(u)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if parsed.Scheme != "https" {
			logger.DebugContext(ctx, "OIDC provider doesn't offer valid user info endpoint, using only id token claims")
			unsafeUserInfoLookup = true
		}
	}

	if unsafeUserInfoLookup {
		return idToken, nil
	}

	// The relying party is wrapped so that the status code
	// of the user info response can be inspected.
	wrappedRP := &wrappedRP{RelyingParty: relyingParty}
	userInfo, err := rp.Userinfo[*oidc.UserInfo](ctx, idToken.AccessToken, idToken.TokenType, idToken.IDTokenClaims.GetSubject(), wrappedRP)
	if err != nil {
		switch wrappedRP.statusCode.Load() {
		case http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusForbidden,
			http.StatusMethodNotAllowed:
			logger.DebugContext(ctx, "UserInfo endpoint returned an error, using token claims", "error", err)
			return idToken, nil
		}

		return nil, trace.Wrap(err, "Failed to extract OIDC claims. This may indicate need to set 'provider' flag in connector definition. See: https://goteleport.com/docs/admin-guides/access-controls/sso/#provider-specific-workarounds")
	}

	logger.DebugContext(ctx, "UserInfo claims", "claims", userInfo.Claims)

	// make sure that the subject in the userinfo claim matches the subject in
	// the id token otherwise there is the possibility of a token substitution attack.
	// see section 16.11 of the oidc spec for more details.
	if idToken.IDTokenClaims.Subject != userInfo.Subject {
		logger.DebugContext(ctx, "OIDC claim subjects don't match",
			"id_token_sub", idToken.IDTokenClaims.Subject,
			"user_info_sub", userInfo.Subject,
		)
		return nil, trace.BadParameter("OIDC claim subjects in UserInfo does not match")
	}

	// Merge userInfo claims into IDToken claims.
	mergeIDTokenAndUserInfo := func(i *oidc.IDTokenClaims, u *oidc.UserInfo) {
		// Implement subset of (*oidc.IDTokenClaims).SetUserInfo(i *UserInfo)
		// to override claims known to zitadel/oidc.
		i.Subject = u.Subject
		i.UserInfoProfile = u.UserInfoProfile
		i.UserInfoEmail = u.UserInfoEmail
		i.UserInfoPhone = u.UserInfoPhone
		i.Address = u.Address
		if i.Claims == nil {
			i.Claims = make(map[string]any, len(i.Claims))
		}

		// Overriding claim only if the key does not exist is the
		// legacy behavior (Teleport v17 and below) and needs to be
		// preserved because for the IdP like Azure OIDC IdP (issuer=sts.windows.net),
		// the groups claim value received from the userinfo endpoint
		// is not supported by claims to role mapper and will result
		// in an authentication failure due to zero claims mapped for
		// the user.
		for k, v := range u.Claims {
			_, ok := i.Claims[k]
			if !ok {
				i.Claims[k] = v
			}
		}
	}
	mergeIDTokenAndUserInfo(idToken.IDTokenClaims, userInfo)

	switch {
	case isGoogleWorkspaceConnector(connector):
		if err = addGoogleWorkspaceClaims(ctx, connector, idToken); err != nil {
			return nil, trace.Wrap(err)
		}
	case entraid.IsEntraIDConnector(connector):
		provider := entraid.OIDCEntraIDGroupsProvider{
			Connector:  connector,
			IDToken:    idToken,
			Logger:     logger,
			HTTPClient: oas.client,
		}
		if err := provider.MaybeFetchEntraIDGroups(ctx, nil /* graph client for test */); err != nil {
			// swallowing error here to let the program continue
			// with other claims that may be vaid enough for the
			// SSO to succeed.
			logger.ErrorContext(ctx, "Entra ID groups provider", "error", err)
		}
	}
	return idToken, nil
}

// OIDCAuthRequestFromProto converts the types.OIDCAuthRequest to OIDCAuthRequest.
func OIDCAuthRequestFromProto(req *types.OIDCAuthRequest) authclient.OIDCAuthRequest {
	return authclient.OIDCAuthRequest{
		ConnectorID:       req.ConnectorID,
		SSHPubKey:         req.SshPublicKey,
		TLSPubKey:         req.TlsPublicKey,
		CSRFToken:         req.CSRFToken,
		CreateWebSession:  req.CreateWebSession,
		ClientRedirectURL: req.ClientRedirectURL,
	}
}

// OIDCClaimsToTraits converts OIDC-style claims into teleport-specific trait format
func OIDCClaimsToTraits(claims map[string]any) map[string][]string {
	traits := make(map[string][]string)

	for claimName := range claims {
		switch v := claims[claimName].(type) {
		case string:
			traits[claimName] = []string{v}
		case []string:
			traits[claimName] = v
		case []any:
			claim := make([]string, 0, len(v))
			for _, val := range v {
				if stringVal, ok := val.(string); ok {
					claim = append(claim, stringVal)
				}
			}

			traits[claimName] = claim
		}
	}

	return traits
}

func (oas *OIDCAuthService) calculateOIDCUser(ctx context.Context, diagCtx *auth.SSODiagContext, connector types.OIDCConnector, ident *oidc.Tokens[*oidc.IDTokenClaims], request *types.OIDCAuthRequest) (*auth.CreateUserParams, error) {
	username := ident.IDTokenClaims.Email
	if usernameClaim := connector.GetUsernameClaim(); usernameClaim != "" {
		u, ok := ident.IDTokenClaims.Claims[usernameClaim].(string)
		if !ok {
			return nil, trace.BadParameter("The configured username_claim of %q was not received from the IdP. Please update the username_claim in connector %q.", usernameClaim, connector.GetName())
		}
		username = u
	}

	p := auth.CreateUserParams{
		ConnectorName: connector.GetName(),
		Username:      username,
	}

	evaluationOutput, err := oas.auth.GetLoginRuleEvaluator().Evaluate(ctx, &loginrule.EvaluationInput{
		Traits: OIDCClaimsToTraits(ident.IDTokenClaims.Claims),
		Claims: ident.IDTokenClaims.Claims,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.Traits = evaluationOutput.Traits
	diagCtx.Info.AppliedLoginRules = truncateAppliedLoginRules(evaluationOutput.AppliedRules)

	diagCtx.Info.OIDCTraitsFromClaims = p.Traits
	diagCtx.Info.OIDCConnectorTraitMapping = connector.GetTraitMappings()

	var warnings []string
	warnings, p.Roles = services.TraitsToRoles(connector.GetTraitMappings(), p.Traits)
	if len(p.Roles) == 0 {
		if len(warnings) != 0 {
			logger.WarnContext(ctx, "No roles mapped from claims", "warning", warnings, "connector", connector.WithoutSecrets())
			diagCtx.Info.OIDCClaimsToRolesWarnings = &types.SSOWarnings{
				Message:  "No roles mapped for the user",
				Warnings: warnings,
			}
		} else {
			logger.WarnContext(ctx, "No roles mapped from claims", "connector", connector.WithoutSecrets())
			diagCtx.Info.OIDCClaimsToRolesWarnings = &types.SSOWarnings{
				Message: "No roles mapped for the user. The mappings may contain typos.",
			}
		}
		return nil, trace.Wrap(ErrOIDCNoRoles)
	}

	// Pick smaller for role: session TTL from role or requested TTL.
	roles, err := services.FetchRoles(p.Roles, oas.auth, p.Traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleTTL := roles.AdjustSessionTTL(apidefaults.MaxCertDuration)
	p.SessionTTL = utils.MinTTL(roleTTL, request.CertTTL)

	return &p, nil
}

// truncateAppliedLoginRules truncates a list of login rule names to a maximum
// length of 200. I doubt anyone will have that many rules, but this is to avoid
// the off-chance of too many rules ending up in the UserLogin event such that
// it can't be emitted.
func truncateAppliedLoginRules(rules []string) []string {
	const maxRules = 200
	if len(rules) <= maxRules {
		return rules
	}
	return rules[:maxRules]
}

func (oas *OIDCAuthService) createOIDCUser(ctx context.Context, p *auth.CreateUserParams, dryRun bool) (types.User, error) {
	expires := oas.auth.GetClock().Now().UTC().Add(p.SessionTTL)

	logger.DebugContext(ctx, "Generating dynamic OIDC identity",
		"connector", p.ConnectorName,
		"user", p.Username,
		"roles", p.Roles,
		"dry_run", dryRun,
	)
	user := types.User(&types.UserV2{
		Kind:    types.KindUser,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:    p.Username,
			Expires: &expires,
		},
		Spec: types.UserSpecV2{
			Roles:  p.Roles,
			Traits: p.Traits,
			OIDCIdentities: []types.ExternalIdentity{
				{
					ConnectorID: p.ConnectorName,
					Username:    p.Username,
				},
			},
			CreatedBy: types.CreatedBy{
				User: types.UserRef{Name: teleport.UserSystem},
				Time: oas.auth.GetClock().Now().UTC(),
				Connector: &types.ConnectorRef{
					Type:     constants.OIDC,
					ID:       p.ConnectorName,
					Identity: p.Username,
				},
			},
		},
	})

	if dryRun {
		return user, nil
	}

	// Get the user to check if it already exists or not.
	existingUser, err := oas.auth.Services.GetUser(ctx, p.Username, false)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	// Overwrite existing user if it was created from an external identity provider.
	if existingUser != nil {
		connectorRef := existingUser.GetCreatedBy().Connector

		// If the existing user is a local user, fail and advise how to fix the problem.
		if connectorRef == nil {
			return nil, trace.AlreadyExists("local user with name %q already exists. Either change "+
				"email in OIDC identity or remove local user and try again.", existingUser.GetName())
		}

		logger.DebugContext(ctx, "Overwriting existing user",
			"user", existingUser.GetName(),
			"connector_type", connectorRef.Type,
			"connector_id", connectorRef.ID,
			"origin", existingUser.Origin(),
		)

		// When SCIM integration is enabled, SCIM provisioning manages the lifetime of the SSO user,
		// meaning the user is no longer ephemeral. In this case, user expiration settings are ignored,
		// and the user object should remain persistent.
		// SCIM-originated users must not be overwritten (apart from roles and traits),
		// otherwise it will break SCIM provisioning functionality.
		if existingUser.Origin() == typescommon.OriginSCIM {
			if user, err = oas.handleSCIMOriginUser(ctx, existingUser, user, p); err != nil {
				return nil, trace.Wrap(err, "failed to handle existing SCIM-originated user %q", existingUser.GetName())
			}
		}

		user.SetRevision(existingUser.GetRevision())
		created, err := oas.auth.UpdateUser(ctx, user)
		return created, trace.Wrap(err)
	}

	created, err := oas.auth.CreateUser(ctx, user)
	return created, trace.Wrap(err)
}

func (oas *OIDCAuthService) handleSCIMOriginUser(ctx context.Context, existingUser, newUser types.User, p *auth.CreateUserParams) (types.User, error) {
	existingUserConnector := existingUser.GetCreatedBy().Connector
	if existingUserConnector == nil {
		// SCIM-originated users should always have connector information.
		return nil, trace.BadParameter(
			"The existing SCIM-originated user %q is missing the CreatedBy.Connector field."+
				"Please report this issue and include a description of your setup.", existingUser.GetName(),
		)
	}
	newUserConnector := newUser.GetCreatedBy().Connector
	if newUserConnector == nil {
		return nil, trace.BadParameter("newUser.GetCreatedBy.Connector is empty")

	}
	hasSameConnector := existingUserConnector.ID == newUserConnector.ID &&
		existingUserConnector.Type == newUserConnector.Type

	if !hasSameConnector {
		return nil, trace.AlreadyExists(
			"Cannot create user %q: a SCIM-managed user with the same username already exists, "+
				"but it was created using a different connector (%q) than the one you are using now (%q). "+
				"To proceed, select the Teleport connector %q during the Teleport login flow "+
				"or contact your Teleport administrator to resolve username conflicts across multiple connectors.",
			newUser.GetName(),
			existingUserConnector.ID,
			p.ConnectorName,
			existingUserConnector.ID)
	}

	// Update SCIM users roles and traits without overwriting the whole user object.
	existingUser.SetRoles(p.Roles)
	existingUser.SetTraits(p.Traits)

	return existingUser, nil
}

// validateACRValues validates that we get an appropriate response for acr values. By default
// we expect the same value we send, but this function also handles Identity Provider specific
// forms of validation.
func validateACRValues(ctx context.Context, acrValue string, identityProvider string, idTokenClaims *oidc.Tokens[*oidc.IDTokenClaims]) error {
	switch identityProvider {
	case teleport.NetIQ:
		logger.DebugContext(ctx, "Validating OIDC ACR values with provider", "provider", identityProvider)

		tokenAcr, ok := idTokenClaims.IDTokenClaims.Claims["acr"]
		if !ok {
			return trace.BadParameter("acr not found in claims")
		}
		tokenAcrMap, ok := tokenAcr.(map[string]any)
		if !ok {
			return trace.BadParameter("acr unexpected type: %T", tokenAcr)
		}
		tokenAcrValues, ok := tokenAcrMap["values"]
		if !ok {
			return trace.BadParameter("acr.values not found in claims")
		}
		tokenAcrValuesSlice, ok := tokenAcrValues.([]any)
		if !ok {
			return trace.BadParameter("acr.values unexpected type: %T", tokenAcr)
		}

		for _, v := range tokenAcrValuesSlice {
			vv, ok := v.(string)
			if !ok {
				continue
			}
			if acrValue == vv {
				return nil
			}
		}

		logger.DebugContext(ctx, "No OIDC ACR match found",
			"acr_value", acrValue,
			"token_acr_value", tokenAcrValues,
		)
		return trace.BadParameter("acr claim does not match")

	default:
		logger.DebugContext(ctx, "Validating OIDC ACR values with default rules")

		if idTokenClaims.IDTokenClaims.AuthenticationContextClassReference != "" &&
			acrValue != idTokenClaims.IDTokenClaims.AuthenticationContextClassReference {
			logger.DebugContext(ctx, "No OIDC ACR match found",
				"acr_value", acrValue,
				"claim_value", idTokenClaims.IDTokenClaims.AuthenticationContextClassReference,
			)
			return trace.BadParameter("acr claim does not match")
		}
	}

	return nil
}
