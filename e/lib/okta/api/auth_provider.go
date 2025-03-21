package oktaapi

import (
	"github.com/okta/okta-sdk-golang/v2/okta"
)

// AuthProvider is an interface for providing Okta client configuration options.
type AuthProvider interface {
	// GetAuthOptions returns the Okta auth provider configuration options.
	GetAuthOptions() []okta.ConfigSetter
}
