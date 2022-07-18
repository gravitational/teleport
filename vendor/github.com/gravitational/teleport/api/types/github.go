/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"time"

	"github.com/gravitational/teleport/api/utils"

	"github.com/gravitational/trace"
)

// GithubConnector defines an interface for a Github OAuth2 connector
type GithubConnector interface {
	// ResourceWithSecrets is a common interface for all resources
	ResourceWithSecrets
	// SetMetadata sets object metadata
	SetMetadata(meta Metadata)
	// GetClientID returns the connector client ID
	GetClientID() string
	// SetClientID sets the connector client ID
	SetClientID(string)
	// GetClientSecret returns the connector client secret
	GetClientSecret() string
	// SetClientSecret sets the connector client secret
	SetClientSecret(string)
	// GetRedirectURL returns the connector redirect URL
	GetRedirectURL() string
	// SetRedirectURL sets the connector redirect URL
	SetRedirectURL(string)
	// GetTeamsToLogins returns the mapping of Github teams to allowed logins
	GetTeamsToLogins() []TeamMapping
	// SetTeamsToLogins sets the mapping of Github teams to allowed logins
	SetTeamsToLogins([]TeamMapping)
	// MapClaims returns the list of allows logins based on the retrieved claims
	// returns list of logins and kubernetes groups
	MapClaims(GithubClaims) (logins []string, kubeGroups []string, kubeUsers []string)
	// GetDisplay returns the connector display name
	GetDisplay() string
	// SetDisplay sets the connector display name
	SetDisplay(string)
}

// NewGithubConnector creates a new Github connector from name and spec
func NewGithubConnector(name string, spec GithubConnectorSpecV3) (GithubConnector, error) {
	c := &GithubConnectorV3{
		Metadata: Metadata{
			Name: name,
		},
		Spec: spec,
	}
	if err := c.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return c, nil
}

// GithubClaims represents Github user information obtained during OAuth2 flow
type GithubClaims struct {
	// Username is the user's username
	Username string
	// OrganizationToTeams is the user's organization and team membership
	OrganizationToTeams map[string][]string
	// Teams is the users team membership
	Teams []string
}

// GetVersion returns resource version
func (c *GithubConnectorV3) GetVersion() string {
	return c.Version
}

// GetKind returns resource kind
func (c *GithubConnectorV3) GetKind() string {
	return c.Kind
}

// GetSubKind returns resource sub kind
func (c *GithubConnectorV3) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind
func (c *GithubConnectorV3) SetSubKind(s string) {
	c.SubKind = s
}

// GetResourceID returns resource ID
func (c *GithubConnectorV3) GetResourceID() int64 {
	return c.Metadata.ID
}

// SetResourceID sets resource ID
func (c *GithubConnectorV3) SetResourceID(id int64) {
	c.Metadata.ID = id
}

// GetName returns the name of the connector
func (c *GithubConnectorV3) GetName() string {
	return c.Metadata.GetName()
}

// SetName sets the connector name
func (c *GithubConnectorV3) SetName(name string) {
	c.Metadata.SetName(name)
}

// Expiry returns the connector expiration time
func (c *GithubConnectorV3) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// SetExpiry sets the connector expiration time
func (c *GithubConnectorV3) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// SetMetadata sets connector metadata
func (c *GithubConnectorV3) SetMetadata(meta Metadata) {
	c.Metadata = meta
}

// GetMetadata returns the connector metadata
func (c *GithubConnectorV3) GetMetadata() Metadata {
	return c.Metadata
}

// WithoutSecrets returns an instance of resource without secrets.
func (c *GithubConnectorV3) WithoutSecrets() Resource {
	if c.GetClientSecret() == "" {
		return c
	}
	c2 := *c
	c2.SetClientSecret("")
	return &c2
}

// setStaticFields sets static resource header and metadata fields.
func (c *GithubConnectorV3) setStaticFields() {
	c.Kind = KindGithubConnector
	c.Version = V3
}

// CheckAndSetDefaults verifies the connector is valid and sets some defaults
func (c *GithubConnectorV3) CheckAndSetDefaults() error {
	c.setStaticFields()
	if err := c.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetClientID returns the connector client ID
func (c *GithubConnectorV3) GetClientID() string {
	return c.Spec.ClientID
}

// SetClientID sets the connector client ID
func (c *GithubConnectorV3) SetClientID(id string) {
	c.Spec.ClientID = id
}

// GetClientSecret returns the connector client secret
func (c *GithubConnectorV3) GetClientSecret() string {
	return c.Spec.ClientSecret
}

// SetClientSecret sets the connector client secret
func (c *GithubConnectorV3) SetClientSecret(secret string) {
	c.Spec.ClientSecret = secret
}

// GetRedirectURL returns the connector redirect URL
func (c *GithubConnectorV3) GetRedirectURL() string {
	return c.Spec.RedirectURL
}

// SetRedirectURL sets the connector redirect URL
func (c *GithubConnectorV3) SetRedirectURL(redirectURL string) {
	c.Spec.RedirectURL = redirectURL
}

// GetTeamsToLogins returns the connector team membership mappings
func (c *GithubConnectorV3) GetTeamsToLogins() []TeamMapping {
	return c.Spec.TeamsToLogins
}

// SetTeamsToLogins sets the connector team membership mappings
func (c *GithubConnectorV3) SetTeamsToLogins(teamsToLogins []TeamMapping) {
	c.Spec.TeamsToLogins = teamsToLogins
}

// GetDisplay returns the connector display name
func (c *GithubConnectorV3) GetDisplay() string {
	return c.Spec.Display
}

// SetDisplay sets the connector display name
func (c *GithubConnectorV3) SetDisplay(display string) {
	c.Spec.Display = display
}

// MapClaims returns a list of logins based on the provided claims,
// returns a list of logins and list of kubernetes groups
func (c *GithubConnectorV3) MapClaims(claims GithubClaims) ([]string, []string, []string) {
	var logins, kubeGroups, kubeUsers []string
	for _, mapping := range c.GetTeamsToLogins() {
		teams, ok := claims.OrganizationToTeams[mapping.Organization]
		if !ok {
			// the user does not belong to this organization
			continue
		}
		for _, team := range teams {
			// see if the user belongs to this team
			if team == mapping.Team {
				logins = append(logins, mapping.Logins...)
				kubeGroups = append(kubeGroups, mapping.KubeGroups...)
				kubeUsers = append(kubeUsers, mapping.KubeUsers...)
			}
		}
	}
	return utils.Deduplicate(logins), utils.Deduplicate(kubeGroups), utils.Deduplicate(kubeUsers)
}
