package ui

import (
	"fmt"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// PluginSpec identifies a type as a plugin Spec
type PluginSpec interface {
	PluginSpecType() types.PluginType
}

// OktaPluginSpec holds information about the Okta plugin.
type OktaPluginSpec struct {
	// SCIMBearerToken is the plain text of the bearer token that Okta will use
	// to authenticate SCIM requests
	SCIMBearerToken string `json:"scimBearerToken,omitempty"`

	// OktaAppID is the Okta ID of the SAML App created during the Okta plugin
	// installation
	OktaAppID string `json:"oktaAppId,omitempty"`

	// OktaAppName is the human readable name of the Okta SAML app created
	// during the Okta plugin installation
	OktaAppName string `json:"oktaAppName,omitempty"`

	// TeleportSSOConnector is the name of the Teleport SAML SSO connector
	// created by the plugin during installation
	TeleportSSOConnector string `json:"teleportSsoConnector,omitempty"`

	// Error contains a description of any failures during plugin installation
	// that were deemed not serious enough to fail the plugin installation, but
	// may effect the operation of advanced features like User Sync or SCIM.
	Error string `json:"error,omitempty"`
}

// PluginSpecType implements PluginSpec for OktaPluginSpec
func (*OktaPluginSpec) PluginSpecType() types.PluginType {
	return types.PluginTypeOkta
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
}

// NewPlugin constructs a new UI plugin from types.Plugin
func NewPlugin(p types.Plugin) (*Plugin, error) {
	if p == nil {
		return nil, trace.BadParameter("p must be set")
	}
	var statusCode types.PluginStatusCode
	if s := p.GetStatus(); s != nil {
		statusCode = s.GetCode()
	}

	return &Plugin{
		Name:       p.GetName(),
		Type:       p.GetType(),
		Details:    pluginDetails(p),
		StatusCode: statusCode,
	}, nil
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
	default:
		return ""
	}
}
