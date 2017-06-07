package ui

import (
	"github.com/gravitational/teleport/lib/defaults"
	teleservices "github.com/gravitational/teleport/lib/services"
)

// OIDConnector describes oidc connector properties
type OIDConnector struct {
	// ID is a provider name
	ID string `json:"id"`
	// Issuer URL is the endpoint of the provider, e.g. https://accounts.google.com
	IssuerURL string `json:"issuerUrl"`
	// ClientID is id for authentication client (in our case it's our Auth server)
	ClientID string `json:"clientId"`
	// ClientSecret is used to authenticate our client and should not be visible to end user
	ClientSecret string `json:"clientSecret"`
	// Should match the URL on Provider's side
	RedirectURL string `json:"redirectUrl"`
	// Display - Friendly name for this provider.
	Display string `json:"displayName"`
	// Scope is additional scopes set by provder
	Scope []string `json:"scope"`
	// ClaimsToRoles specifies dynamic mapping from claims to roles
	ClaimsToRoles []teleservices.ClaimMapping `json:"roleMapping,omitempty"`
}

// NewOIDConnector creates a new instance of OIConnector
func NewOIDConnector(teleConnector teleservices.OIDCConnector) OIDConnector {
	displayName := teleConnector.GetDisplay()
	if len(displayName) == 0 {
		displayName = teleConnector.GetName()
	}

	connector := OIDConnector{
		ID:            teleConnector.GetName(),
		IssuerURL:     teleConnector.GetIssuerURL(),
		ClientID:      teleConnector.GetClientID(),
		ClientSecret:  teleConnector.GetClientSecret(),
		RedirectURL:   teleConnector.GetRedirectURL(),
		Display:       displayName,
		Scope:         teleConnector.GetScope(),
		ClaimsToRoles: teleConnector.GetClaimsToRoles(),
	}

	return connector
}

// ToStorageConnector converts to Storage OIDC Connector
func (c *OIDConnector) ToStorageConnector() teleservices.OIDCConnector {
	teleConnector := newConnector(
		c.ID,
		teleservices.OIDCConnectorSpecV2{
			IssuerURL:     c.IssuerURL,
			ClientID:      c.ClientID,
			RedirectURL:   c.RedirectURL,
			Scope:         c.Scope,
			Display:       c.Display,
			ClaimsToRoles: c.ClaimsToRoles,
			ClientSecret:  c.ClientSecret,
		})

	return teleConnector
}

// Apply updates provided Storage OIDC Connector with currect values
func (c *OIDConnector) Apply(teleConnector teleservices.OIDCConnector) {
	teleConnector.SetName(c.ID)
	teleConnector.SetIssuerURL(c.IssuerURL)
	teleConnector.SetClientID(c.ClientID)
	teleConnector.SetRedirectURL(c.RedirectURL)
	teleConnector.SetScope(c.Scope)
	teleConnector.SetClaimsToRoles(c.ClaimsToRoles)
	teleConnector.SetDisplay(c.Display)
	teleConnector.SetClientSecret(c.ClientSecret)
}

func newConnector(name string, spec teleservices.OIDCConnectorSpecV2) teleservices.OIDCConnector {
	return &teleservices.OIDCConnectorV2{
		Kind:    teleservices.KindOIDC,
		Version: teleservices.V2,
		Metadata: teleservices.Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}
}
