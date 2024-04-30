/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
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

func (w *wrappedClient) ListAccessLists(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessList, string, error) {
	return w.Client.AccessListClient().ListAccessLists(ctx, pageSize, pageToken)
}

// ListAccessMonitoringRules lists current access monitoring rules.
func (w *wrappedClient) ListAccessMonitoringRules(ctx context.Context, limit int, startKey string) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error) {
	return w.Client.AccessMonitoringRulesClient().ListAccessMonitoringRules(ctx, limit, startKey)
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
