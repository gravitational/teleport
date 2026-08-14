package plugins

import "github.com/gravitational/teleport/api/types"

// HostedPluginLabel defines the name for the hosted plugin label.
// When this label is set to "true" on a Plugin resource,
// it indicates that the Plugin should be run by the Cloud service,
// rather than self-hosted plugin services.
const HostedPluginLabel = types.TeleportNamespace + "/hosted-plugin"
