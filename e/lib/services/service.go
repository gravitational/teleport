/*
Copyright 2023 Gravitational, Inc.

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
