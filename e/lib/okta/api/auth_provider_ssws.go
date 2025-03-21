package oktaapi

import (
	"github.com/okta/okta-sdk-golang/v2/okta"
)

// NewSSWSAuthProvider returns  a new  Okta client SSWS credential provider.
func NewSSWSAuthProvider(token string) *SSWSAuthProvider {
	return &SSWSAuthProvider{
		SSWSToken: token,
	}
}

// SSWSAuthProvider is an Okta client credential provider that will allow to authenticate
// using SSWS token: https://developer.okta.com/docs/guides/create-an-api-token/main/#okta-api-tokens
type SSWSAuthProvider struct {
	SSWSToken string
}

// GetAuthOptions returns the Okta client configuration options.
func (a *SSWSAuthProvider) GetAuthOptions() []okta.ConfigSetter {
	return []okta.ConfigSetter{
		okta.WithToken(a.SSWSToken),
	}
}
