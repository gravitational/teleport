package saml

import (
	"encoding/xml"
	"fmt"

	"github.com/beevik/etree"
	"github.com/crewjam/saml"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"google.golang.org/protobuf/types/known/timestamppb"

	samlidppb "github.com/gravitational/teleport/api/gen/proto/go/teleport/samlidp/v1"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	stringType = "xs:string"

	// The following formats are all defined in the SAML 2.0 Core OS Standard.
	// https://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
	uriNameFormat         = "urn:oasis:names:tc:SAML:2.0:attrname-format:uri"
	basicNameFormat       = "urn:oasis:names:tc:SAML:2.0:attrname-format:basic"
	unspecifiedNameFormat = "urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified"
	transientFormat       = "urn:oasis:names:tc:SAML:2.0:nameid-format:transient"
	entityFormat          = "urn:oasis:names:tc:SAML:2.0:nameid-format:entity"

	// The following class refs are defined in the SAML 2.0 Authentication Context Standard.
	// https://docs.oasis-open.org/security/saml/v2.0/saml-authn-context-2.0-os.pdf
	authnContextClassRef = "urn:oasis:names:tc:SAML:2.0:ac:classes:X509"

	// The following methods are defined in the SAML 2.0 Technical Overview.
	// http://docs.oasis-open.org/security/saml/Post2.0/sstc-saml-tech-overview-2.0-cd-02.pdf
	bearerMethod = "urn:oasis:names:tc:SAML:2.0:cm:bearer"

	// The following are object identifiers. A link to the entry for these in the OID-Info DB is
	// provided if applicable.

	// http://www.oid-info.com/cgi-bin/display?oid=urn%3Aoid%3A0.9.2342.19200300.100.1.1&a=display
	uidFriendlyName = "uid"
	uidName         = "urn:oid:0.9.2342.19200300.100.1.1"

	// http://www.oid-info.com/cgi-bin/display?oid=urn%3Aoid%3A1.3.6.1.4.1.5923.1.1.1.1&a=display
	eduPersonAffiliationFriendlyName = "eduPersonAffiliation"
	eduPersonAffiliationName         = "urn:oid:1.3.6.1.4.1.5923.1.1.1.1"

	// The following is the general purpose subject identifier defined in SAML 2.0 Subject Identifier Attributes.
	// http://docs.oasis-open.org/security/saml-subject-id-attr/v1.0/csprd03/saml-subject-id-attr-v1.0-csprd03.pdf
	subjectIDFriendlyName = ""
	subjectIDName         = "urn:oasis:names:tc:SAML:attribute:subject-id"
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

	var attributes []saml.Attribute
	// Push in any requested attributes.
	for _, requestedAttribute := range attributeConsumingService.RequestedAttributes {
		switch requestedAttribute.NameFormat {
		case basicNameFormat, unspecifiedNameFormat:
			var value string
			switch requestedAttribute.Name {
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
			attributes = addAttributeWithFormat(attributes, requestedAttribute.FriendlyName, requestedAttribute.Name, requestedAttribute.NameFormat, value)
		}
	}

	attributes = addAttribute(attributes, uidFriendlyName, uidName, session.UserName)
	attributes = addAttribute(attributes, eduPersonAffiliationFriendlyName, eduPersonAffiliationName, session.Groups...)
	attributes = addAttribute(attributes, subjectIDFriendlyName, subjectIDName, session.SubjectID)

	attributes = append(attributes, session.CustomAttributes...)

	// allow for some clock skew in the validity period using the
	// issuer's apparent clock.
	notBefore := req.Now.Add(-1 * saml.MaxClockSkew)
	notOnOrAfterAfter := req.Now.Add(saml.MaxIssueDelay)
	if notBefore.Before(req.Request.IssueInstant) {
		notBefore = req.Request.IssueInstant
		notOnOrAfterAfter = notBefore.Add(saml.MaxIssueDelay)
	}

	// Default to using the transient format.
	nameIDFormat := transientFormat

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
			Format: entityFormat,
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
					Method: bearerMethod,
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
						Value: authnContextClassRef,
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

	return nil
}

// addAttribute will add an attribute to the given slice if the number of values is non-zero. If there is one element,
// the attribute will not be added if it is empty.
func addAttribute(attributes []saml.Attribute, friendlyName, name string, values ...string) []saml.Attribute {
	return addAttributeWithFormat(attributes, friendlyName, name, uriNameFormat, values...)
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

	return append(attributes, attribute(friendlyName, name, format, values...))
}

// attribute creates a new saml.Attribute.
func attribute(friendlyName, name, format string, values ...string) saml.Attribute {
	// Create the list of attribute values to add to the attribute.
	attributeValues := make([]saml.AttributeValue, len(values))
	for i, value := range values {
		attributeValues[i] = saml.AttributeValue{
			Type:  stringType,
			Value: value,
		}
	}

	return saml.Attribute{
		FriendlyName: friendlyName,
		Name:         name,
		NameFormat:   format,
		Values:       attributeValues,
	}
}
