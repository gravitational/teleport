package instance

import (
	"context"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
)

// Instance represents a single plugin instance.
type Instance struct {
	// Cancel is a function that closes the instance's context
	Cancel func()

	// Plugin is a copy of the resource that was used to configure the running
	// plugin instance
	Plugin *types.PluginV1

	// staticCredentials is the list of known credential at startup time.
	StaticCredentials []*types.PluginStaticCredentialsV1
}

// New creates and initializes a new [Instance] to represent a running plugin instance.
func New(cancel context.CancelFunc, plugin *types.PluginV1, credentials []*types.PluginStaticCredentialsV1) *Instance {
	return &Instance{
		Cancel:            cancel,
		Plugin:            apiutils.CloneProtoMsg(plugin),
		StaticCredentials: credentials,
	}
}

// IsUpToDate checks if the running plugin represented by this [Instance] was
// started with the latest configuration.
func (i Instance) IsUpToDate(plugin *types.PluginV1) bool {
	return i.Plugin.Spec.Equal(&plugin.Spec) && i.Plugin.Credentials.Equal(plugin.Credentials)
}

// FindCredentialByName looks up a specific credential instance in the plugin's
// in-use credential list, returning nil if not found.
func (i *Instance) FindCredentialByName(name string) *types.PluginStaticCredentialsV1 {
	for _, c := range i.StaticCredentials {
		if c.GetName() == name {
			return c
		}
	}
	return nil
}

// GetName fetches the underlying plugin's name.
func (i *Instance) GetName() string {
	if i.Plugin == nil {
		return ""
	}
	return i.Plugin.GetName()
}
