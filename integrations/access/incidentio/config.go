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

package incidentio

import (
	"context"
	"net/url"

	"github.com/gravitational/trace"
	"github.com/pelletier/go-toml"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
)

// Config stores the full configuration for the teleport-incidentio plugin to run.
type Config struct {
	common.BaseConfig
	// Incident contains the incident.io specific configuration.
	Incident common.GenericAPIConfig
	// ClientConfig contains the config for the incident.io client.
	ClientConfig ClientConfig `toml:"client_config"`

	// Teleport is a handle to the client to use when communicating with
	// the Teleport auth server. The ServiceNow app will create a gRPC-based
	// client on startup if this is not set.
	Client teleport.Client
	// TeleportUserName is the name of the Teleport user that will act
	// as the access request approver.
	TeleportUserName string
}

// LoadConfig reads the config file, initializes a new Config struct object, and returns it.
// Optionally returns an error if the file is not readable, or if file format is invalid.
func LoadConfig(filepath string) (*Config, error) {
	t, err := toml.LoadFile(filepath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	conf := &Config{}
	if err := t.Unmarshal(conf); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := conf.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return conf, nil
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

	if c.Log.Output == "" {
		c.Log.Output = "stderr"
	}
	if c.Log.Severity == "" {
		c.Log.Severity = "info"
	}

	c.PluginType = types.PluginTypeIncidentio
	return nil
}

// GetTeleportClient returns the configured Teleport client.
func (c *Config) GetTeleportClient(ctx context.Context) (teleport.Client, error) {
	if c.Client != nil {
		return c.Client, nil
	}
	return c.BaseConfig.GetTeleportClient(ctx)
}

// NewBot initializes the new incident.io message generator (Incidentbot)
func (c *Config) NewBot(clusterName, webProxyAddr string) (common.MessagingBot, error) {
	webProxyURL, err := url.Parse(webProxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.ClientConfig.WebProxyURL = webProxyURL
	c.ClientConfig.ClusterName = clusterName
	apiClient, err := NewAPIClient(c.ClientConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	alertClient, err := NewAlertClient(c.ClientConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Bot{
		apiClient:   apiClient,
		alertClient: alertClient,
		clusterName: clusterName,
		webProxyURL: webProxyURL,
	}, nil
}
