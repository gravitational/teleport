package ui

import (
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// Plugin represents a hosted plugin instance
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
	case *types.PluginSpecV1_Jamf:
		return "Devices will be synced from Jamf to Teleport device inventory"
	case *types.PluginSpecV1_Okta:
		return "Okta applications and groups will be synced to Teleport"
	case *types.PluginSpecV1_Opsgenie:
		if len(settings.Opsgenie.DefaultSchedules) > 0 {
			return fmt.Sprintf(`Teleport access requests will show up as alerts in schedule %q`, settings.Opsgenie.DefaultSchedules[0])
		}
		return "Teleport access requests will be created in the Opsgenie schedule indicated by opsgenie_notify_services annotation on the access request"
	default:
		return ""
	}
}
