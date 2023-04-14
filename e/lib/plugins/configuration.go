package plugins

import (
	"context"

	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/access/slack"
)

// pluginConfiguration is an implementation of common.PluginConfiguration
type pluginConfiguration struct {
	client       teleport.Client
	slackConfig  slack.Config
	defaultRoute string
}

func (p *pluginConfiguration) GetRecipients() common.RawRecipientsMap {
	return common.RawRecipientsMap{
		"*": {p.defaultRoute},
	}
}

func (p *pluginConfiguration) GetTeleportClient(ctx context.Context) (teleport.Client, error) {
	return p.client, nil
}

func (p *pluginConfiguration) NewBot(clusterName string, webProxyAddr string) (common.MessagingBot, error) {
	return p.slackConfig.NewBot(clusterName, webProxyAddr)
}
