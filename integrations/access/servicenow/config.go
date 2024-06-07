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

package servicenow

import (
	"context"
	"net/url"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
)

// Config stores the full configuration for the teleport-servicenow plugin to run.
type Config struct {
	common.BaseConfig
	ClientConfig
	ServiceNow common.GenericAPIConfig

	// Teleport is a handle to the client to use when communicating with
	// the Teleport auth server. The ServiceNow app will create a gRPC-based
	// client on startup if this is not set.
	Client teleport.Client

	// TeleportUser is the name of the Teleport user that will act
	// as the access request approver
	TeleportUser string
}

// CheckAndSetDefaults checks the config struct for any logical errors, and sets default values
// if some values are missing.
// If critical values are missing and we can't set defaults for them, this will return an error.
func (c *Config) CheckAndSetDefaults() error {
	if err := c.Teleport.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if c.Log.Output == "" {
		c.Log.Output = "stderr"
	}
	if c.Log.Severity == "" {
		c.Log.Severity = "info"
	}

	if len(c.Recipients) == 0 {
		return trace.BadParameter("missing required value role_to_recipients.")
	} else if len(c.Recipients[types.Wildcard]) == 0 {
		return trace.BadParameter("missing required value role_to_recipients[%v].", types.Wildcard)
	}
	c.PluginType = types.PluginTypeServiceNow
	return nil
}

func (c *Config) GetTeleportClient(ctx context.Context) (teleport.Client, error) {
	if c.Client != nil {
		return c.Client, nil
	}
	return c.BaseConfig.GetTeleportClient(ctx)
}

// NewBot initializes the new Servicenow message generator (ServicenowBot)
// takes GenericAPIConfig as an argument.
func (c *Config) NewBot(clusterName, webProxyAddr string) (common.MessagingBot, error) {
	var (
		webProxyURL *url.URL
		err         error
	)
	if webProxyAddr != "" {
		if c.WebProxyURL, err = lib.AddrToURL(webProxyAddr); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if clusterName != "" {
		c.ClusterName = clusterName
	}

	client, err := NewClient(c.ClientConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Bot{
		client:      client,
		webProxyURL: webProxyURL,
	}, nil
}
