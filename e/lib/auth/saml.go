package auth

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/beevik/etree"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	saml2 "github.com/russellhaering/gosaml2"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/loginrule"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
)

// SAMLAuthService implements the logic of the SAML connector, allowing SSO
// logins using the SAML protocol.
//
// SAMLAuthService implements the SAMLService interface.
type SAMLAuthService struct {
	auth                   *auth.Server
	emitter                apievents.Emitter
	assertionReplayService *local.AssertionReplayService
	license                License
	samlProviders          map[string]*samlProvider
	lock                   sync.Mutex
}

type SAMLAuthServiceConfig struct {
	Auth                   *auth.Server
	Emitter                apievents.Emitter
	AssertionReplayService *local.AssertionReplayService
	License                License
}

const maxCompressedBytes = 1024 * 1024 // 1 MB, will error if size is hit

func (cfg *SAMLAuthServiceConfig) CheckAndSetDefaults() error {
	if cfg.Auth == nil {
		return trace.BadParameter("auth.Server not provided")
	}
	if cfg.License == nil {
		return trace.BadParameter("License not provided")
	}
	if cfg.AssertionReplayService == nil {
		cfg.AssertionReplayService = cfg.Auth.Unstable.AssertionReplayService
	}
	if cfg.Emitter == nil {
		cfg.Emitter = events.NewDiscardEmitter()
	}
	return nil
}

// NewSAMLAuthService returns a SAMLAuthService configured to use the
// services given in the config.
func NewSAMLAuthService(cfg *SAMLAuthServiceConfig) (*SAMLAuthService, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, err
	}

	return &SAMLAuthService{
		auth:                   cfg.Auth,
		emitter:                cfg.Emitter,
		assertionReplayService: cfg.AssertionReplayService,
		license:                cfg.License,
		samlProviders:          make(map[string]*samlProvider),
	}, nil
}

// samlProvider is internal structure that stores SAML client and its config
type samlProvider struct {
	provider  *saml2.SAMLServiceProvider
	connector types.SAMLConnector
}

// ErrSAMLNoRoles results from not mapping any roles from SAML claims.
var ErrSAMLNoRoles = trace.AccessDenied("No roles mapped from claims. The mappings may contain typos.")

func (sas *SAMLAuthService) CreateSAMLAuthRequest(ctx context.Context, req types.SAMLAuthRequest) (*types.SAMLAuthRequest, error) {
	if sas.license.IsDisabled() {
		return nil, ErrLicenseExpired
	}

	connector, provider, err := sas.getSAMLConnectorAndProvider(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	doc, err := provider.BuildAuthRequestDocument()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	attr := doc.Root().SelectAttr("ID")
	if attr == nil || attr.Value == "" {
		return nil, trace.BadParameter("missing auth request ID")
	}

	req.ID = attr.Value

	// Workaround for Ping: Ping expects `SigAlg` and `Signature` query
	// parameters when "Enforce Signed Authn Request" is enabled, but gosaml2
	// only provides these parameters when binding == BindingHttpRedirect.
	// Luckily, BuildAuthURLRedirect sets this and is otherwise identical to
	// the standard BuildAuthURLFromDocument.
	if connector.GetProvider() == teleport.Ping {
		req.RedirectURL, err = provider.BuildAuthURLRedirect("", doc)
	} else {
		req.RedirectURL, err = provider.BuildAuthURLFromDocument("", doc)
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = sas.auth.Services.CreateSAMLAuthRequest(ctx, req, defaults.SAMLAuthRequestTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &req, nil
}

func (sas *SAMLAuthService) getSAMLConnectorAndProviderByID(ctx context.Context, connectorID string) (types.SAMLConnector, *saml2.SAMLServiceProvider, error) {
	connector, err := sas.auth.Identity.GetSAMLConnector(ctx, connectorID, true)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	provider, err := sas.getSAMLProvider(connector)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return connector, provider, nil
}

func (sas *SAMLAuthService) getSAMLConnectorAndProvider(ctx context.Context, req types.SAMLAuthRequest) (types.SAMLConnector, *saml2.SAMLServiceProvider, error) {
	if req.SSOTestFlow {
		if req.ConnectorSpec == nil {
			return nil, nil, trace.BadParameter("ConnectorSpec cannot be nil when SSOTestFlow is true")
		}

		if req.ConnectorID == "" {
			return nil, nil, trace.BadParameter("ConnectorID cannot be empty")
		}

		// stateless test flow
		connector, err := types.NewSAMLConnector(req.ConnectorID, *req.ConnectorSpec)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		// validate, set defaults for connector
		err = services.ValidateSAMLConnector(connector, sas.auth)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		// we don't want to cache the provider. construct it directly instead of using sas.getSAMLProvider()
		provider, err := services.GetSAMLServiceProvider(connector, sas.auth.GetClock())
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		return connector, provider, nil
	}

	// regular execution flow
	return sas.getSAMLConnectorAndProviderByID(ctx, req.ConnectorID)
}

func (sas *SAMLAuthService) getSAMLProvider(conn types.SAMLConnector) (*saml2.SAMLServiceProvider, error) {
	sas.lock.Lock()
	defer sas.lock.Unlock()

	providerPack, ok := sas.samlProviders[conn.GetName()]
	if ok && cmp.Equal(providerPack.connector, conn) {
		return providerPack.provider, nil
	}
	delete(sas.samlProviders, conn.GetName())

	serviceProvider, err := services.GetSAMLServiceProvider(conn, sas.auth.GetClock())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sas.samlProviders[conn.GetName()] = &samlProvider{connector: conn, provider: serviceProvider}

	return serviceProvider, nil
}

func (sas *SAMLAuthService) calculateSAMLUser(ctx context.Context, diagCtx *auth.SSODiagContext, connector types.SAMLConnector, assertionInfo saml2.AssertionInfo, request *types.SAMLAuthRequest) (*auth.CreateUserParams, error) {
	p := auth.CreateUserParams{
		ConnectorName: connector.GetName(),
		Username:      assertionInfo.NameID,
	}

	p.Traits = services.SAMLAssertionsToTraits(assertionInfo)

	evaluationOutput, err := sas.auth.GetLoginRuleEvaluator().Evaluate(ctx, &loginrule.EvaluationInput{
		Traits: p.Traits,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.Traits = evaluationOutput.Traits
	diagCtx.Info.AppliedLoginRules = truncateAppliedLoginRules(evaluationOutput.AppliedRules)

	diagCtx.Info.SAMLTraitsFromAssertions = p.Traits
	diagCtx.Info.SAMLConnectorTraitMapping = connector.GetTraitMappings()

	var warnings []string
	warnings, p.Roles = services.TraitsToRoles(connector.GetTraitMappings(), p.Traits)
	if len(p.Roles) == 0 {
		if len(warnings) != 0 {
			log.WithField("connector", connector).Warnf("No roles mapped from claims. Warnings: %q", warnings)
			diagCtx.Info.SAMLAttributesToRolesWarnings = &types.SSOWarnings{
				Message:  "No roles mapped for the user",
				Warnings: warnings,
			}
		} else {
			log.WithField("connector", connector).Warnf("No roles mapped from claims.")
			diagCtx.Info.SAMLAttributesToRolesWarnings = &types.SSOWarnings{
				Message: "No roles mapped for the user. The mappings may contain typos.",
			}
		}
		return nil, trace.Wrap(ErrSAMLNoRoles)
	}

	// Pick smaller for role: session TTL from role or requested TTL.
	roles, err := services.FetchRoles(p.Roles, sas.auth, p.Traits)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleTTL := roles.AdjustSessionTTL(apidefaults.MaxCertDuration)

	if request != nil {
		p.SessionTTL = utils.MinTTL(roleTTL, request.CertTTL)
	} else {
		p.SessionTTL = roleTTL
	}

	return &p, nil
}

func (sas *SAMLAuthService) createSAMLUser(ctx context.Context, p *auth.CreateUserParams, dryRun bool) (types.User, error) {
	expires := sas.auth.GetClock().Now().UTC().Add(p.SessionTTL)

	log.Debugf("Generating dynamic SAML identity %v/%v with roles: %v. Dry run: %v.", p.ConnectorName, p.Username, p.Roles, dryRun)

	user := &types.UserV2{
		Kind:    types.KindUser,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      p.Username,
			Namespace: apidefaults.Namespace,
			Expires:   &expires,
		},
		Spec: types.UserSpecV2{
			Roles:  p.Roles,
			Traits: p.Traits,
			SAMLIdentities: []types.ExternalIdentity{
				{
					ConnectorID: p.ConnectorName,
					Username:    p.Username,
				},
			},
			CreatedBy: types.CreatedBy{
				User: types.UserRef{
					Name: teleport.UserSystem,
				},
				Time: sas.auth.GetClock().Now().UTC(),
				Connector: &types.ConnectorRef{
					Type:     constants.SAML,
					ID:       p.ConnectorName,
					Identity: p.Username,
				},
			},
		},
	}

	if dryRun {
		return user, nil
	}

	// Get the user to check if it already exists or not.
	existingUser, err := sas.auth.Services.GetUser(ctx, p.Username, false)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	// Overwrite existing user if it was created from an external identity provider.
	if existingUser != nil {
		connectorRef := existingUser.GetCreatedBy().Connector

		// If the existing user is a local user, fail and advise how to fix the problem.
		if connectorRef == nil {
			return nil, trace.AlreadyExists("local user with name %q already exists. Either change "+
				"NameID in assertion or remove local user and try again.", existingUser.GetName())
		}

		log.Debugf("Overwriting existing user %q created with %v connector %v.",
			existingUser.GetName(), connectorRef.Type, connectorRef.ID)

		user.SetRevision(existingUser.GetRevision())
		updated, err := sas.auth.UpdateUser(ctx, user)
		return updated, trace.Wrap(err)
	}

	created, err := sas.auth.CreateUser(ctx, user)
	return created, trace.Wrap(err)
}

func ParseSAMLInResponseTo(response string) (string, error) {
	raw, _ := base64.StdEncoding.DecodeString(response)

	doc := etree.NewDocument()
	err := doc.ReadFromBytes(raw)
	if err != nil {
		// Attempt to inflate the response in case it happens to be compressed (as with one case at saml.oktadev.com)
		lr := io.LimitReader(flate.NewReader(bytes.NewReader(raw)), maxCompressedBytes)
		buf := new(bytes.Buffer)
		if _, err := io.Copy(buf, lr); err != nil {
			return "", trace.Wrap(err)
		} else if buf.Len() == maxCompressedBytes {
			return "", trace.LimitExceeded("compressed SAML response exceeded max size")
		}

		doc = etree.NewDocument()
		err = doc.ReadFromBytes(buf.Bytes())
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	if doc.Root() == nil {
		return "", trace.BadParameter("unable to parse response")
	}

	// Try to find the InResponseTo attribute in the SAML response. If we can't find this, return
	// a predictable error message so the caller may choose interpret it as an IdP-initiated payload.
	el := doc.Root()
	responseTo := el.SelectAttr("InResponseTo")
	if responseTo == nil {
		return "", trace.NotFound("missing InResponseTo attribute")
	}
	if responseTo.Value == "" {
		return "", trace.BadParameter("InResponseTo can not be empty")
	}
	return responseTo.Value, nil
}

// SAMLAuthRequestFromProto converts the types.SAMLAuthRequest to SAMLAuthRequestData.
func SAMLAuthRequestFromProto(req *types.SAMLAuthRequest) auth.SAMLAuthRequest {
	return auth.SAMLAuthRequest{
		ID:                req.ID,
		PublicKey:         req.PublicKey,
		CSRFToken:         req.CSRFToken,
		CreateWebSession:  req.CreateWebSession,
		ClientRedirectURL: req.ClientRedirectURL,
	}
}

// ValidateSAMLResponse consumes attribute statements from SAML identity provider
func (sas *SAMLAuthService) ValidateSAMLResponse(ctx context.Context, samlResponse, connectorID, clientIP string) (*auth.SAMLAuthResponse, error) {
	if sas.license.IsDisabled() {
		return nil, ErrLicenseExpired
	}

	event := &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
		},
		Method: events.LoginMethodSAML,
	}

	diagCtx := auth.NewSSODiagContext(types.KindSAML, sas.auth)

	auth, err := sas.validateSAMLResponse(ctx, diagCtx, samlResponse, connectorID, clientIP)
	diagCtx.Info.Error = trace.UserMessage(err)

	diagCtx.WriteToBackend(ctx)

	event.AppliedLoginRules = diagCtx.Info.AppliedLoginRules

	attributeStatements := diagCtx.Info.SAMLAttributeStatements
	if attributeStatements != nil {
		attributes, err := apievents.EncodeMapStrings(attributeStatements)
		if err != nil {
			event.Status.UserMessage = fmt.Sprintf("Failed to encode identity attributes: %v", err.Error())
			log.WithError(err).Debug("Failed to encode identity attributes.")
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
		if err := sas.emitter.EmitAuditEvent(ctx, event); err != nil {
			log.WithError(err).Warn("Failed to emit SAML login failed event.")
		}
		return nil, trace.Wrap(err)
	}

	event.Status.Success = true
	event.User = auth.Username
	event.Code = events.UserSSOLoginCode
	if diagCtx.Info.TestFlow {
		event.Code = events.UserSSOTestFlowLoginCode
	}

	if err := sas.emitter.EmitAuditEvent(ctx, event); err != nil {
		log.WithError(err).Warn("Failed to emit SAML login event.")
	}

	return auth, nil
}

// Name of the test connector used to simulate identity provider initiated flow.
const idpInitiatedSAMLTestConn = "idp-initiated-saml-test-conn"

func (sas *SAMLAuthService) checkIDPInitiatedSAML(ctx context.Context, connector types.SAMLConnector, assertion *saml2.AssertionInfo) error {
	if !connector.GetAllowIDPInitiated() && connector.GetName() != idpInitiatedSAMLTestConn {
		return trace.AccessDenied("IdP initiated SAML is not allowed by the connector configuration")
	}

	// Not all IdP's provide these variables, replay mitigation is best effort.
	if assertion.SessionIndex != "" || assertion.SessionNotOnOrAfter == nil {
		return nil
	}

	err := sas.assertionReplayService.RecognizeSSOAssertion(ctx, connector.GetName(), assertion.SessionIndex, assertion.NameID, *assertion.SessionNotOnOrAfter)
	return trace.Wrap(err)
}

func (sas *SAMLAuthService) validateSAMLResponse(ctx context.Context, diagCtx *auth.SSODiagContext, samlResponse, connectorID, clientIP string) (*auth.SAMLAuthResponse, error) {
	idpInitiated := false
	var connector types.SAMLConnector
	var provider *saml2.SAMLServiceProvider
	var request *types.SAMLAuthRequest
	requestID, err := ParseSAMLInResponseTo(samlResponse)

	if connectorID == idpInitiatedSAMLTestConn {
		// Simulate identity provider initiated SAML login, when there's no original request on our side.
		requestID = ""
		err = trace.NotFound("")
		diagCtx.Info.TestFlow = true
	}

	switch {
	case trace.IsNotFound(err):
		if connectorID == "" {
			return nil, trace.BadParameter("ACS URI did not include a valid SAML connector ID parameter")
		}

		idpInitiated = true
		connector, provider, err = sas.getSAMLConnectorAndProviderByID(ctx, connectorID)
		if err != nil {
			return nil, trace.Wrap(err, "Failed to get SAML connector and provider")
		}
	case err != nil:
		return nil, trace.Wrap(err)
	default:
		diagCtx.RequestID = requestID
		request, err = sas.auth.Identity.GetSAMLAuthRequest(ctx, requestID)
		if err != nil {
			return nil, trace.Wrap(err, "Failed to get SAML Auth Request")
		}

		diagCtx.Info.TestFlow = request.SSOTestFlow
		connector, provider, err = sas.getSAMLConnectorAndProvider(ctx, *request)
		if err != nil {
			return nil, trace.Wrap(err, "Failed to get SAML connector and provider")
		}
	}

	assertionInfo, err := provider.RetrieveAssertionInfo(samlResponse)
	if err != nil {
		samlErr := trace.AccessDenied("received response with incorrect or missing attribute statements, please check the identity provider configuration to make sure that mappings for claims/attribute statements are set up correctly. <See: https://goteleport.com/teleport/docs/enterprise/sso/ssh-sso/>, failed to retrieve SAML assertion info from response: %v.", err)
		return nil, trace.WithUserMessage(samlErr, "Failed to retrieve assertion info. This may indicate IdP configuration error.")
	}

	if assertionInfo != nil {
		diagCtx.Info.SAMLAssertionInfo = (*types.AssertionInfo)(assertionInfo)
	}

	if idpInitiated {
		if err := sas.checkIDPInitiatedSAML(ctx, connector, assertionInfo); err != nil {
			if trace.IsAccessDenied(err) {
				log.Warnf("Failed to process IdP-initiated login request. IdP-initiated login is disabled for this connector: %v.", err)
			}

			return nil, trace.Wrap(err)
		}
	}

	if assertionInfo.WarningInfo.InvalidTime {
		samlErr := trace.AccessDenied("invalid time in SAML assertion info")
		return nil, trace.WithUserMessage(samlErr, "SAML assertion info contained warning: invalid time.")
	}

	if assertionInfo.WarningInfo.NotInAudience {
		samlErr := trace.AccessDenied("no audience in SAML assertion info")
		return nil, trace.WithUserMessage(samlErr, "SAML: not in expected audience. Check auth connector audience field and IdP configuration for typos and other errors.")
	}

	log.Debugf("Obtained SAML assertions for %q.", assertionInfo.NameID)
	log.Debugf("SAML assertion warnings: %+v.", assertionInfo.WarningInfo)

	attributeStatements := map[string][]string{}

	for key, val := range assertionInfo.Values {
		var vals []string
		for _, vv := range val.Values {
			vals = append(vals, vv.Value)
		}
		log.Debugf("SAML assertion: %q: %q.", key, vals)
		attributeStatements[key] = vals
	}

	diagCtx.Info.SAMLAttributeStatements = attributeStatements
	diagCtx.Info.SAMLAttributesToRoles = connector.GetAttributesToRoles()

	user, err := sas.auth.GetUser(ctx, assertionInfo.NameID, false)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	// We don't want to overwrite a sync-service based user with an
	// ephemeral one, so we do the user upsert *IF* and *ONLY IF* we
	// determine that:
	//
	//  - There is no user with that name in the system, or
	//  - The user with that name is an ephemeral, sso-connector
	//    created user.
	//
	// All Sync-service create users have an origin label, while those created
	// via calculateSAMLUser do not.
	var sessionTTL time.Duration
	if user == nil || user.Origin() == "" {
		// This is an ephemeral SAML user: we can happily update and overwrite
		// this user.
		if len(connector.GetAttributesToRoles()) == 0 {
			samlErr := trace.BadParameter("no attributes to roles mapping, check connector documentation")
			return nil, trace.WithUserMessage(samlErr, "Attributes-to-roles mapping is empty, SSO user will never have any roles.")
		}

		log.Debugf("Applying %v SAML attribute to roles mappings.", len(connector.GetAttributesToRoles()))

		// Calculate (figure out name, roles, traits, session TTL) of user and
		// create the user in the backend.
		params, err := sas.calculateSAMLUser(ctx, diagCtx, connector, *assertionInfo, request)
		if err != nil {
			return nil, trace.Wrap(err, "Failed to calculate user attributes.")
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

		user, err = sas.createSAMLUser(ctx, params, diagCtx.Info.TestFlow)
		if err != nil {
			return nil, trace.Wrap(err, "Failed to create user from provided parameters.")
		}

		sessionTTL = params.SessionTTL
	} else {
		// Calculate the session TTL as the minimum of all TTLs associated with
		// the user and their roles.
		roles, err := services.FetchRoles(user.GetRoles(), sas.auth, user.GetTraits())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		sessionTTL = roles.AdjustSessionTTL(apidefaults.MaxCertDuration)
	}

	if err := sas.auth.CallLoginHooks(ctx, user); err != nil {
		return nil, trace.Wrap(err)
	}

	userState, err := sas.auth.GetUserOrLoginState(ctx, user.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Auth was successful, return session, certificate, etc. to caller.
	resp := &auth.SAMLAuthResponse{
		Identity: types.ExternalIdentity{
			ConnectorID: user.GetCreatedBy().Connector.ID,
			Username:    user.GetName(),
		},
		Username: userState.GetName(),
	}

	if request != nil {
		resp.Req = SAMLAuthRequestFromProto(request)
	} else {
		resp.Req = auth.SAMLAuthRequest{
			CreateWebSession: true,
		}
	}

	loginIP := ""
	if request != nil {
		loginIP = request.ClientLoginIP
	} else if clientIP != "" {
		// In case of IdP initiated login we don't have a request with the client IP, so we take the IP from
		// incoming connection, sent by the Proxy.
		loginIP = clientIP
	} else if addr, err := authz.ClientSrcAddrFromContext(ctx); err == nil {
		host, _, err := net.SplitHostPort(addr.String())
		if err != nil {
			return nil, trace.Wrap(err, "failed to parse client source address")
		}
		loginIP = host
	}
	// If the request is coming from a browser, create a web session.
	if request == nil || request.CreateWebSession {
		session, err := sas.auth.CreateWebSessionFromReq(ctx, types.NewWebSessionRequest{
			User:             userState.GetName(),
			Roles:            userState.GetRoles(),
			Traits:           userState.GetTraits(),
			SessionTTL:       sessionTTL,
			LoginTime:        sas.auth.GetClock().Now().UTC(),
			LoginIP:          loginIP,
			AttestWebSession: true,
		})
		if err != nil {
			return nil, trace.Wrap(err, "Failed to create web session.")
		}

		resp.Session = session
	}

	// If a public key was provided, sign it and return a certificate.
	if request != nil && len(request.PublicKey) != 0 {
		sshCert, tlsCert, err := sas.auth.CreateSessionCert(userState, sessionTTL, request.PublicKey, request.Compatibility, request.RouteToCluster,
			request.KubernetesCluster, loginIP, keys.AttestationStatementFromProto(request.AttestationStatement))
		if err != nil {
			return nil, trace.Wrap(err, "Failed to create session certificate.")
		}
		clusterName, err := sas.auth.GetClusterName()
		if err != nil {
			return nil, trace.Wrap(err, "Failed to obtain cluster name.")
		}
		resp.Cert = sshCert
		resp.TLSCert = tlsCert

		// Return the host CA for this cluster only.
		authority, err := sas.auth.GetCertAuthority(ctx, types.CertAuthID{
			Type:       types.HostCA,
			DomainName: clusterName.GetClusterName(),
		}, false)
		if err != nil {
			return nil, trace.Wrap(err, "Failed to obtain cluster's host CA.")
		}
		resp.HostSigners = append(resp.HostSigners, authority)
	}

	diagCtx.Info.Success = true
	return resp, nil
}

// validateSAMLResponseWeb provides a HTTP/JSON interface to
// SAMLAuthService.ValidateSAMLResponse. It is called by a teleport proxy in
// response to it receiving the SAML callback (ACS) from the identity provider.
func validateSAMLResponseWeb(authClient *auth.ServerWithRoles, w http.ResponseWriter, r *http.Request, p httprouter.Params, version string) (interface{}, error) {
	var req *auth.ValidateSAMLResponseReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := authClient.ValidateSAMLResponse(r.Context(), req.Response, req.ConnectorID, req.ClientIP)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	raw := auth.SAMLAuthRawResponse{
		Username: response.Username,
		Identity: response.Identity,
		Cert:     response.Cert,
		Req:      response.Req,
		TLSCert:  response.TLSCert,
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
