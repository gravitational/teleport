package saml

import (
	"encoding/xml"
	"time"

	"github.com/crewjam/go-xmlsec"
)

// AuthnRequest represents the SAML object of the same name, a request from a service provider
// to authenticate a user.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
type AuthnRequest struct {
	XMLName                     xml.Name  `xml:"urn:oasis:names:tc:SAML:2.0:protocol AuthnRequest"`
	AssertionConsumerServiceURL string    `xml:",attr"`
	Destination                 string    `xml:",attr"`
	ID                          string    `xml:",attr"`
	IssueInstant                time.Time `xml:",attr"`

	// Protocol binding is a URI reference that identifies a SAML protocol binding to be used when returning
	// the <Response> message. See [SAMLBind] for more information about protocol bindings and URI references
	// defined for them. This attribute is mutually exclusive with the AssertionConsumerServiceIndex attribute
	// and is typically accompanied by the AssertionConsumerServiceURL attribute.
	ProtocolBinding string `xml:",attr"`

	Version      string            `xml:",attr"`
	Issuer       Issuer            `xml:"urn:oasis:names:tc:SAML:2.0:assertion Issuer"`
	Signature    *xmlsec.Signature `xml:"http://www.w3.org/2000/09/xmldsig# Signature"`
	NameIDPolicy NameIDPolicy      `xml:"urn:oasis:names:tc:SAML:2.0:protocol NameIDPolicy"`
}

func (a *AuthnRequest) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	type Alias AuthnRequest
	aux := &struct {
		IssueInstant RelaxedTime `xml:",attr"`
		*Alias
	}{
		IssueInstant: RelaxedTime(a.IssueInstant),
		Alias:        (*Alias)(a),
	}
	return e.Encode(aux)
}

func (a *AuthnRequest) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type Alias AuthnRequest
	aux := &struct {
		IssueInstant RelaxedTime `xml:",attr"`
		*Alias
	}{
		Alias: (*Alias)(a),
	}
	if err := d.DecodeElement(&aux, &start); err != nil {
		return err
	}
	a.IssueInstant = time.Time(aux.IssueInstant)
	return nil
}

// Issuer represents the SAML object of the same name.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
type Issuer struct {
	XMLName xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion Issuer"`
	Format  string   `xml:",attr"`
	Value   string   `xml:",chardata"`
}

// NameIDPolicy represents the SAML object of the same name.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
type NameIDPolicy struct {
	XMLName     xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:protocol NameIDPolicy"`
	AllowCreate bool     `xml:",attr"`
	Format      string   `xml:",chardata"`
}

// Response represents the SAML object of the same name.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
type Response struct {
	XMLName            xml.Name  `xml:"urn:oasis:names:tc:SAML:2.0:protocol Response"`
	Destination        string    `xml:",attr"`
	ID                 string    `xml:",attr"`
	InResponseTo       string    `xml:",attr"`
	IssueInstant       time.Time `xml:",attr"`
	Version            string    `xml:",attr"`
	Issuer             *Issuer   `xml:"urn:oasis:names:tc:SAML:2.0:assertion Issuer"`
	Status             *Status   `xml:"urn:oasis:names:tc:SAML:2.0:protocol Status"`
	EncryptedAssertion *EncryptedAssertion
	Assertion          *Assertion `xml:"urn:oasis:names:tc:SAML:2.0:assertion Assertion"`
}

func (r *Response) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	type Alias Response
	aux := &struct {
		IssueInstant RelaxedTime `xml:",attr"`
		*Alias
	}{
		IssueInstant: RelaxedTime(r.IssueInstant),
		Alias:        (*Alias)(r),
	}
	return e.Encode(aux)
}

func (r *Response) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type Alias Response
	aux := &struct {
		IssueInstant RelaxedTime `xml:",attr"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := d.DecodeElement(&aux, &start); err != nil {
		return err
	}
	r.IssueInstant = time.Time(aux.IssueInstant)
	return nil
}

// Status represents the SAML object of the same name.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
type Status struct {
	XMLName    xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:protocol Status"`
	StatusCode StatusCode
}

// StatusCode represents the SAML object of the same name.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
type StatusCode struct {
	XMLName xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:protocol StatusCode"`
	Value   string   `xml:",attr"`
}

// StatusSuccess is the value of a StatusCode element when the authentication succeeds.
// (nominally a constant, except for testing)
var StatusSuccess = "urn:oasis:names:tc:SAML:2.0:status:Success"

// EncryptedAssertion represents the SAML object of the same name.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
type EncryptedAssertion struct {
	Assertion     *Assertion
	EncryptedData []byte `xml:",innerxml"`
}

// Assertion represents the SAML object of the same name.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
type Assertion struct {
	XMLName            xml.Name  `xml:"urn:oasis:names:tc:SAML:2.0:assertion Assertion"`
	ID                 string    `xml:",attr"`
	IssueInstant       time.Time `xml:",attr"`
	Version            string    `xml:",attr"`
	Issuer             *Issuer   `xml:"urn:oasis:names:tc:SAML:2.0:assertion Issuer"`
	Signature          *xmlsec.Signature
	Subject            *Subject
	Conditions         *Conditions
	AuthnStatement     *AuthnStatement
	AttributeStatement *AttributeStatement
}

func (a *Assertion) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	type Alias Assertion
	aux := &struct {
		IssueInstant RelaxedTime `xml:",attr"`
		*Alias
	}{
		IssueInstant: RelaxedTime(a.IssueInstant),
		Alias:        (*Alias)(a),
	}
	return e.Encode(aux)
}

func (a *Assertion) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type Alias Assertion
	aux := &struct {
		IssueInstant RelaxedTime `xml:",attr"`
		*Alias
	}{
		Alias: (*Alias)(a),
	}
	if err := d.DecodeElement(&aux, &start); err != nil {
		return err
	}
	a.IssueInstant = time.Time(aux.IssueInstant)
	return nil
}

// Subject represents the SAML object of the same name.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
type Subject struct {
	XMLName             xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion Subject"`
	NameID              *NameID
	SubjectConfirmation *SubjectConfirmation
}

// NameID represents the SAML object of the same name.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
type NameID struct {
	Format          string `xml:",attr"`
	NameQualifier   string `xml:",attr"`
	SPNameQualifier string `xml:",attr"`
	Value           string `xml:",chardata"`
}

// SubjectConfirmation represents the SAML object of the same name.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
type SubjectConfirmation struct {
	Method                  string `xml:",attr"`
	SubjectConfirmationData SubjectConfirmationData
}

// SubjectConfirmationData represents the SAML object of the same name.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
type SubjectConfirmationData struct {
	Address      string    `xml:",attr"`
	InResponseTo string    `xml:",attr"`
	NotOnOrAfter time.Time `xml:",attr"`
	Recipient    string    `xml:",attr"`
}

func (s *SubjectConfirmationData) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	type Alias SubjectConfirmationData
	aux := &struct {
		NotOnOrAfter RelaxedTime `xml:",attr"`
		*Alias
	}{
		NotOnOrAfter: RelaxedTime(s.NotOnOrAfter),
		Alias:        (*Alias)(s),
	}
	return e.EncodeElement(aux, start)
}

func (s *SubjectConfirmationData) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type Alias SubjectConfirmationData
	aux := &struct {
		NotOnOrAfter RelaxedTime `xml:",attr"`
		*Alias
	}{
		Alias: (*Alias)(s),
	}
	if err := d.DecodeElement(&aux, &start); err != nil {
		return err
	}
	s.NotOnOrAfter = time.Time(aux.NotOnOrAfter)
	return nil
}

// Conditions represents the SAML object of the same name.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
type Conditions struct {
	NotBefore           time.Time `xml:",attr"`
	NotOnOrAfter        time.Time `xml:",attr"`
	AudienceRestriction *AudienceRestriction
}

func (c *Conditions) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	type Alias Conditions
	aux := &struct {
		NotBefore    RelaxedTime `xml:",attr"`
		NotOnOrAfter RelaxedTime `xml:",attr"`
		*Alias
	}{
		NotBefore:    RelaxedTime(c.NotBefore),
		NotOnOrAfter: RelaxedTime(c.NotOnOrAfter),
		Alias:        (*Alias)(c),
	}
	return e.EncodeElement(aux, start)
}

func (c *Conditions) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type Alias Conditions
	aux := &struct {
		NotBefore    RelaxedTime `xml:",attr"`
		NotOnOrAfter RelaxedTime `xml:",attr"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}
	if err := d.DecodeElement(&aux, &start); err != nil {
		return err
	}
	c.NotBefore = time.Time(aux.NotBefore)
	c.NotOnOrAfter = time.Time(aux.NotOnOrAfter)
	return nil
}

// AudienceRestriction represents the SAML object of the same name.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
type AudienceRestriction struct {
	Audience *Audience
}

// Audience represents the SAML object of the same name.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
type Audience struct {
	Value string `xml:",chardata"`
}

// AuthnStatement represents the SAML object of the same name.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
type AuthnStatement struct {
	AuthnInstant    time.Time `xml:",attr"`
	SessionIndex    string    `xml:",attr"`
	SubjectLocality SubjectLocality
	AuthnContext    AuthnContext
}

func (a *AuthnStatement) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	type Alias AuthnStatement
	aux := &struct {
		AuthnInstant RelaxedTime `xml:",attr"`
		*Alias
	}{
		AuthnInstant: RelaxedTime(a.AuthnInstant),
		Alias:        (*Alias)(a),
	}
	return e.EncodeElement(aux, start)
}

func (a *AuthnStatement) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type Alias AuthnStatement
	aux := &struct {
		AuthnInstant RelaxedTime `xml:",attr"`
		*Alias
	}{
		Alias: (*Alias)(a),
	}
	if err := d.DecodeElement(&aux, &start); err != nil {
		return err
	}
	a.AuthnInstant = time.Time(aux.AuthnInstant)
	return nil
}

// SubjectLocality represents the SAML object of the same name.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
type SubjectLocality struct {
	Address string `xml:",attr"`
}

// AuthnContext represents the SAML object of the same name.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
type AuthnContext struct {
	AuthnContextClassRef *AuthnContextClassRef
}

// AuthnContextClassRef represents the SAML object of the same name.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
type AuthnContextClassRef struct {
	Value string `xml:",chardata"`
}

// AttributeStatement represents the SAML object of the same name.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
type AttributeStatement struct {
	Attributes []Attribute `xml:"Attribute"`
}

// Attribute represents the SAML object of the same name.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
type Attribute struct {
	FriendlyName string           `xml:",attr"`
	Name         string           `xml:",attr"`
	NameFormat   string           `xml:",attr"`
	Values       []AttributeValue `xml:"AttributeValue"`
}

// AttributeValue represents the SAML object of the same name.
//
// See http://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf
type AttributeValue struct {
	Type   string `xml:"http://www.w3.org/2001/XMLSchema-instance type,attr"`
	Value  string `xml:",chardata"`
	NameID *NameID
}
