package ui

import (
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// PluginStatus represents the user-facing plugin status
type PluginStatus struct {
	// Code is the short form of the status.
	// This corresponds with the proto PluginStatus, but is more user-readable.
	Code string `json:"code"`

	// Description is a longer, user-readable status string.
	Description string `json:"description,omitempty"`
}

// Plugin represents a hosted plugin instance
type Plugin struct {
	// Name of the plugin
	Name string `json:"name"`

	// Type of the plugin
	Type types.PluginType `json:"type,omitempty"`

	// Details is a free-form text field,
	// providing provider-specific information about the plugin instance.
	Details string `json:"details"`

	// Status is the user-facing plugin status
	Status PluginStatus `json:"status"`
}

// NewPlugin constructs a new UI plugin from types.Plugin
func NewPlugin(p types.Plugin) (*Plugin, error) {
	if p == nil {
		return nil, trace.BadParameter("p must be set")
	}

	return &Plugin{
		Name:    p.GetName(),
		Type:    p.GetType(),
		Details: pluginDetails(p),
		Status:  pluginStatus(p.GetStatus()),
	}, nil
}

func pluginStatus(s types.PluginStatus) PluginStatus {
	status := PluginStatus{}
	switch s.GetCode() {
	case types.PluginStatusCode_UNKNOWN:
		status.Code = "Unknown"
	case types.PluginStatusCode_RUNNING:
		status.Code = "Running"
	case types.PluginStatusCode_OTHER_ERROR:
		status.Code = "Unknown error"
	case types.PluginStatusCode_UNAUTHORIZED:
		status.Code = "Unauthorized"
	case types.PluginStatusCode_SLACK_NOT_IN_CHANNEL:
		status.Code = "Bot not invited to channel"
	}
	return status
}

func pluginDetails(p types.Plugin) string {
	v1, ok := p.(*types.PluginV1)
	if !ok {
		return ""
	}
	switch settings := v1.Spec.Settings.(type) {
	case *types.PluginSpecV1_SlackAccessPlugin:
		return fmt.Sprintf(`Messages will be sent to assigned reviewers and the "%s" channel`, settings.SlackAccessPlugin.FallbackChannel)
	default:
		return ""
	}
}
