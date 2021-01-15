package types

import (
	"time"

	"github.com/gravitational/trace"
)

// GenerateAppTokenRequest are the parameters used to generate an application token.
type GenerateAppTokenRequest struct {
	// Username is the Teleport identity.
	Username string

	// Roles are the roles assigned to the user within Teleport.
	Roles []string

	// Expiry is time to live for the token.
	Expires time.Time

	// URI is the URI of the recipient application.
	URI string
}

// Check validates the request.
func (p *GenerateAppTokenRequest) Check() error {
	if p.Username == "" {
		return trace.BadParameter("username missing")
	}
	if len(p.Roles) == 0 {
		return trace.BadParameter("roles missing")
	}
	if p.Expires.IsZero() {
		return trace.BadParameter("expires missing")
	}
	if p.URI == "" {
		return trace.BadParameter("uri missing")
	}
	return nil
}
