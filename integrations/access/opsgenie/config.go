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

package opsgenie

import (
	"context"
	"net/url"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/auth"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
)

// Config stores the full configuration for the teleport-opsgenie plugin to run.
type Config struct {
	common.BaseConfig
	// Opsgenie contains the opsgenie specific configuration.
	Opsgenie common.GenericAPIConfig
	// ClientConfig contains the config for the opsgenie client.
	ClientConfig ClientConfig
	// AccessTokenProvider provides a method to get the bearer token
	// for use when authorizing to a 3rd-party provider API.
	AccessTokenProvider auth.AccessTokenProvider

	// Teleport is a handle to the client to use when communicating with
	// the Teleport auth server. The ServiceNow app will create a gRPC-based
	// client on startup if this is not set.
	Client teleport.Client
	// TeleportUserName is the name of the Teleport user that will act
	// as the access request approver.
	TeleportUserName string
}

// CheckAndSetDefaults checks the config struct for any logical errors, and sets default values
// if some values are missing.
// If critical values are missing and we can't set defaults for them, this will return an error.
func (c *Config) CheckAndSetDefaults() error {
	if err := c.Teleport.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if err := c.ClientConfig.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if c.AccessTokenProvider == nil {
		if c.Opsgenie.Token == "" {
			return trace.BadParameter("missing required value opsgenie.token")
		}
		c.AccessTokenProvider = auth.NewStaticAccessTokenProvider(c.Opsgenie.Token)
	} else {
		if c.Opsgenie.Token != "" {
			return trace.BadParameter("exactly one of opsgenie.token and AccessTokenProvider must be set")
		}
	}

	if c.Log.Output == "" {
		c.Log.Output = "stderr"
	}
	if c.Log.Severity == "" {
		c.Log.Severity = "info"
	}

	c.PluginType = types.PluginTypeOpsgenie
	return nil
}

// GetTeleportClient returns the configured Teleport client.
func (c *Config) GetTeleportClient(ctx context.Context) (teleport.Client, error) {
	if c.Client != nil {
		return c.Client, nil
	}
	return c.BaseConfig.GetTeleportClient(ctx)
}

// NewBot initializes the new Opsgenie message generator (OpsgenieBot)
func (c *Config) NewBot(clusterName, webProxyAddr string) (common.MessagingBot, error) {
	webProxyURL, err := url.Parse(webProxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.ClientConfig.WebProxyURL = webProxyURL
	c.ClientConfig.ClusterName = clusterName
	client, err := NewClient(c.ClientConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Bot{
		client:      client,
		clusterName: clusterName,
		webProxyURL: webProxyURL,
	}, nil
}
