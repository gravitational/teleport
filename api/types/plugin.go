/*
Copyright 2022 Gravitational, Inc.

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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
)

// PluginType represents the type of the plugin
type PluginType string

const (
	// PluginTypeUnknown is returned when no plugin type matches.
	PluginTypeUnknown PluginType = ""
	// PluginTypeServiceNow is the Servicenow access request plugin
	PluginTypeServiceNow = "servicenow"
	// PluginTypeSlack is the Slack access request plugin
	PluginTypeSlack = "slack"
	// PluginTypeOpenAI is the OpenAI plugin
	PluginTypeOpenAI = "openai"
	// PluginTypeOkta is the Okta plugin
	PluginTypeOkta = "okta"
	// PluginTypeJamf is the Jamf MDM plugin
	PluginTypeJamf = "jamf"
	// PluginTypeJira is the Jira access plugin
	PluginTypeJira = "jira"
	// PluginTypeOpsgenie is the Opsgenie access request plugin
	PluginTypeOpsgenie = "opsgenie"
	// PluginTypePagerDuty is the PagerDuty access plugin
	PluginTypePagerDuty = "pagerduty"
	// PluginTypeMattermost is the PagerDuty access plugin
	PluginTypeMattermost = "mattermost"
	// PluginTypeDiscord indicates the Discord access plugin
	PluginTypeDiscord = "discord"
)

// PluginSubkind represents the type of the plugin, e.g., access request, MDM etc.
type PluginSubkind string

const (
	// PluginSubkindUnknown is returned when no plugin subkind matches.
	PluginSubkindUnknown PluginSubkind = ""
	// PluginSubkindMDM represents MDM plugins collectively
	PluginSubkindMDM = "mdm"
	// PluginSubkindAccess represents access request plugins collectively
	PluginSubkindAccess = "access"
)

// Plugin represents a plugin instance
type Plugin interface {
	// ResourceWithSecrets provides common resource methods.
	ResourceWithSecrets
	Clone() Plugin
	GetCredentials() PluginCredentials
	GetStatus() PluginStatus
	GetType() PluginType
	SetCredentials(PluginCredentials) error
	SetStatus(PluginStatus) error
	GetGeneration() string
}

// PluginCredentials are the credentials embedded in Plugin
type PluginCredentials interface {
	GetOauth2AccessToken() *PluginOAuth2AccessTokenCredentials
	GetStaticCredentialsRef() *PluginStaticCredentialsRef
}

// PluginStatus is the plugin status
type PluginStatus interface {
	GetCode() PluginStatusCode
}

// NewPluginV1 creates a new PluginV1 resource.
func NewPluginV1(metadata Metadata, spec PluginSpecV1, creds *PluginCredentialsV1) *PluginV1 {
	p := &PluginV1{
		Metadata: metadata,
		Spec:     spec,
	}
	if creds != nil {
		p.SetCredentials(creds)
	}

	return p
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults.
func (p *PluginV1) CheckAndSetDefaults() error {
	p.setStaticFields()

	if err := p.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	switch settings := p.Spec.Settings.(type) {
	case *PluginSpecV1_SlackAccessPlugin:
		// Check settings.
		if settings.SlackAccessPlugin == nil {
			return trace.BadParameter("settings must be set")
		}
		if err := settings.SlackAccessPlugin.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

		if p.Credentials == nil {
			// TODO: after credential exchange during creation is implemented,
			// this should validate that credentials are not empty
			break
		}
		if p.Credentials.GetOauth2AccessToken() == nil {
			return trace.BadParameter("Slack access plugin can only be used with OAuth2 access token credential type")
		}
		if err := p.Credentials.GetOauth2AccessToken().CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	case *PluginSpecV1_Openai:
		if p.Credentials == nil {
			return trace.BadParameter("credentials must be set")
		}

		bearer := p.Credentials.GetBearerToken()
		if bearer == nil {
			return trace.BadParameter("openai plugin must be used with the bearer token credential type")
		}
		if bearer.Token == "" {
			return trace.BadParameter("Token must be specified")
		}
	case *PluginSpecV1_Opsgenie:
		if settings.Opsgenie == nil {
			return trace.BadParameter("missing opsgenie settings")
		}
		if err := settings.Opsgenie.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

		staticCreds := p.Credentials.GetStaticCredentialsRef()
		if staticCreds == nil {
			return trace.BadParameter("opsgenie plugin must be used with the static credentials ref type")
		}
		if len(staticCreds.Labels) == 0 {
			return trace.BadParameter("labels must be specified")
		}
	case *PluginSpecV1_Mattermost:
		if settings.Mattermost == nil {
			return trace.BadParameter("missing Mattermost settings")
		}
		if err := settings.Mattermost.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

		staticCreds := p.Credentials.GetStaticCredentialsRef()
		if staticCreds == nil {
			return trace.BadParameter("Mattermost plugin must be used with the static credentials ref type")
		}
		if len(staticCreds.Labels) == 0 {
			return trace.BadParameter("labels must be specified")
		}
	case *PluginSpecV1_Jamf:
		// Check Jamf settings.
		if settings.Jamf == nil {
			return trace.BadParameter("missing Jamf settings")
		}
		if err := settings.Jamf.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		if p.Credentials == nil {
			return trace.BadParameter("credentials must be set")
		}
		staticCreds := p.Credentials.GetStaticCredentialsRef()
		if staticCreds == nil {
			return trace.BadParameter("jamf plugin must be used with the static credentials ref type")
		}
		if len(staticCreds.Labels) == 0 {
			return trace.BadParameter("labels must be specified")
		}

	case *PluginSpecV1_Jira:
		if settings.Jira == nil {
			return trace.BadParameter("missing Jira settings")
		}

		if err := settings.Jira.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

		if p.Credentials == nil {
			return trace.BadParameter("credentials must be set")
		}

		staticCreds := p.Credentials.GetStaticCredentialsRef()
		if staticCreds == nil {
			return trace.BadParameter("jira plugin must be used with the static credentials ref type")
		}

		if len(staticCreds.Labels) == 0 {
			return trace.BadParameter("labels must be specified")
		}

	case *PluginSpecV1_Okta:
		// Check settings.
		if settings.Okta == nil {
			return trace.BadParameter("missing Okta settings")
		}
		if err := settings.Okta.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

		if p.Credentials == nil {
			return trace.BadParameter("credentials must be set")
		}
		staticCreds := p.Credentials.GetStaticCredentialsRef()
		if staticCreds == nil {
			return trace.BadParameter("okta plugin must be used with the static credentials ref type")
		}
		if len(staticCreds.Labels) == 0 {
			return trace.BadParameter("labels must be specified")
		}
	case *PluginSpecV1_PagerDuty:
		if settings.PagerDuty == nil {
			return trace.BadParameter("missing PagerDuty settings")
		}
		if err := settings.PagerDuty.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

	case *PluginSpecV1_Discord:
		if settings.Discord == nil {
			return trace.BadParameter("missing Discord settings")
		}
		if err := settings.Discord.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

		staticCreds := p.Credentials.GetStaticCredentialsRef()
		if staticCreds == nil {
			return trace.BadParameter("Discord plugin must be used with the static credentials ref type")
		}
	case *PluginSpecV1_ServiceNow:
		if settings.ServiceNow == nil {
			return trace.BadParameter("missing ServiceNow settings")
		}
		if err := settings.ServiceNow.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

		staticCreds := p.Credentials.GetStaticCredentialsRef()
		if staticCreds == nil {
			return trace.BadParameter("ServiceNow plugin must be used with the static credentials ref type")
		}

	default:
		return trace.BadParameter("settings are not set or have an unknown type")
	}

	return nil
}

// WithoutSecrets returns an instance of resource without secrets.
func (p *PluginV1) WithoutSecrets() Resource {
	if p.Credentials == nil {
		return p
	}

	p2 := p.Clone().(*PluginV1)
	p2.SetCredentials(nil)
	return p2
}

func (p *PluginV1) setStaticFields() {
	p.Kind = KindPlugin
	p.Version = V1
}

// Clone returns a copy of the Plugin instance
func (p *PluginV1) Clone() Plugin {
	return utils.CloneProtoMsg(p)
}

// GetVersion returns resource version
func (p *PluginV1) GetVersion() string {
	return p.Version
}

// GetKind returns resource kind
func (p *PluginV1) GetKind() string {
	return p.Kind
}

// GetSubKind returns resource sub kind
func (p *PluginV1) GetSubKind() string {
	return p.SubKind
}

// SetSubKind sets resource subkind
func (p *PluginV1) SetSubKind(s string) {
	p.SubKind = s
}

// GetResourceID returns resource ID
func (p *PluginV1) GetResourceID() int64 {
	return p.Metadata.ID
}

// SetResourceID sets resource ID
func (p *PluginV1) SetResourceID(id int64) {
	p.Metadata.ID = id
}

// GetRevision returns the revision
func (p *PluginV1) GetRevision() string {
	return p.Metadata.GetRevision()
}

// SetRevision sets the revision
func (p *PluginV1) SetRevision(rev string) {
	p.Metadata.SetRevision(rev)
}

// GetMetadata returns object metadata
func (p *PluginV1) GetMetadata() Metadata {
	return p.Metadata
}

// SetMetadata sets object metadata
func (p *PluginV1) SetMetadata(meta Metadata) {
	p.Metadata = meta
}

// Expiry returns expiry time for the object
func (p *PluginV1) Expiry() time.Time {
	return p.Metadata.Expiry()
}

// SetExpiry sets expiry time for the object
func (p *PluginV1) SetExpiry(expires time.Time) {
	p.Metadata.SetExpiry(expires)
}

// GetName returns the name of the User
func (p *PluginV1) GetName() string {
	return p.Metadata.Name
}

// SetName sets the name of the User
func (p *PluginV1) SetName(e string) {
	p.Metadata.Name = e
}

// GetCredentials implements Plugin
func (p *PluginV1) GetCredentials() PluginCredentials {
	return p.Credentials
}

// SetCredentials implements Plugin
func (p *PluginV1) SetCredentials(creds PluginCredentials) error {
	if creds == nil {
		p.Credentials = nil
		return nil
	}
	switch creds := creds.(type) {
	case *PluginCredentialsV1:
		p.Credentials = creds
	default:
		return trace.BadParameter("unsupported plugin credential type %T", creds)
	}
	return nil
}

// GetStatus implements Plugin
func (p *PluginV1) GetStatus() PluginStatus {
	return p.Status
}

// SetStatus implements Plugin
func (p *PluginV1) SetStatus(status PluginStatus) error {
	if status == nil {
		p.Status = PluginStatusV1{}
		return nil
	}
	p.Status = PluginStatusV1{
		Code: status.GetCode(),
	}
	return nil
}

// GetGeneration returns the plugin generation.
func (p *PluginV1) GetGeneration() string {
	return p.Spec.Generation
}

// GetType implements Plugin
func (p *PluginV1) GetType() PluginType {
	switch p.Spec.Settings.(type) {
	case *PluginSpecV1_SlackAccessPlugin:
		return PluginTypeSlack
	case *PluginSpecV1_Openai:
		return PluginTypeOpenAI
	case *PluginSpecV1_Okta:
		return PluginTypeOkta
	case *PluginSpecV1_Jamf:
		return PluginTypeJamf
	case *PluginSpecV1_Jira:
		return PluginTypeJira
	case *PluginSpecV1_Opsgenie:
		return PluginTypeOpsgenie
	case *PluginSpecV1_PagerDuty:
		return PluginTypePagerDuty
	case *PluginSpecV1_Mattermost:
		return PluginTypeMattermost
	case *PluginSpecV1_Discord:
		return PluginTypeDiscord
	case *PluginSpecV1_ServiceNow:
		return PluginTypeServiceNow
	default:
		return PluginTypeUnknown
	}
}

// CheckAndSetDefaults validates and set the default values
func (s *PluginSlackAccessSettings) CheckAndSetDefaults() error {
	if s.FallbackChannel == "" {
		return trace.BadParameter("fallback_channel must be set")
	}

	return nil
}

// CheckAndSetDefaults validates and set the default values.
func (s *PluginOktaSettings) CheckAndSetDefaults() error {
	if s.OrgUrl == "" {
		return trace.BadParameter("org_url must be set")
	}

	// If sync settings is not set, upgrade the legacy values to a
	// to a new SyncSettings block
	if s.SyncSettings == nil {
		// TODO(mdwn): Remove upgrade once modifications have been made in enterprise.
		s.SyncSettings = &PluginOktaSyncSettings{
			SyncUsers:      s.EnableUserSync,
			SsoConnectorId: s.SsoConnectorId,
		}
	}

	if s.SyncSettings.SyncUsers && s.SyncSettings.SsoConnectorId == "" {
		return trace.BadParameter("sso_connector_id must be set when user sync enabled")
	}

	if s.SyncSettings.SyncAccessLists && len(s.SyncSettings.DefaultOwners) == 0 {
		return trace.BadParameter("default owners must be set when access list import is enabled")
	}

	return nil
}

// CheckAndSetDefaults validates and set the default values
func (s *PluginOpsgenieAccessSettings) CheckAndSetDefaults() error {
	if s.ApiEndpoint == "" {
		return trace.BadParameter("opsgenie api endpoint url must be set")
	}
	return nil
}

// CheckAndSetDefaults validates and set the default values.
func (s *PluginJamfSettings) CheckAndSetDefaults() error {
	if s.JamfSpec.ApiEndpoint == "" {
		return trace.BadParameter("api endpoint must be set")
	}

	return nil
}

func (s *PluginJiraSettings) CheckAndSetDefaults() error {
	if s.ServerUrl == "" {
		return trace.BadParameter("Jira server URL must be set")
	}

	if s.ProjectKey == "" {
		return trace.BadParameter("Jira project key must be set")
	}

	if s.IssueType == "" {
		return trace.BadParameter("Jira issue type must be set")
	}

	return nil
}

// CheckAndSetDefaults validates and set the default values
func (s *PluginMattermostSettings) CheckAndSetDefaults() error {
	if s.ServerUrl == "" {
		return trace.BadParameter("server url is required")
	}

	// If one field is defined, both should be required.
	if len(s.Channel) > 0 || len(s.Team) > 0 {
		if len(s.Team) == 0 {
			return trace.BadParameter("team is required")
		}
		if len(s.Channel) == 0 {
			return trace.BadParameter("channel is required")
		}
	}
	return nil
}

// CheckAndSetDefaults validates and set the default values
func (c *PluginOAuth2AuthorizationCodeCredentials) CheckAndSetDefaults() error {
	if c.AuthorizationCode == "" {
		return trace.BadParameter("authorization_code must be set")
	}
	if c.RedirectUri == "" {
		return trace.BadParameter("redirect_uri must be set")
	}

	return nil
}

// CheckAndSetDefaults validates and set the default PagerDuty values
func (c *PluginPagerDutySettings) CheckAndSetDefaults() error {
	if c.ApiEndpoint == "" {
		return trace.BadParameter("api_endpoint must be set")
	}

	if c.UserEmail == "" {
		return trace.BadParameter("user_email must be set")
	}
	return nil
}

func (c *PluginDiscordSettings) CheckAndSetDefaults() error {
	if len(c.RoleToRecipients) == 0 {
		return trace.BadParameter("role_to_recipients must be set")
	}

	if _, present := c.RoleToRecipients[Wildcard]; !present {
		return trace.BadParameter("role_to_recipients must contain default entry `*`")
	}

	return nil
}

// CheckAndSetDefaults checks that the required fields for the servicenow plugin are set.
func (c *PluginServiceNowSettings) CheckAndSetDefaults() error {
	if c.ApiEndpoint == "" {
		return trace.BadParameter("API endpoint must be set")
	}

	return nil
}

// CheckAndSetDefaults validates and set the default values
func (c *PluginOAuth2AccessTokenCredentials) CheckAndSetDefaults() error {
	if c.AccessToken == "" {
		return trace.BadParameter("access_token must be set")
	}
	if c.RefreshToken == "" {
		return trace.BadParameter("refresh_token must be set")
	}
	c.Expires = c.Expires.UTC()

	return nil
}

// GetCode returns the status code
func (c PluginStatusV1) GetCode() PluginStatusCode {
	return c.Code
}
