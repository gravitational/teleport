package services

import "github.com/gravitational/teleport"

const (
	// OktaIdentityEvent is generated when the Okta service identity has been
	// initialized in the backend.
	OktaIdentityEvent = "OktaIdentity"

	// OktaReady is generated when the Teleport Okta service is ready to start
	// accepting connections.
	OktaReady = "OktaReady"

	// OktaStopped is generated when the Teleport Okta service has stopped.
	OktaStopped = "OktaStopped"
)

// EventWithComponents will generate an event name with components appended to the end.
func EventWithComponents(eventName string, components ...string) string {
	return teleport.Component(append([]string{eventName}, components...)...)
}

// pluginLogComponent will generate a log component based on the plugin name.
func pluginLogComponent(pluginName string) string {
	return teleport.Component("plugin", pluginName)
}
