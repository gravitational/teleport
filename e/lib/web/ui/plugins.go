package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// PluginSpec identifies a type as a plugin Spec
type PluginSpec interface {
	PluginSpecType() types.PluginType
}

type SlackPluginSpec struct {
	FallbackChannel string `json:"fallbackChannel,omitempty"`
}

// PluginSpecType implements PluginSpec for SlackPluginSpec
func (*SlackPluginSpec) PluginSpecType() types.PluginType {
	return types.PluginTypeSlack
}

type MattermostPluginSpec struct {
	Channel       string `json:"channel,omitempty"`
	Team          string `json:"team,omitempty"`
	ReportToEmail string `json:"reportToEmail,omitempty"`
}

// PluginSpecType implements PluginSpec for MattermostPluginSpec
func (*MattermostPluginSpec) PluginSpecType() types.PluginType {
	return types.PluginTypeMattermost
}

type OpsgeniePluginSpec struct {
	DefaultSchedules []string `json:"defaultSchedules,omitempty"`
}

// PluginSpecType implements PluginSpec for OpsgeniePluginSpec
func (*OpsgeniePluginSpec) PluginSpecType() types.PluginType {
	return types.PluginTypeOpsgenie
}

type DatadogPluginSpec struct {
	ApiEndpoint       string `json:"apiEndpoint,omitempty"`
	FallbackRecipient string `json:"fallbackRecipient,omitempty"`
}

// PluginSpecType implements PluginSpec for DatadogPluginSpec
func (*DatadogPluginSpec) PluginSpecType() types.PluginType {
	return types.PluginTypeDatadog
}

type MsTeamsPluginSpec struct {
	DefaultRecipient string `json:"defaultRecipient,omitempty"`
}

// PluginSpecType implements PluginSpec for MsTeamsPluginSpec
func (*MsTeamsPluginSpec) PluginSpecType() types.PluginType {
	return types.PluginTypeMSTeams
}

// NetIQPluginSpec holds the configuration for the NetIQ plugin.
type NetIQPluginSpec struct {
	ApiEndpoint         string `json:"apiEndpoint,omitempty"`
	OauthIssuerEndpoint string `json:"oauthIssuerEndpoint,omitempty"`
	InsecureSkipVerify  bool   `json:"insecureSkipVerify,omitempty"`
}

// PluginSpecType implements PluginSpec for NetIqPluginSpec
func (*NetIQPluginSpec) PluginSpecType() types.PluginType {
	return types.PluginTypeNetIQ
}

type EmailPluginSpec struct {
	Sender            string `json:"sender,omitempty"`
	FallbackRecipient string `json:"fallbackRecipient,omitempty"`
}

// PluginSpecType implements PluginSpec for EmailPluginSpec
func (*EmailPluginSpec) PluginSpecType() types.PluginType {
	return types.PluginTypeEmail
}

// Plugin holds a UI-visible representation of a hosted plugin instance
type Plugin struct {
	// Name of the plugin
	Name string `json:"name"`

	// Type of the plugin
	Type types.PluginType `json:"type,omitempty"`

	// Details is a free-form text field,
	// providing provider-specific information about the plugin instance.
	Details string `json:"details"`

	// StatusCode is the user-facing plugin status
	StatusCode types.PluginStatusCode `json:"statusCode"`

	// Spec contains any pluginType-specific information that may be useful to
	// the UI. May be nil at any time.
	Spec PluginSpec `json:"spec,omitempty"`

	// Status contains the last known status of the plugin
	Status *PluginStatusV1 `json:"status,omitempty"`

	// Credentials contains the credentials used by the plugin.
	// Returned only plugin creation.
	Credentials *Credentials `json:"credentials,omitempty"`
}

// OAuthPluginStartResponse contains field related to starting
// an OAuth2 grant flow.
type OAuthPluginStartResponse struct {
	RedirectURL string `json:"redirectUrl"`
}

// PluginStatusV1 holds information about the status of a plugin
type PluginStatusV1 struct {
	// Code is the status code of the plugin
	Code types.PluginStatusCode `json:"code,omitempty"`
	// LastSyncTime is the time the plugin was last run
	LastSyncTime time.Time `json:"lastRun"`
	// ErrorMessage is the last error message from the plugin
	ErrorMessage string `json:"errorMessage,omitempty"`
	// Details contains provider-specific status information
	Details *PluginDetails `json:"details,omitempty"`
}

// PluginDetails holds information about the plugin
type PluginDetails struct {
	// Gitlab is the status of the Gitlab plugin
	Gitlab *PluginGitlabDetails `json:"gitlab,omitempty"`

	// Okta is the status of the Okta plugin
	Okta *types.PluginOktaStatusV1 `json:"okta,omitempty"`

	// NetIQ is the status of the NetIQ plugin
	NetIQ *types.PluginNetIQStatusV1 `json:"netiq,omitempty"`
}

// PluginGitlabDetails holds information about the Gitlab plugin
type PluginGitlabDetails struct {
	// ImportedUsers is the number of users imported from Gitlab
	ImportedUsers uint32 `json:"importedUsers,omitempty"`
	// ImportedGroups is the number of groups imported from Gitlab
	ImportedGroups uint32 `json:"importedGroups,omitempty"`
	// ImportedProjects is the number of projects imported from Gitlab
	ImportedProjects uint32 `json:"importedProjects,omitempty"`
}

// UnmarshalJSON implements spec-aware JSON decoding for Plugin
func (p *Plugin) UnmarshalJSON(data []byte) error {
	type plugin Plugin

	var msg struct {
		Type types.PluginType `json:"type,omitempty"`
	}

	if err := json.Unmarshal(data, &msg); err != nil {
		return trace.Wrap(err)
	}

	var out plugin
	switch msg.Type {
	case types.PluginTypeOkta:
		out.Spec = &OktaPluginSpec{}
	case types.PluginTypeNetIQ:
		out.Spec = &NetIQPluginSpec{}
	}

	if err := json.Unmarshal(data, &out); err != nil {
		return trace.Wrap(err)
	}

	*p = (Plugin)(out)
	return nil
}

// NewPlugin constructs a new UI plugin from types.Plugin
func NewPlugin(p *types.PluginV1) (*Plugin, error) {
	if p == nil {
		return nil, trace.BadParameter("p must be set")
	}

	status := p.GetStatus()
	uiP := &Plugin{
		Name:       p.GetName(),
		Type:       p.GetType(),
		Details:    pluginDetails(p),
		Spec:       pluginSpec(p),
		StatusCode: status.GetCode(),
		Status: &PluginStatusV1{
			Code:         status.GetCode(),
			LastSyncTime: status.GetLastSyncTime(),
			ErrorMessage: status.GetErrorMessage(),
		},
	}

	switch {
	case status.GetGitlab() != nil:
		gitlabStatus := status.GetGitlab()
		uiP.Status.Details = &PluginDetails{
			Gitlab: &PluginGitlabDetails{
				ImportedUsers:    gitlabStatus.ImportedUsers,
				ImportedGroups:   gitlabStatus.ImportedGroups,
				ImportedProjects: gitlabStatus.ImportedProjects,
			},
		}

	case status.GetOkta() != nil:
		details, err := makeOktaDetails(p)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		uiP.Status.Details = details
	case status.GetNetIq() != nil:
		uiP.Status.Details = &PluginDetails{
			NetIQ: status.GetNetIq(),
		}
	}

	return uiP, nil
}

func pluginDetails(p types.Plugin) string {
	v1, ok := p.(*types.PluginV1)
	if !ok {
		return ""
	}
	switch settings := v1.Spec.Settings.(type) {
	case *types.PluginSpecV1_SlackAccessPlugin:
		return fmt.Sprintf(`Messages will be sent to assigned reviewers and the "%s" channel`, settings.SlackAccessPlugin.FallbackChannel)
	case *types.PluginSpecV1_Mattermost:
		hasTeamChannelDefined := len(settings.Mattermost.Channel) > 0 && len(settings.Mattermost.Team) > 0
		hasEmailDefined := len(settings.Mattermost.ReportToEmail) > 0
		if hasTeamChannelDefined && hasEmailDefined {
			return fmt.Sprintf(`Messages will be sent to assigned reviewers, to Mattermost user "%s", and to the "%s" channel from team "%s"`, settings.Mattermost.ReportToEmail, settings.Mattermost.Channel, settings.Mattermost.Team)
		}
		if hasTeamChannelDefined && !hasEmailDefined {
			return fmt.Sprintf(`Messages will be sent to assigned reviewers and to the "%s" channel from team "%s"`, settings.Mattermost.Channel, settings.Mattermost.Team)
		}
		if hasEmailDefined {
			return fmt.Sprintf(`Messages will be sent to assigned reviewers and to Mattermost user "%s"`, settings.Mattermost.ReportToEmail)
		}
		return "Messages will be sent to assigned reviewers defined in access requests"
	case *types.PluginSpecV1_Jamf:
		return "Devices will be synced from Jamf to Teleport device inventory"
	case *types.PluginSpecV1_Jira:
		return fmt.Sprintf(`Teleport access requests will be created on %s project %s`, settings.Jira.ServerUrl, settings.Jira.ProjectKey)
	case *types.PluginSpecV1_Okta:
		return "Okta applications and groups will be synced to Teleport"
	case *types.PluginSpecV1_Opsgenie:
		if len(settings.Opsgenie.DefaultSchedules) > 0 {
			return fmt.Sprintf(`Teleport access requests will show up as alerts in schedule %q`, settings.Opsgenie.DefaultSchedules[0])
		}
		return "Teleport access requests will be created in the Opsgenie schedule indicated by opsgenie_notify_services annotation on the access request"
	case *types.PluginSpecV1_PagerDuty:
		return fmt.Sprintf(`Incidents will be created by PagerDuty user %q`, settings.PagerDuty.UserEmail)
	case *types.PluginSpecV1_Discord:
		channels := settings.Discord.RoleToRecipients[types.Wildcard]
		suffix := ""
		if len(channels.ChannelIds) > 1 {
			suffix = "s"
		}
		combinedChannels := strings.Join(channels.ChannelIds, ", ")
		return fmt.Sprintf(`Messages will be sent to Discord channel%s %s`, suffix, combinedChannels)
	case *types.PluginSpecV1_ServiceNow:
		return fmt.Sprintf(`Incidents will be created at %q`, settings.ServiceNow.ApiEndpoint)
	case *types.PluginSpecV1_Gitlab:
		return fmt.Sprintf(`GitLab users, projects and groups will be imported from %q`, settings.Gitlab.ApiEndpoint)
	case *types.PluginSpecV1_EntraId:
		return "Users and groups will be synchronized from the Entra ID directory"
	case *types.PluginSpecV1_Datadog:
		return fmt.Sprintf(`Incidents will be created at %q and notify %q recipient`, settings.Datadog.ApiEndpoint, settings.Datadog.FallbackRecipient)
	case *types.PluginSpecV1_Msteams:
		return fmt.Sprintf(`Messages will be sent to assigned reviewers and the default recipient "%s"`, settings.Msteams.DefaultRecipient)
	case *types.PluginSpecV1_Email:
		return fmt.Sprintf(`Emails will be sent by %q to %q`, settings.Email.Sender, settings.Email.FallbackRecipient)
	case *types.PluginSpecV1_NetIq:
		return fmt.Sprintf(`Users, groups, roles and resources will be synchronized from NetIQ at %q`, settings.NetIq.ApiEndpoint)
	default:
		return ""
	}
}

func pluginSpec(p types.Plugin) PluginSpec {
	v1, ok := p.(*types.PluginV1)
	if !ok {
		return nil
	}
	switch settings := v1.Spec.Settings.(type) {
	case *types.PluginSpecV1_SlackAccessPlugin:
		return &SlackPluginSpec{
			FallbackChannel: settings.SlackAccessPlugin.FallbackChannel,
		}

	case *types.PluginSpecV1_Mattermost:
		return &MattermostPluginSpec{
			Channel:       settings.Mattermost.Channel,
			Team:          settings.Mattermost.Team,
			ReportToEmail: settings.Mattermost.ReportToEmail,
		}

	case *types.PluginSpecV1_Opsgenie:
		return &OpsgeniePluginSpec{
			DefaultSchedules: settings.Opsgenie.DefaultSchedules,
		}

	case *types.PluginSpecV1_Okta:
		return &OktaPluginSpec{
			OktaOrgURL:              settings.Okta.OrgUrl,
			OktaAppID:               settings.Okta.SyncSettings.AppId,
			OktaAppName:             settings.Okta.SyncSettings.AppName,
			TeleportSSOConnector:    settings.Okta.SyncSettings.SsoConnectorId,
			DefaultOwners:           settings.Okta.SyncSettings.DefaultOwners,
			EnableUserSync:          settings.Okta.SyncSettings.SyncUsers,
			AssignDefaultRoles:      settings.Okta.SyncSettings.GetAssignDefaultRoles(),
			EnableAppGroupSync:      !settings.Okta.SyncSettings.DisableSyncAppGroups,
			EnableAccessListSync:    settings.Okta.SyncSettings.SyncAccessLists,
			EnableBidirectionalSync: !settings.Okta.SyncSettings.DisableBidirectionalSync,
			CredentialInfo:          toCredentialInfo(settings.Okta.CredentialsInfo),
			EnableSystemLogExport:   settings.Okta.SyncSettings.EnableSystemLogExport,
		}
	case *types.PluginSpecV1_Msteams:
		return &MsTeamsPluginSpec{
			DefaultRecipient: settings.Msteams.DefaultRecipient,
		}

	case *types.PluginSpecV1_Datadog:
		return &DatadogPluginSpec{
			ApiEndpoint:       settings.Datadog.ApiEndpoint,
			FallbackRecipient: settings.Datadog.FallbackRecipient,
		}

	case *types.PluginSpecV1_Email:
		return &EmailPluginSpec{
			Sender:            settings.Email.Sender,
			FallbackRecipient: settings.Email.FallbackRecipient,
		}
	case *types.PluginSpecV1_NetIq:
		return &NetIQPluginSpec{
			ApiEndpoint:         settings.NetIq.ApiEndpoint,
			OauthIssuerEndpoint: settings.NetIq.OauthIssuerEndpoint,
			InsecureSkipVerify:  settings.NetIq.InsecureSkipVerify,
		}
	default:
		return nil
	}
}

// PluginNeedsCleanup is the response from the needs cleanup endpoint.
type PluginNeedsCleanup struct {
	// NeedsCleanup is whether or not the plugin needs cleanup.
	NeedsCleanup bool `json:"needsCleanup"`
}

func toCredentialInfo(info *types.PluginOktaCredentialsInfo) *OktaCredentialInfo {
	if info == nil {
		return nil
	}

	return &OktaCredentialInfo{
		HasConfiguredSSMSToken:        info.HasSsmToken,
		HasConfiguredOauthCredentials: info.HasOauthCredentials,
		HasConfiguredSCIMToken:        info.HasSsmToken,
	}
}

// PluginUpdateRequest is the request to update a plugin's configuration.
type PluginUpdateRequest struct {
	// Plugin is the name of the plugin to update
	Plugin string            `json:"plugin,omitempty"`
	Okta   *OktaPluginUpdate `json:"okta,omitempty"`
}

// OktaPluginUpdate contains the fields that can be updated in the Okta plugin.
type OktaPluginUpdate struct {
	// EnableUserSync indicates whether User Sync should be enabled/disabled.
	EnableUserSync bool `json:"enableUserSync,omitempty"`
	// AssignDefaultRoles indicates whether the builtin okta-requester role should be
	// assigned to the synchronized users.
	AssignDefaultRoles bool `json:"assignDefaultRoles,omitempty"`
	// EnableAccessListSync indicates whether Access List Sync should be enabled/disabled.
	EnableAccessListSync bool `json:"enableAccessListSync,omitempty"`
	// EnableAppGroupSync indicates whether App/Group Sync should be enabled/disabled.
	EnableAppGroupSync bool `json:"enableAppGroupSync,omitempty"`
	// EnableBidirectionalSync indicates whether changes made in Teleport should be synced back to Okta.
	EnableBidirectionalSync bool `json:"enableBidirectionalSync,omitempty"`
	// ClientID is the Client ID used for OAuth with Okta.
	ClientID string `json:"clientID"`
	// DefaultOwners is the list of default owners for synced Access Lists.
	DefaultOwners []string `json:"defaultOwners,omitempty"`
	// SCIMToken is the SCIM bearer token used for SCIM operations.
	SCIMToken string `json:"scimToken,omitempty"`
	// AppFilters is the list of filters for applications to sync.
	AppFilters []string `json:"appFilters,omitempty"`
	// GroupFilters is the list of filters for groups to sync.
	GroupFilters []string `json:"groupFilters,omitempty"`
	// EnableSystemLogExport indicates whether the Teleport Identity Security SIEM integration for Okta should be enabled.
	EnableSystemLogExport bool `json:"enableSystemLogExport,omitempty"`
}

// Credentials holds plugin credentials returned during creation only.
type Credentials struct {
	// OAuthCreds holds OAuth client credentials.
	OAuthCreds *OAuthCredentials `json:"oauth_creds,omitempty"`
}

// OAuthCredentials holds OAuth client credentials.
type OAuthCredentials struct {
	// ClientID is the OAuth client ID.
	ClientID string `json:"client_id,omitempty"`
	// ClientSecret is the OAuth client secret.
	ClientSecret string `json:"client_secret,omitempty"`
}
