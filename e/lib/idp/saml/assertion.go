package saml

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"

	"github.com/beevik/etree"
	"github.com/crewjam/saml"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/client/proto"
	samlidppb "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/idp/saml/attribute"
	"github.com/gravitational/teleport/lib/client" // TODO(cthach): Move common MFA types to a separate package.
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
)

// MakeAssertion implements AssertionMaker. It produces a SAML assertion from the
// given request and assigns it to req.Assertion. The incoming request is assumed
// to be valid, and the functions that call it (ServeSSO, ServeIDPInitiated)
// guarantee this.
// This has been adapted from crewjam's DefaultAssertionMaker.
func (s *Service) MakeAssertion(req *saml.IdpAuthnRequest, session *saml.Session) error {
	// None of these are expected to be missing when this has been called through
	// ServeSSO or ServeIDPInitiated, but this will avoid nil pointer panics.
	if req.SPSSODescriptor == nil {
		return trace.BadParameter("SPSSO descriptor is missing")
	}
	if req.ACSEndpoint == nil {
		return trace.BadParameter("ACS endpoint is missing")
	}
	if req.IDP == nil {
		return trace.BadParameter("IDP is missing")
	}
	if req.ServiceProviderMetadata == nil {
		return trace.BadParameter("service provider metadata is missing")
	}
	if req.HTTPRequest == nil {
		return trace.BadParameter("HTTP request is missing")
	}

	// Grab the first default
	var attributeConsumingService *saml.AttributeConsumingService
	for i, acs := range req.SPSSODescriptor.AttributeConsumingServices {
		// Set to the first element.
		if attributeConsumingService == nil {
			attributeConsumingService = &req.SPSSODescriptor.AttributeConsumingServices[i]
		}

		// If there is a default found, use that instead.
		if acs.IsDefault != nil && *acs.IsDefault {
			attributeConsumingService = &req.SPSSODescriptor.AttributeConsumingServices[i]
			break
		}
	}

	// If we can't find any, just use an empty service.
	if attributeConsumingService == nil {
		attributeConsumingService = &saml.AttributeConsumingService{}
	}

	attributes, err := s.buildAttributes(req, session, attributeConsumingService)
	if err != nil {
		return trace.Wrap(err)
	}

	// allow for some clock skew in the validity period using the
	// issuer's apparent clock.
	notBefore := req.Now.Add(-1 * saml.MaxClockSkew)
	notOnOrAfterAfter := req.Now.Add(saml.MaxIssueDelay)
	if notBefore.Before(req.Request.IssueInstant) {
		notBefore = req.Request.IssueInstant
		notOnOrAfterAfter = notBefore.Add(saml.MaxIssueDelay)
	}

	nameIDFormat := getNameIDFormatFromSPSSODescriptor(req.ServiceProviderMetadata.SPSSODescriptors)
	if session.NameIDFormat != "" {
		nameIDFormat = session.NameIDFormat
	}

	randomHex, err := utils.CryptoRandomHex(20)
	if err != nil {
		return trace.Wrap(err)
	}

	req.Assertion = &saml.Assertion{
		ID:           fmt.Sprintf("id-%s", randomHex),
		IssueInstant: s.clock.Now().UTC(),
		Version:      "2.0",
		Issuer: saml.Issuer{
			Format: types.SAMLEntityNameIDFormat,
			Value:  req.IDP.Metadata().EntityID,
		},
		Subject: &saml.Subject{
			NameID: &saml.NameID{
				Format:          nameIDFormat,
				NameQualifier:   req.IDP.Metadata().EntityID,
				SPNameQualifier: req.ServiceProviderMetadata.EntityID,
				Value:           session.NameID,
			},
			SubjectConfirmations: []saml.SubjectConfirmation{
				{
					Method: types.SAMLBearerMethod,
					SubjectConfirmationData: &saml.SubjectConfirmationData{
						Address:      req.HTTPRequest.RemoteAddr,
						InResponseTo: req.Request.ID,
						NotOnOrAfter: req.Now.Add(saml.MaxIssueDelay),
						Recipient:    req.ACSEndpoint.Location,
					},
				},
			},
		},
		Conditions: &saml.Conditions{
			NotBefore:    notBefore,
			NotOnOrAfter: notOnOrAfterAfter,
			AudienceRestrictions: []saml.AudienceRestriction{
				{Audience: saml.Audience{Value: req.ServiceProviderMetadata.EntityID}},
			},
		},
		AuthnStatements: []saml.AuthnStatement{
			{
				AuthnInstant: session.CreateTime,
				SessionIndex: session.Index,
				SubjectLocality: &saml.SubjectLocality{
					Address: req.HTTPRequest.RemoteAddr,
				},
				AuthnContext: saml.AuthnContext{
					AuthnContextClassRef: &saml.AuthnContextClassRef{
						Value: types.SAMLAuthnContextPublicKeyX509ClassRef,
					},
				},
			},
		},
		AttributeStatements: []saml.AttributeStatement{
			{
				Attributes: attributes,
			},
		},
	}

	// To sign the response, we'll need to send the assertion and the service provider SSO descriptor
	// to the auth server for signing.
	doc := etree.NewDocument()
	doc.SetRoot(req.Assertion.Element())
	assertionBytes, err := doc.WriteToBytes()
	if err != nil {
		return trace.Wrap(err)
	}

	spssoDescriptor, err := xml.Marshal(req.SPSSODescriptor)
	if err != nil {
		return trace.Wrap(err)
	}

	// Get the MFA response from the query parameter `MFAResponse`. The MFA
	// response will later be used to authenticate the user.
	encodedMFAResp := req.HTTPRequest.URL.Query().Get(MFAResponse.String())

	// If `MFAResponse` is not available, it might mean we're dealing with an older client,
	// so try to get the MFA response from the WebAuthn query parameter.
	// TODO(cthach): DELETE IN v20.0.0 WebAuthn query parameter.
	if encodedMFAResp == "" {
		encodedMFAResp = req.HTTPRequest.URL.Query().Get(Webauthn.String())
	}

	var mfaProtoResponse *proto.MFAAuthenticateResponse
	if encodedMFAResp != "" {
		decodedMFAResp, err := base64.RawURLEncoding.DecodeString(encodedMFAResp)
		if err != nil {
			return trace.Wrap(err)
		}

		mfaProtoResponse, err = client.ParseMFAChallengeResponse(decodedMFAResp)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Make the SAML IdP response on the auth server.
	ctx := req.HTTPRequest.Context()

	resp, err := s.client.SAMLIdPClient().ProcessSAMLIdPRequest(ctx, &samlidppb.ProcessSAMLIdPRequestRequest{
		Assertion:                    assertionBytes,
		Destination:                  req.ACSEndpoint.Location,
		RequestId:                    req.Request.ID,
		RequestTime:                  timestamppb.New(req.Now),
		MetadataUrl:                  s.metadataURL.String(),
		SignatureMethod:              s.signatureMethod,
		ServiceProviderSsoDescriptor: spssoDescriptor,
		MfaResponse:                  mfaProtoResponse,
	})
	if err != nil {
		return trail.FromGRPC(err)
	}

	// Parse out the SAML response from the response and assign it to the
	// request.
	respDoc := etree.NewDocument()
	if err := respDoc.ReadFromBytes(resp.Response); err != nil {
		return trace.Wrap(err)
	}

	// Setting this will prevent the local identity provider from signing
	// the response using its local certificate and key.
	req.ResponseEl = respDoc.Root()

	// Emit success event here since the assertion is signed.
	// The only step remaining now is to respond with HTML post form.
	s.emitAuthAttemptEvent(ctx, session.UserName, req.ServiceProviderMetadata.EntityID, "", nil)

	return nil
}

// buildAttributes assembles the full set of SAML attributes for an assertion.
func (s *Service) buildAttributes(req *saml.IdpAuthnRequest, session *saml.Session, acs *saml.AttributeConsumingService) ([]saml.Attribute, error) {
	var attributes []saml.Attribute

	// Collect attributes requested by the SP metadata.
	for _, ra := range acs.RequestedAttributes {
		switch ra.NameFormat {
		case types.SAMLBasicNameFormat, types.SAMLUnspecifiedNameFormat:
			var value string
			switch ra.Name {
			case "email", "emailaddress":
				value = session.UserEmail
			case "name", "fullname", "cn", "commonname":
				value = session.UserCommonName
			case "givenname", "firstname":
				value = session.UserGivenName
			case "surname", "lastname", "familyname":
				value = session.UserSurname
			case "uid", "user", "userid":
				value = session.UserName
			}
			attributes = addAttributeWithFormat(attributes, ra.FriendlyName, ra.Name, ra.NameFormat, value)
		}
	}

	// Custom mappings take precedence over default attributes, so we add defaults first and then custom attributes will overwrite
	// any defaults with the same name.
	customNames := s.customAttributeNames(req)
	addDefault := func(friendlyName, name string, values ...string) {
		// custom attribute mapping takes precedence over default attribute, even if it evaluates to an empty set.
		if !customNames[name] {
			attributes = addAttribute(attributes, friendlyName, name, values...)
		}
	}
	addDefault(types.SAMLUIDFriendlyName, types.SAMLUIDName, session.UserName)
	addDefault(types.SAMLEduPersonAffiliationFriendlyName, types.SAMLEduPersonAffiliationName, session.Groups...)
	addDefault("", types.SAMLSubjectIDName, session.SubjectID)

	custom, err := s.evaluateCustomAttributes(req, session)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	attributes = append(attributes, custom...)

	return attributes, nil
}

// evaluateCustomAttributes processes custom attribute mappings from the
// Teleport resources description.
func (s *Service) evaluateCustomAttributes(req *saml.IdpAuthnRequest, session *saml.Session) ([]saml.Attribute, error) {
	var out []saml.Attribute
	_, teleportSPSSODescriptor := local.GetTeleportSPSSODescriptor(req.ServiceProviderMetadata.SPSSODescriptors)
	attrs := attributesToMappableUserSpec(session.CustomAttributes)
	attrs.Username = session.UserName

	for _, acs := range teleportSPSSODescriptor.AttributeConsumingServices {
		evaluated, err := attribute.EvaluateAttributes(acs.RequestedAttributes, attrs)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, evaluated...)
	}
	return out, nil
}

// addAttribute will add an attribute to the given slice if the number of values is non-zero. If there is one element,
// the attribute will not be added if it is empty.
func addAttribute(attributes []saml.Attribute, friendlyName, name string, values ...string) []saml.Attribute {
	return addAttributeWithFormat(attributes, friendlyName, name, types.SAMLURINameFormat, values...)
}

// addAttributeWithFormat has the same behavior as addAttribute but allows for the user to specify the name format.
func addAttributeWithFormat(attributes []saml.Attribute, friendlyName, name, format string, values ...string) []saml.Attribute {
	numValues := len(values)
	if numValues == 0 {
		return attributes
	}
	if numValues == 1 && values[0] == "" {
		return attributes
	}

	return append(attributes, attribute.New(friendlyName, name, format, values...))
}

// customAttributeNames returns the set of attribute names defined by custom
// attribute mappings in the service provider's Teleport-specific descriptor.
func (s *Service) customAttributeNames(req *saml.IdpAuthnRequest) map[string]bool {
	_, teleportSPSSODescriptor := local.GetTeleportSPSSODescriptor(req.ServiceProviderMetadata.SPSSODescriptors)
	names := make(map[string]bool)
	for _, acs := range teleportSPSSODescriptor.AttributeConsumingServices {
		for _, ra := range acs.RequestedAttributes {
			names[ra.Name] = true
		}
	}
	return names
}

// attributesToMappableUserSpec unpacks saml session custom attributes to samlMappableUserSpec.
func attributesToMappableUserSpec(customAttrs []saml.Attribute) attribute.SAMLMappableUserSpec {
	var mappableAttrs attribute.SAMLMappableUserSpec
	mappableAttrs.Traits = make(map[string][]string, 0)
	for _, attr := range customAttrs {
		switch attr.Name {
		case "roles":
			mappableAttrs.Roles = samlAttributeValuesToSlice(attr.Values)
		default:
			mappableAttrs.Traits[attr.Name] = samlAttributeValuesToSlice(attr.Values)
		}
	}
	return mappableAttrs
}

func samlAttributeValuesToSlice(attrVals []saml.AttributeValue) []string {
	attrValues := make([]string, 0, len(attrVals))
	for _, values := range attrVals {
		attrValues = append(attrValues, values.Value)
	}
	return attrValues
}

func getNameIDFormatFromSPSSODescriptor(spSSODescriptors []saml.SPSSODescriptor) string {
	for _, spSSODescriptor := range spSSODescriptors {
		for _, acs := range spSSODescriptor.AssertionConsumerServices {
			if acs.Binding == "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" {
				if len(spSSODescriptor.NameIDFormats) == 0 {
					// Empty Name ID element isn't generally expected but SP may not specify
					// the format and expect IdP to respond with IdP preferred NameID format.
					// It's also best for us to rule out malformed entity descriptor and save
					// us from index out of range panic below.
					return types.SAMLUnspecifiedNameIDFormat
				} else {
					// Only one Name ID element is expected. But SAML specification allows SP
					// to advertise multiple supported Name ID formats. We will pick the first
					// one available.
					// TODO(sshah): investigate if we would like to have a preferred Name ID format.
					// If we have one, instead of picking the first element, we can iterate
					// through all values and elect the format we prefer.
					return nameIDFormatStringFromSAMLNameIDType(spSSODescriptor.NameIDFormats[0])
				}
			}
		}
	}
	return ""
}

func nameIDFormatStringFromSAMLNameIDType(nameIDType saml.NameIDFormat) string {
	switch nameIDType {
	case types.SAMLUnspecifiedNameIDFormat,
		types.SAMLEmailAddressNameIDFormat,
		types.SAMLWindowsDomainQualifiedNameNameIDFormat,
		types.SAMLKerberosPrincipalNameNameNameIDFormat,
		types.SAMLX509SubjectNameNameIDFormat,
		types.SAMLEntityNameIDFormat,
		types.SAMLPersistentNameIDFormat,
		types.SAMLTransientNameIDFormat:
		return string(nameIDType)
	default:
		return types.SAMLUnspecifiedNameIDFormat
	}
}
