/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

package datadog

import (
	"context"
	"strings"

	"github.com/gravitational/trace"
	"github.com/pelletier/go-toml"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
)

const (
	// APIEndpointDefaultURL specifies the default US1 region api endpoint.
	APIEndpointDefaultURL = "https://api.datadoghq.com"
	// APIVersion specifies the api version.
	APIVersion = "api/v2"
	// APIUnstable specifies the unstable api endpoint.
	//
	// TODO: Remove once on-call API is merged into official API.
	APIUnstable = "api/unstable"
	// SeverityDefault specifies the default incident severity.
	SeverityDefault = "SEV-3"
)

// Config stores the full configuration for the teleport-datadog plugin to run.
type Config struct {
	// BaseConfig specifies the base configuration for an access plugin.
	common.BaseConfig

	// Datadog specifies Datadog API client configuration
	Datadog DatadogConfig

	// StatusSink receives any status updates from the plugin for
	// further processing. Status updates will be ignored if not set.
	StatusSink common.StatusSink

	// Teleport is a handle to the client to use when communicating with
	// the Teleport auth server. The Datadog app will create a gRPC-based
	// client on startup if this is not set.
	Client teleport.Client
}

// DatadogConfig stores datadog specifc configuration.
type DatadogConfig struct {
	// APIEndpoint specifies the Datadog API endpoint.
	APIEndpoint string `toml:"api_endpoint"`
	// APIKey specifies a Datadog API key.
	APIKey string `toml:"api_key"`
	// ApplicationKey specifies a Datadog Application key.
	ApplicationKey string `toml:"application_key"`
	// Severity configures the incident severity. Default is 'SEV-3'.
	Severity string `toml:"severity"`
}

// LoadConfig loads configuration from specified filepath.
func LoadConfig(filepath string) (*Config, error) {
	t, err := toml.LoadFile(filepath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	conf := &Config{}
	if err := t.Unmarshal(conf); err != nil {
		return nil, trace.Wrap(err)
	}
	if strings.HasPrefix(conf.Datadog.APIKey, "/") {
		conf.Datadog.APIKey, err = lib.ReadPassword(conf.Datadog.APIKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if strings.HasPrefix(conf.Datadog.ApplicationKey, "/") {
		conf.Datadog.ApplicationKey, err = lib.ReadPassword(conf.Datadog.ApplicationKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if err := conf.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return conf, nil
}

// CheckAndSetDefaults checks the config and sets defaults.
func (c *Config) CheckAndSetDefaults() error {
	if err := c.Teleport.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if c.Datadog.APIEndpoint == "" {
		c.Datadog.APIEndpoint = APIEndpointDefaultURL
	}
	if c.Datadog.APIKey == "" {
		return trace.BadParameter("missing required value datadog.api_key")
	}
	if c.Datadog.ApplicationKey == "" {
		return trace.BadParameter("missing required value datadog.application_key")
	}
	if c.Datadog.Severity == "" {
		c.Datadog.Severity = SeverityDefault
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
	c.PluginType = types.PluginTypeDatadog
	return nil
}

// GetTeleportClient returns the configured Teleport client.
func (c *Config) GetTeleportClient(ctx context.Context) (teleport.Client, error) {
	if c.Client != nil {
		return c.Client, nil
	}
	return c.BaseConfig.GetTeleportClient(ctx)
}

// NewBot initializes a new Datadog bot.
func (c *Config) NewBot(clusterName, webProxyAddr string) (common.MessagingBot, error) {
	datadog, err := NewDatadogClient(c.Datadog, webProxyAddr, c.StatusSink)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	webProxyURL, err := lib.AddrToURL(webProxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return Bot{
		datadog:     datadog,
		clusterName: clusterName,
		webProxyURL: webProxyURL,
	}, nil
}
