package attribute

import (
	"github.com/crewjam/saml"

	"github.com/gravitational/teleport/api/types"
)

// New creates a new saml.Attribute.
func New(friendlyName, name, nameFormat string, values ...string) saml.Attribute {
	// Create the list of attribute values to add to the attribute.
	attributeValues := make([]saml.AttributeValue, len(values))
	for i, value := range values {
		attributeValues[i] = saml.AttributeValue{
			Type:  types.SAMLStringType,
			Value: value,
		}
	}

	return saml.Attribute{
		FriendlyName: friendlyName,
		Name:         name,
		NameFormat:   nameFormat,
		Values:       attributeValues,
	}
}

// NewWithSAMLURINameFormat returns a slice of saml.Attribute with types.SAMLURINameFormat as the NameFormat type.
func NewWithSAMLURINameFormat(attributes []saml.Attribute, friendlyName, name string, values ...string) []saml.Attribute {
	return NewWithFormat(attributes, friendlyName, name, types.SAMLURINameFormat, values...)
}

// NewWithFormat returns a slice of saml.Attribute with the given nameFormat type.
// It expects attributes to have more than one element and if the length is exactly one,
// it expects the value of the first element to be non-empty string.
func NewWithFormat(attributes []saml.Attribute, friendlyName, name, nameFormat string, values ...string) []saml.Attribute {
	if len(values) == 0 || (len(values) == 1 && values[0] == "") {
		return attributes
	}

	return append(attributes, New(friendlyName, name, nameFormat, values...))
}
