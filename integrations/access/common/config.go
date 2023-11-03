/*
Copyright 2022 Gravitational, Inc.

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

package common

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/lib/services"
)

type PluginConfiguration interface {
	GetTeleportClient(ctx context.Context) (teleport.Client, error)
	GetRecipients() RawRecipientsMap
	NewBot(clusterName string, webProxyAddr string) (MessagingBot, error)
	GetPluginType() types.PluginType
}

type BaseConfig struct {
	Teleport   lib.TeleportConfig `toml:"teleport"`
	Recipients RawRecipientsMap   `toml:"role_to_recipients"`
	Log        logger.Config      `toml:"log"`
	PluginType types.PluginType
}

func (c BaseConfig) GetRecipients() RawRecipientsMap {
	return c.Recipients
}

type wrappedClient struct {
	*client.Client
}

func (w *wrappedClient) AccessListClient() services.AccessLists {
	return w.Client.AccessListClient()
}

// wrapAPIClient will wrap the API client such that it conforms to the Teleport plugin client interface.
func wrapAPIClient(clt *client.Client) teleport.Client {
	return &wrappedClient{
		Client: clt,
	}
}

// GetTeleportClient will return a Teleport plugin client given a config.
func GetTeleportClient(ctx context.Context, conf lib.TeleportConfig) (teleport.Client, error) {
	clt, err := conf.NewClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return wrapAPIClient(clt), nil
}

// GetTeleportClient returns a Teleport plugin client for the given config.
func (c BaseConfig) GetTeleportClient(ctx context.Context) (teleport.Client, error) {
	return GetTeleportClient(ctx, c.Teleport)
}

// GetPluginType returns the type of plugin this config is for.
func (c BaseConfig) GetPluginType() types.PluginType {
	return c.PluginType
}

// GenericAPIConfig holds common configuration use by a messaging service.
// MessagingBots requiring more custom configuration (MSTeams for example) can
// implement their own APIConfig instead.
type GenericAPIConfig struct {
	Token string
	// DELETE IN 11.0.0 (Joerger) - use "role_to_recipients["*"]" instead
	Recipients []string
	APIURL     string
}
