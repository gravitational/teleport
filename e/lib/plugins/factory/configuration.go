package factory

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
)

// pluginConfiguration is an implementation of common.PluginConfiguration
type pluginConfiguration struct {
	client teleport.Client

	pluginConfig pluginConfig
	// defaultRoutes are the recipients defined in
	// the plugin configuration.
	defaultRoutes []string
	pluginType    types.PluginType
	// teleportUser is the name of the teleport user that acts as the
	// access request approver.
	teleportUser string
}

func (p *pluginConfiguration) GetRecipients() common.RawRecipientsMap {
	return common.RawRecipientsMap{
		"*": p.defaultRoutes,
	}
}

func (p *pluginConfiguration) GetTeleportClient(ctx context.Context) (teleport.Client, error) {
	return p.client, nil
}

func (p *pluginConfiguration) NewBot(clusterName string, webProxyAddr string) (common.MessagingBot, error) {
	if p.pluginConfig != nil {
		return p.pluginConfig.NewBot(clusterName, webProxyAddr)
	}
	return nil, trace.BadParameter("plugin config must be provided")
}

// GetPluginType returns the type of plugin this config is for.
func (p *pluginConfiguration) GetPluginType() types.PluginType {
	return p.pluginType
}

func (p *pluginConfiguration) GetTeleportUser() string {
	return p.teleportUser
}

type pluginConfig interface {
	NewBot(clusterName, webProxyUrl string) (common.MessagingBot, error)
}
