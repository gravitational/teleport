package saml

import (
	"context"
	"encoding/xml"
	"net/url"

	"github.com/beevik/etree"
	"github.com/crewjam/saml"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	samlidppb "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
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
	GetClusterName(...services.MarshalOption) (types.ClusterName, error)

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

	// Log is the logrus logging entry.
	Log *logrus.Entry
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
	if s.Log == nil {
		return trace.BadParameter("logger is missing")
	}

	return nil
}

// NewSAMNewSAMLIdPServiceLIdP will create the new SAML IdP service.
func NewSAMLIdPService(cfg *SAMLIdPServiceConfig) (*SAMLIdPService, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &SAMLIdPService{
		client:     cfg.Client,
		keyStore:   cfg.KeyStore,
		authorizer: cfg.Authorizer,
		log:        cfg.Log,
	}, nil
}

// SAMLIdPService is the SAML IdP service used for
// SAML response signing and testing attribute mapping configuration.
type SAMLIdPService struct {
	samlidppb.UnimplementedSAMLIdPServiceServer

	client     ProcessSAMLIdPRequestClient
	keyStore   *keystore.Manager
	authorizer authz.Authorizer
	log        *logrus.Entry
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
	if len(req.Assertion) == 0 {
		return nil, trace.BadParameter("missing assertions")
	}

	// Parse the assertion XML
	assertion := &saml.Assertion{}
	if err := xml.Unmarshal(req.Assertion, assertion); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Destination == "" {
		return nil, trace.BadParameter("missing destination")
	}

	if req.SignatureMethod == "" {
		return nil, trace.BadParameter("missing signature method")
	}

	// We're intentionally not checking the presence of the request ID, as for
	// IdP initiated SSO it may not be present.

	// Get the metadata URL, which for our IdP implementation doubles as the
	// entity ID.
	metadataURL, err := url.Parse(req.MetadataUrl)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Parse out the service provider SSO descriptor.
	if len(req.ServiceProviderSsoDescriptor) == 0 {
		return nil, trace.BadParameter("missing service provider SSO descriptor")
	}

	var spssoDescriptor saml.SPSSODescriptor
	err = xml.Unmarshal(req.ServiceProviderSsoDescriptor, &spssoDescriptor)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get the cert for signing the request.
	clusterName, err := s.client.GetClusterName()
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
	}

	// Create an authn request that has our various bits and pieces packed into it.
	idpAuthnRequest := &saml.IdpAuthnRequest{
		IDP: idp, // The cert, key, signature method, and entity ID (metadataURL) and keys will be retrieved from the idp object.
		Request: saml.AuthnRequest{
			ID: req.RequestId, // The incoming request ID, will be used to show what this request is in response to.
		},
		ACSEndpoint: &saml.IndexedEndpoint{
			Location: req.Destination, // The destination that this response will be sent to.
		},
		SPSSODescriptor: &spssoDescriptor,         // The service provider SSO descriptor.
		Now:             req.RequestTime.AsTime(), // The request time.
		Assertion:       assertion,                // The assertions to sign.
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
	// only users who can create attribute mapping should be able to test it.
	if err := s.authorizeAccess(ctx, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	var mappableUserSpec []samlMappableUserSpec
	for _, user := range req.Users {
		mappableUserSpec = append(mappableUserSpec, samlMappableUserSpec{
			Username: user.GetName(),
			Roles:    user.Spec.Roles,
			Traits:   user.Spec.Traits,
		})
	}

	var resp samlidppb.TestSAMLIdPAttributeMappingResponse
	for _, userSpec := range mappableUserSpec {
		var attributes []saml.Attribute
		reqAttrs := attributeToRequestedAttribute(req.ServiceProvider.GetAttributeMapping())
		evaluatedAttributes, err := evaluateAttributes(reqAttrs, userSpec)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		attributes = append(attributes, evaluatedAttributes...)
		mapped := attributeToTestSAMLIdPAttributeMappingResponse(userSpec.Username, attributes)
		resp.MappedAttributes = append(resp.MappedAttributes, mapped)
	}

	return &resp, nil
}

// authorizeAccess checks user context with given authorizeVerbs against KindSAMLIdPServiceProvider resource.
func (s *SAMLIdPService) authorizeAccess(ctx context.Context, authorizeVerbs ...string) error {
	authzWithContext, err := authz.AuthorizeWithVerbs(ctx, s.log, s.authorizer, true /* quiet */, types.KindSAMLIdPServiceProvider, authorizeVerbs...)
	if err != nil {
		return trace.Wrap(err)
	}
	if err = authz.AuthorizeAdminAction(ctx, authzWithContext); err != nil {
		return trace.Wrap(err)
	}
	return nil
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
	return &samlidppb.MappedAttribute{
		Username:     user,
		MappedValues: mapped,
	}
}

func attributeValuesToStringSlice(avals []saml.AttributeValue) []string {
	av := make([]string, 0)
	for _, v := range avals {
		av = append(av, v.Value)
	}
	return av
}
