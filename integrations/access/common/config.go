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
	"github.com/gravitational/teleport/api/client/accesslist"
	"github.com/gravitational/teleport/api/client/accessmonitoringrules"
	"github.com/gravitational/teleport/api/client/userloginstate"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

type PluginConfiguration interface {
	GetTeleportClient(ctx context.Context) (teleport.Client, error)
	GetRecipients() RawRecipientsMap
	NewBot(clusterName string, webProxyAddr string) (MessagingBot, error)
	GetPluginType() types.PluginType
	// GetTeleportUser returns the name of the teleport user that acts as the
	// access request approver.
	GetTeleportUser() string
}

type BaseConfig struct {
	Teleport   lib.TeleportConfig `toml:"teleport"`
	Recipients RawRecipientsMap   `toml:"role_to_recipients"`
	Log        logger.Config      `toml:"log"`
	PluginType types.PluginType
	// TeleportUser is the name of the teleport user that acts as the
	// access request approver.
	TeleportUser string
}

func (c BaseConfig) GetRecipients() RawRecipientsMap {
	return c.Recipients
}

// client type alias are used to embed the types in the wrappedClient

type userLoginStateClient = *userloginstate.Client
type accessListClient = *accesslist.Client
type accessMonitoringRulesClient = *accessmonitoringrules.Client

type wrappedClient struct {
	*client.Client
	userLoginStateClient
	accessListClient
	accessMonitoringRulesClient
}

// wrapAPIClient will wrap the API client such that it conforms to the Teleport plugin client interface.
func wrapAPIClient(clt *client.Client) teleport.Client {
	return &wrappedClient{
		Client:                      clt,
		userLoginStateClient:        clt.UserLoginStateClient(),
		accessListClient:            clt.AccessListClient(),
		accessMonitoringRulesClient: clt.AccessMonitoringRulesClient(),
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

// GetTeleportUser returns the name of the teleport user that acts as the
// access request approver.
func (c BaseConfig) GetTeleportUser() string {
	return c.TeleportUser
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
