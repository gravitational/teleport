package saml

import (
	"encoding/xml"
	"time"
)

// HTTPPostBinding is the official URN for the HTTP-POST binding (transport)
const HTTPPostBinding = "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"

// HTTPRedirectBinding is the official URN for the HTTP-Redirect binding (transport)
const HTTPRedirectBinding = "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect"

// EntitiesDescriptor represents the SAML object of the same name.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-metadata-2.0-os.pdf section 2.3.1
type EntitiesDescriptor struct {
	XMLName          xml.Name    `xml:"urn:oasis:names:tc:SAML:2.0:metadata EntitiesDescriptor"`
	EntityDescriptor []*Metadata `xml:"urn:oasis:names:tc:SAML:2.0:metadata EntityDescriptor"`
}

// Metadata represents the SAML EntityDescriptor object.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-metadata-2.0-os.pdf section 2.3.2
type Metadata struct {
	XMLName          xml.Name          `xml:"urn:oasis:names:tc:SAML:2.0:metadata EntityDescriptor"`
	ValidUntil       time.Time         `xml:"validUntil,attr"`
	CacheDuration    time.Duration     `xml:"cacheDuration,attr,omitempty"`
	EntityID         string            `xml:"entityID,attr"`
	SPSSODescriptor  *SPSSODescriptor  `xml:"SPSSODescriptor"`
	IDPSSODescriptor *IDPSSODescriptor `xml:"IDPSSODescriptor"`
}

func (m *Metadata) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	type Alias Metadata
	aux := &struct {
		ValidUntil RelaxedTime `xml:"validUntil,attr"`
		*Alias
	}{
		ValidUntil: RelaxedTime(m.ValidUntil),
		Alias:      (*Alias)(m),
	}
	return e.Encode(aux)
}

func (m *Metadata) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type Alias Metadata
	aux := &struct {
		ValidUntil RelaxedTime `xml:"validUntil,attr"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}
	if err := d.DecodeElement(&aux, &start); err != nil {
		return err
	}
	m.ValidUntil = time.Time(aux.ValidUntil)
	return nil
}

// KeyDescriptor represents the XMLSEC object of the same name
type KeyDescriptor struct {
	Use               string             `xml:"use,attr"`
	KeyInfo           KeyInfo            `xml:"http://www.w3.org/2000/09/xmldsig# KeyInfo"`
	EncryptionMethods []EncryptionMethod `xml:"EncryptionMethod"`
}

// EncryptionMethod represents the XMLSEC object of the same name
type EncryptionMethod struct {
	Algorithm string `xml:"Algorithm,attr"`
}

// KeyInfo represents the XMLSEC object of the same name
type KeyInfo struct {
	XMLName     xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# KeyInfo"`
	Certificate string   `xml:"X509Data>X509Certificate"`
}

// Endpoint represents the SAML EndpointType object.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-metadata-2.0-os.pdf section 2.2.2
type Endpoint struct {
	Binding          string `xml:"Binding,attr"`
	Location         string `xml:"Location,attr"`
	ResponseLocation string `xml:"ResponseLocation,attr,omitempty"`
}

// IndexedEndpoint represents the SAML IndexedEndpointType object.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-metadata-2.0-os.pdf section 2.2.3
type IndexedEndpoint struct {
	Binding  string `xml:"Binding,attr"`
	Location string `xml:"Location,attr"`
	Index    int    `xml:"index,attr"`
}

// SPSSODescriptor represents the SAML SPSSODescriptorType object.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-metadata-2.0-os.pdf section 2.4.2
type SPSSODescriptor struct {
	XMLName                    xml.Name          `xml:"urn:oasis:names:tc:SAML:2.0:metadata SPSSODescriptor"`
	AuthnRequestsSigned        bool              `xml:",attr"`
	WantAssertionsSigned       bool              `xml:",attr"`
	ProtocolSupportEnumeration string            `xml:"protocolSupportEnumeration,attr"`
	KeyDescriptor              []KeyDescriptor   `xml:"KeyDescriptor"`
	ArtifactResolutionService  []IndexedEndpoint `xml:"ArtifactResolutionService"`
	SingleLogoutService        []Endpoint        `xml:"SingleLogoutService"`
	ManageNameIDService        []Endpoint
	NameIDFormat               []string          `xml:"NameIDFormat"`
	AssertionConsumerService   []IndexedEndpoint `xml:"AssertionConsumerService"`
	AttributeConsumingService  []interface{}
}

// IDPSSODescriptor represents the SAML IDPSSODescriptorType object.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-metadata-2.0-os.pdf section 2.4.3
type IDPSSODescriptor struct {
	XMLName                    xml.Name        `xml:"urn:oasis:names:tc:SAML:2.0:metadata IDPSSODescriptor"`
	WantAuthnRequestsSigned    bool            `xml:",attr"`
	ProtocolSupportEnumeration string          `xml:"protocolSupportEnumeration,attr"`
	KeyDescriptor              []KeyDescriptor `xml:"KeyDescriptor"`
	NameIDFormat               []string        `xml:"NameIDFormat"`
	SingleSignOnService        []Endpoint      `xml:"SingleSignOnService"`
}
