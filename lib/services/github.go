/*
Copyright 2017 Gravitational, Inc.

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

package services

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// GithubConnector defines an interface for a Github OAuth2 connector
type GithubConnector interface {
	// Resource is a common interface for all resources
	Resource
	// CheckAndSetDefaults validates the connector and sets some defaults
	CheckAndSetDefaults() error
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
	MapClaims(GithubClaims) []string
	// GetDisplay returns the connector display name
	GetDisplay() string
	// SetDisplay sets the connector display name
	SetDisplay(string)
}

// NewGithubConnector creates a new Github connector from name and spec
func NewGithubConnector(name string, spec GithubConnectorSpecV3) GithubConnector {
	return &GithubConnectorV3{
		Kind:    KindGithubConnector,
		Version: V3,
		Metadata: Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}
}

// GithubConnectorV3 represents a Github connector
type GithubConnectorV3 struct {
	// Kind is a resource kind, for Github connector it is "github"
	Kind string `json:"kind"`
	// Version is resource version
	Version string `json:"version"`
	// Metadata is resource metadata
	Metadata Metadata `json:"metadata"`
	// Spec contains connector specification
	Spec GithubConnectorSpecV3 `json:"spec"`
}

// GithubConnectorSpecV3 is the current Github connector spec
type GithubConnectorSpecV3 struct {
	// ClientID is the Github OAuth app client ID
	ClientID string `json:"client_id"`
	// ClientSecret is the Github OAuth app client secret
	ClientSecret string `json:"client_secret"`
	// RedirectURL is the authorization callback URL
	RedirectURL string `json:"redirect_url"`
	// TeamsToLogins maps Github team memberships onto allowed logins/roles
	TeamsToLogins []TeamMapping `json:"teams_to_logins"`
	// Display is the connector display name
	Display string `json:"display"`
}

// TeamMapping represents a single team membership mapping
type TeamMapping struct {
	// Organization is a Github organization a user belongs to
	Organization string `json:"organization"`
	// Team is a team within the organization a user belongs to
	Team string `json:"team"`
	// Logins is a list of allowed logins for this org/team
	Logins []string `json:"logins"`
}

// GithubClaims represents Github user information obtained during OAuth2 flow
type GithubClaims struct {
	// Username is the user's username
	Username string
	// OrganizationToTeams is the user's organization and team membership
	OrganizationToTeams map[string][]string
}

// GetName returns the name of the connector
func (c *GithubConnectorV3) GetName() string {
	return c.Metadata.GetName()
}

// SetName sets the connector name
func (c *GithubConnectorV3) SetName(name string) {
	c.Metadata.SetName(name)
}

// Expires returns the connector expiration time
func (c *GithubConnectorV3) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// SetExpiry sets the connector expiration time
func (c *GithubConnectorV3) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// SetTTL sets the connector TTL
func (c *GithubConnectorV3) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	c.Metadata.SetTTL(clock, ttl)
}

// GetMetadata returns the connector metadata
func (c *GithubConnectorV3) GetMetadata() Metadata {
	return c.Metadata
}

// CheckAndSetDefaults verifies the connector is valid and sets some defaults
func (c *GithubConnectorV3) CheckAndSetDefaults() error {
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

// MapClaims returns a list of logins based on the provided claims
func (c *GithubConnectorV3) MapClaims(claims GithubClaims) []string {
	var logins []string
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
			}
		}
	}
	return utils.Deduplicate(logins)
}

var githubConnectorMarshaler GithubConnectorMarshaler = &TeleportGithubConnectorMarshaler{}

// SetGithubConnectorMarshaler sets Github connector marshaler
func SetGithubConnectorMarshaler(m GithubConnectorMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	githubConnectorMarshaler = m
}

// GetGithubConnectorMarshaler returns currently set Github connector marshaler
func GetGithubConnectorMarshaler() GithubConnectorMarshaler {
	marshalerMutex.RLock()
	defer marshalerMutex.RUnlock()
	return githubConnectorMarshaler
}

// GithubConnectorMarshaler defines interface for Github connector marshaler
type GithubConnectorMarshaler interface {
	// Unmarshal unmarshals connector from binary representation
	Unmarshal(bytes []byte) (GithubConnector, error)
	// Marshal marshals connector to binary representation
	Marshal(c GithubConnector, opts ...MarshalOption) ([]byte, error)
}

// GetGithubConnectorSchema returns schema for Github connector
func GetGithubConnectorSchema() string {
	return fmt.Sprintf(GithubConnectorV3SchemaTemplate, MetadataSchema, GithubConnectorSpecV3Schema)
}

// TeleportGithubConnectorMarshaler is the default Github connector marshaler
type TeleportGithubConnectorMarshaler struct{}

// UnmarshalGithubConnector unmarshals Github connector from JSON
func (*TeleportGithubConnectorMarshaler) Unmarshal(bytes []byte) (GithubConnector, error) {
	var h ResourceHeader
	if err := json.Unmarshal(bytes, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case V3:
		var c GithubConnectorV3
		if err := utils.UnmarshalWithSchema(GetGithubConnectorSchema(), &c, bytes); err != nil {
			return nil, trace.Wrap(err)
		}
		if err := c.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		return &c, nil
	}
	return nil, trace.BadParameter(
		"Github connector resource version %q is not supported", h.Version)
}

// MarshalGithubConnector marshals Github connector to JSON
func (*TeleportGithubConnectorMarshaler) Marshal(c GithubConnector, opts ...MarshalOption) ([]byte, error) {
	return json.Marshal(c)
}

// GithubConnectorV3SchemaTemplate is the JSON schema for a Github connector
const GithubConnectorV3SchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["kind", "spec", "metadata", "version"],
  "properties": {
    "kind": {"type": "string"},
    "version": {"type": "string", "default": "v3"},
    "metadata": %v,
    "spec": %v
  }
}`

// GithubConnectorSpecV3Schema is the JSON schema for Github connector spec
var GithubConnectorSpecV3Schema = fmt.Sprintf(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["client_id", "client_secret", "redirect_url"],
  "properties": {
    "client_id": {"type": "string"},
    "client_secret": {"type": "string"},
    "redirect_url": {"type": "string"},
    "display": {"type": "string"},
    "teams_to_logins": {
      "type": "array",
      "items": %v
    }
  }
}`, TeamMappingSchema)

// TeamMappingSchema is the JSON schema for team membership mapping
var TeamMappingSchema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["organization", "team"],
  "properties": {
    "organization": {"type": "string"},
    "team": {"type": "string"},
    "logins": {
      "type": "array",
      "items": {
        "type": "string"
      }
    }
  }
}`
