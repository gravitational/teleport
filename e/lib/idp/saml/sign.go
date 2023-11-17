package saml

import (
	"context"
	"encoding/xml"
	"net/url"

	"github.com/beevik/etree"
	"github.com/crewjam/saml"
	"github.com/gravitational/trace"

	samlidppb "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	"github.com/gravitational/teleport/api/types"
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

// SigningServiceConfig is the config for the signing service.
type SigningServiceConfig struct {
	// Client is the SAML client used for retrieving CAs and cluster names.
	Client ProcessSAMLIdPRequestClient

	// KeyStore is the store used to sign SAML IdP response.
	KeyStore *keystore.Manager

	// Authorizer is for authorizing the signing requests.
	Authorizer authz.Authorizer
}

func (s *SigningServiceConfig) CheckAndSetDefaults() error {
	if s.Client == nil {
		return trace.BadParameter("client is missing")
	}
	if s.KeyStore == nil {
		return trace.BadParameter("key store is missing")
	}
	if s.Authorizer == nil {
		return trace.BadParameter("authorizer is missing")
	}

	return nil
}

// NewSigningServicew will create the new signing service.
func NewSigningService(cfg *SigningServiceConfig) (*SigningService, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &SigningService{
		client:     cfg.Client,
		keyStore:   cfg.KeyStore,
		authorizer: cfg.Authorizer,
	}, nil
}

// SigningService is the service that signs SAML IdP responses.
type SigningService struct {
	samlidppb.UnimplementedSAMLIdPServiceServer

	client     ProcessSAMLIdPRequestClient
	keyStore   *keystore.Manager
	authorizer authz.Authorizer
}

// ProcessSAMLIdPRequest makes a signed SAML response to a SAML auth request.
//
//nolint:revive // Because we want this to be IdP.
func (s *SigningService) ProcessSAMLIdPRequest(ctx context.Context, req *samlidppb.ProcessSAMLIdPRequestRequest) (*samlidppb.ProcessSAMLIdPRequestResponse, error) {
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
