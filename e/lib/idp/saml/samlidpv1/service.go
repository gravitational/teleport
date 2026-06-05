package samlidpv1

import (
	"context"
	"encoding/xml"
	stdlog "log"
	"log/slog"
	"net/url"

	"github.com/beevik/etree"
	"github.com/crewjam/saml"
	"github.com/gravitational/trace"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	samlidppb "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/e/lib/idp/saml/attribute"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

// ProcessSAMLIdPRequestClient is a client for retrieving CAs and cluster names, which is
// necessary for creating SAML responses.
type ProcessSAMLIdPRequestClient interface {
	services.AuthorityGetter

	// GetClusterName returns the local cluster name
	GetClusterName(ctx context.Context) (types.ClusterName, error)

	// ListSAMLIdPServiceProviders returns a paginated list of SAML IdP service provider resources.
	ListSAMLIdPServiceProviders(ctx context.Context, pageSize int, nextToken string) ([]types.SAMLIdPServiceProvider, string, error)
}

// SAMLIdPServiceConfig is the config for the SAML IdP service.
type SAMLIdPServiceConfig struct {
	// Client is the SAML client used for retrieving CAs and cluster names.
	Client ProcessSAMLIdPRequestClient

	// KeyStore is the store used to sign SAML IdP response.
	KeyStore *keystore.Manager

	// Authorizer is for authorizing the signing requests.
	Authorizer authz.Authorizer

	// MFAAuthenticator is for authenticating user MFA challenge responses.
	MFAAuthenticator authz.MFAAuthenticator

	// Logger emits log messages.
	Logger *slog.Logger
}

func (s *SAMLIdPServiceConfig) CheckAndSetDefaults() error {
	if s.Client == nil {
		return trace.BadParameter("client is missing")
	}
	if s.KeyStore == nil {
		return trace.BadParameter("key store is missing")
	}
	if s.Authorizer == nil {
		return trace.BadParameter("authorizer is missing")
	}
	if s.MFAAuthenticator == nil {
		return trace.BadParameter("mfa authenticator is missing")
	}
	if s.Logger == nil {
		s.Logger = slog.Default()
	}

	return nil
}

// NewSAMNewSAMLIdPServiceLIdP will create the new SAML IdP service.
func NewSAMLIdPService(cfg SAMLIdPServiceConfig) (*SAMLIdPService, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &SAMLIdPService{
		client:           cfg.Client,
		keyStore:         cfg.KeyStore,
		authorizer:       cfg.Authorizer,
		mfaAuthenticator: cfg.MFAAuthenticator,
		logger:           cfg.Logger,
	}, nil
}

// SAMLIdPService is the SAML IdP service used for
// SAML response signing and testing attribute mapping configuration.
type SAMLIdPService struct {
	samlidppb.UnimplementedSAMLIdPServiceServer

	client           ProcessSAMLIdPRequestClient
	keyStore         *keystore.Manager
	authorizer       authz.Authorizer
	mfaAuthenticator authz.MFAAuthenticator
	logger           *slog.Logger
}

// ProcessSAMLIdPRequest makes a signed SAML response to a SAML auth request.
//
//nolint:revive // Because we want this to be IdP.
func (s *SAMLIdPService) ProcessSAMLIdPRequest(ctx context.Context, req *samlidppb.ProcessSAMLIdPRequestRequest) (*samlidppb.ProcessSAMLIdPRequestResponse, error) {
	// Only the proxy can sign SAML IdP requests.
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.AccessDenied("only the proxy may create SAML IdP responses")
	}

	if !authz.HasBuiltinRole(*authCtx, string(types.RoleProxy)) {
		return nil, trace.AccessDenied("this request can be only executed by a proxy")
	}

	// There needs to be a few assertions.
	if len(req.GetAssertion()) == 0 {
		return nil, trace.BadParameter("missing assertions")
	}

	// Parse the assertion XML
	assertion := &saml.Assertion{}
	if err := xml.Unmarshal(req.GetAssertion(), assertion); err != nil {
		return nil, trace.Wrap(err)
	}

	// If an MFA response is provided, validate it against the user in the saml assertion.
	// The Proxy service authorized this SAML request on the basis of this verification succeeding.
	//
	// TODO(Joerger): Ideally we would authorize the user's request fully on the Auth service side,
	// rather than just validating the MFA response and trusting the Proxy to do the rest. To do this,
	// we could move the saml session + assertion creation logic out of the proxy and into here. This
	// would also cut down on round trips and move SAML IdP Service trust to the Auth Service.
	if req.HasMfaResponse() {
		username := assertion.Subject.NameID.Value
		ext := &mfav1.ChallengeExtensions{Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION}
		if _, err := s.mfaAuthenticator.ValidateMFAAuthResponse(ctx, req.GetMfaResponse(), username, ext); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if req.GetDestination() == "" {
		return nil, trace.BadParameter("missing destination")
	}

	if req.GetSignatureMethod() == "" {
		return nil, trace.BadParameter("missing signature method")
	}

	// We're intentionally not checking the presence of the request ID, as for
	// IdP initiated SSO it may not be present.

	// Get the metadata URL, which for our IdP implementation doubles as the
	// entity ID.
	metadataURL, err := url.Parse(req.GetMetadataUrl())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Parse out the service provider SSO descriptor.
	if len(req.GetServiceProviderSsoDescriptor()) == 0 {
		return nil, trace.BadParameter("missing service provider SSO descriptor")
	}

	var spssoDescriptor saml.SPSSODescriptor
	err = xml.Unmarshal(req.GetServiceProviderSsoDescriptor(), &spssoDescriptor)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get the cert for signing the request.
	clusterName, err := s.client.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ca, err := s.client.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.SAMLIDPCA,
		DomainName: clusterName.GetClusterName(),
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rawCert, signer, err := s.keyStore.GetTLSCertAndSigner(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := tlsca.ParseCertificatePEM(rawCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create an IdP object to stuff into the IdpAuthnRequest.
	idp := &saml.IdentityProvider{
		Certificate:     cert,
		Signer:          signer,
		SignatureMethod: req.GetSignatureMethod(),
		MetadataURL:     *metadataURL,
		Logger:          stdlog.Default(),
	}

	// Create an authn request that has our various bits and pieces packed into it.
	idpAuthnRequest := &saml.IdpAuthnRequest{
		IDP: idp, // The cert, key, signature method, and entity ID (metadataURL) and keys will be retrieved from the idp object.
		Request: saml.AuthnRequest{
			ID: req.GetRequestId(), // The incoming request ID, will be used to show what this request is in response to.
		},
		ACSEndpoint: &saml.IndexedEndpoint{
			Location: req.GetDestination(), // The destination that this response will be sent to.
		},
		SPSSODescriptor: &spssoDescriptor,              // The service provider SSO descriptor.
		Now:             req.GetRequestTime().AsTime(), // The request time.
		Assertion:       assertion,                     // The assertions to sign.
	}

	// Make the signed response. We'll lean on crewjam's implementation here.
	if err := idpAuthnRequest.MakeResponse(); err != nil {
		return nil, trace.NewAggregate(trace.BadParameter("error trying to make response while signing the auth request"), err)
	}

	// Convert the response XML into bytes to pack into the response to this call.
	responseDoc := etree.NewDocument()
	responseDoc.SetRoot(idpAuthnRequest.ResponseEl)

	resp := &samlidppb.ProcessSAMLIdPRequestResponse{}
	resp.Response, err = responseDoc.WriteToBytes()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

func (s *SAMLIdPService) TestSAMLIdPAttributeMapping(ctx context.Context, req *samlidppb.TestSAMLIdPAttributeMappingRequest) (*samlidppb.TestSAMLIdPAttributeMappingResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// only users who can create attribute mapping should be able to test it.
	if err := authCtx.CheckAccessToKind(types.KindSAMLIdPServiceProvider, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	if err = authCtx.AuthorizeAdminAction(); err != nil {
		return nil, trace.Wrap(err)
	}

	var mappableUserSpec []attribute.SAMLMappableUserSpec
	for _, user := range req.GetUsers() {
		mappableUserSpec = append(mappableUserSpec, attribute.SAMLMappableUserSpec{
			Username: user.GetName(),
			Roles:    user.Spec.Roles,
			Traits:   user.Spec.Traits,
		})
	}

	var resp samlidppb.TestSAMLIdPAttributeMappingResponse
	for _, userSpec := range mappableUserSpec {
		var attributes []saml.Attribute
		reqAttrs := attributeToRequestedAttribute(req.GetServiceProvider().GetAttributeMapping())
		evaluatedAttributes, err := attribute.EvaluateAttributes(reqAttrs, userSpec)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		attributes = append(attributes, evaluatedAttributes...)
		mapped := attributeToTestSAMLIdPAttributeMappingResponse(userSpec.Username, attributes)
		resp.SetMappedAttributes(append(resp.GetMappedAttributes(), mapped))
	}

	return &resp, nil
}

func attributeToRequestedAttribute(attributes []*types.SAMLAttributeMapping) (reqAttrs []saml.RequestedAttribute) {
	for _, v := range attributes {
		reqAttrs = append(reqAttrs, saml.RequestedAttribute{
			Attribute: saml.Attribute{
				FriendlyName: v.Name,
				Name:         v.Name,
				NameFormat:   v.NameFormat,
				Values:       []saml.AttributeValue{{Value: v.Value}},
			},
		})
	}
	return
}

func attributeToTestSAMLIdPAttributeMappingResponse(user string, attributes []saml.Attribute) *samlidppb.MappedAttribute {
	mapped := make(map[string]*wrappers.StringValues)
	for _, attr := range attributes {
		mapped[attr.Name] = &wrappers.StringValues{
			Values: attributeValuesToStringSlice(attr.Values),
		}
	}
	return samlidppb.MappedAttribute_builder{
		Username:     user,
		MappedValues: mapped,
	}.Build()
}

func attributeValuesToStringSlice(avals []saml.AttributeValue) []string {
	av := make([]string, 0)
	for _, v := range avals {
		av = append(av, v.Value)
	}
	return av
}
