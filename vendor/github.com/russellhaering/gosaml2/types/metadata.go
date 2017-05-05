package types

import (
	"encoding/xml"

	dsigtypes "github.com/russellhaering/goxmldsig/types"
)

type EntityDescriptor struct {
	XMLName xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:metadata EntityDescriptor"`
	// SAML 2.0 8.3.6 Entity Identifier could be used to represent issuer
	EntityID         string           `xml:"entityID,attr"`
	IDPSSODescriptor IDPSSODescriptor `xml:"IDPSSODescriptor"`
}

type IDPSSODescriptor struct {
	XMLName             xml.Name            `xml:"urn:oasis:names:tc:SAML:2.0:metadata IDPSSODescriptor"`
	KeyDescriptors      []KeyDescriptor     `xml:"KeyDescriptor"`
	NameIDFormats       []NameIDFormat      `xml:"NameIDFormat"`
	SingleSignOnService SingleSignOnService `xml:"SingleSignOnService"`
	Attributes          []Attribute         `xml:"Attribute"`
}

type KeyDescriptor struct {
	XMLName xml.Name          `xml:"urn:oasis:names:tc:SAML:2.0:metadata KeyDescriptor"`
	Use     string            `xml:"use,attr"`
	KeyInfo dsigtypes.KeyInfo `xml:"KeyInfo"`
}

type NameIDFormat struct {
	XMLName xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:metadata NameIDFormat"`
	Value   string   `xml:",chardata"`
}

type SingleSignOnService struct {
	XMLName  xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:metadata SingleSignOnService"`
	Binding  string   `xml:"Binding,attr"`
	Location string   `xml:"Location,attr"`
}
