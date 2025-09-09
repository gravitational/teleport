package scimsdk

import (
	"encoding/json"
	"maps"

	"github.com/gravitational/trace"
)

type Name struct {
	FamilyName string `json:"familyName,omitempty"`
	GivenName  string `json:"givenName,omitempty"`
}

// User represents a SCIM User resource.
type User struct {
	// ID is a unique identifier for a User as defined by the Service Provider.
	ID string `json:"id,omitempty"`
	// ExternalID is an identifier for the User as defined by the User's identity provider.
	ExternalID string `json:"externalId,omitempty"`
	// Meta is a complex attribute containing resource metadata.
	Meta *Metadata `json:"meta,omitempty"`
	// Schemas is a list of URIs that are used to indicate the namespaces of the SCIM schemas used for the representation of a resource.
	Schemas []string `json:"schemas,omitempty"`
	// UserName is the unique identifier for the User, typically used by the user to directly authenticate to the service provider.
	UserName string `json:"userName,omitempty"`
	// Name is the components of the User's real name.
	Name *Name `json:"name,omitempty"`
	// DisplayName is the name of the User, suitable for display to end-users.
	DisplayName string `json:"displayName,omitempty"`
	// Active is a boolean value indicating the User's administrative status.
	Active bool `json:"active,omitempty"`
	// WARNING: If you want to store additional fields in the struct, please extend knownStructFields
	// to not duplicate the fields in the Attributes map.

	// Attributes is a map of all fields that are not part of the User struct
	Attributes AttributeSet `json:"-"`
}

var knownStructFields = []string{
	"id", "externalId", "meta", "schemas", "userName", "name", "displayName", "active",
}

// ListUserResponse represents a SCIM User list response.
type ListUserResponse struct {
	// Schemas is a list of URIs that are used to indicate the namespaces of the SCIM schemas used for the representation of a resource.
	Schemas []string `json:"schemas"`
	// TotalResults is the total number of results returned by the list or query operation.
	TotalResults int32 `json:"totalResults"`
	// StartIndex is the 1-based index of the first result in the current set of list results.
	StartIndex int32 `json:"startIndex"`
	// ItemsPerPage is the number of resources returned in a list response page.
	ItemsPerPage int32 `json:"itemsPerPage"`
	// Users is a list of User resources.
	Users []*User `json:"Resources"`
}

// MarshalJSON is Custom MarshalJSON method allowing to marshal Attributes into top level json  props.
func (r *User) MarshalJSON() ([]byte, error) {
	type Alias User
	aux := &struct{ *Alias }{Alias: (*Alias)(r)}
	base, err := json.Marshal(aux)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	attributes, err := json.Marshal(r.Attributes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var baseMap map[string]any
	if err := json.Unmarshal(base, &baseMap); err != nil {
		return nil, trace.Wrap(err)
	}

	var attrMap map[string]any
	if err := json.Unmarshal(attributes, &attrMap); err != nil {
		return nil, err
	}
	maps.Copy(baseMap, attrMap)
	return json.Marshal(baseMap)
}

// UnmarshalJSON is Custom UnmarshalJSON method to extract Attributes into the map
// and merge them with the rest of the struct fields
func (r *User) UnmarshalJSON(data []byte) error {
	type Alias User
	aux := &struct{ *Alias }{Alias: (*Alias)(r)}
	if err := json.Unmarshal(data, aux); err != nil {
		return trace.Wrap(err)
	}

	var rawMap map[string]any
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return trace.Wrap(err)
	}

	// Known fields are extracted into the struct, the rest are stored in Attributes
	// TODO(smallinsky) Do this dynamically via reflection based on struct json tags
	for _, field := range knownStructFields {
		delete(rawMap, field)
	}

	r.Attributes = rawMap
	return nil
}
