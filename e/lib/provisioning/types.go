package provisioning

import "github.com/gravitational/teleport/api/types"

// ExternalID represents the ID of a principal in the downstream system
type ExternalID string

// String returns string value of ExternalID.
func (e ExternalID) String() string {
	return string(e)
}

const (
	// ExternalIDLabel is used to contain external ID of an Access List member
	// whose account does not exist in Teleport.
	ExternalIDLabel ExternalID = types.TeleportInternalLabelPrefix + "external-id"
)
