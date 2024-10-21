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

package discord

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/gravitational/trace"
	"github.com/pelletier/go-toml"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
)

const discordAPIUrl = "https://discord.com/api/"

type Config struct {
	common.BaseConfig
	Discord common.GenericAPIConfig

	// Teleport is a handle to the client to use when communicating with
	// the Teleport auth server. The PagerDuty app will create a GRPC-
	// based client on startup if this is not set.
	Client teleport.Client

	// StatusSink receives any status updates from the plugin for
	// further processing. Status updates will be ignored if not set.
	StatusSink common.StatusSink
}

// CheckAndSetDefaults checks the config struct for any logical errors, and sets default values
// if some values are missing.
// If critical values are missing and we can't set defaults for them — this will return an error.
func (c *Config) CheckAndSetDefaults() error {
	if err := c.Teleport.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if c.Discord.Token == "" {
		return trace.BadParameter("missing required value discord.token")
	}
	if c.Discord.APIURL == "" {
		c.Discord.APIURL = discordAPIUrl
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

	return nil
}

// GetTeleportClient implements PluginConfiguration. If a pre-created client
// was supplied on construction, this method will return that. If not, an RPC
// client will be created  using the values in the config.
func (c *Config) GetTeleportClient(ctx context.Context) (teleport.Client, error) {
	if c.Client != nil {
		return c.Client, nil
	}
	return c.BaseConfig.GetTeleportClient(ctx)
}

// NewBot initializes the new Discord message generator (DiscordBot)
func (c *Config) NewBot(clusterName, webProxyAddr string) (common.MessagingBot, error) {
	var (
		webProxyURL *url.URL
		err         error
	)
	if webProxyAddr != "" {
		if webProxyURL, err = lib.AddrToURL(webProxyAddr); err != nil {
			return DiscordBot{}, trace.Wrap(err)
		}
	}

	token := "Bot " + c.Discord.Token

	client := resty.
		NewWithClient(&http.Client{
			Timeout: discordHTTPTimeout,
			Transport: &http.Transport{
				MaxConnsPerHost:     discordMaxConns,
				MaxIdleConnsPerHost: discordMaxConns,
				Proxy:               http.ProxyFromEnvironment,
			},
		}).
		SetBaseURL(c.Discord.APIURL).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", token).
		OnAfterResponse(onAfterResponseDiscord(c.StatusSink))

	return DiscordBot{
		client:      client,
		clusterName: clusterName,
		webProxyURL: webProxyURL,
	}, nil
}

// LoadDiscordConfig reads the config file, initializes a new Discord Config
// struct object, and returns it. Optionally returns an error if the file is
// not readable, or if file format is invalid.
func LoadDiscordConfig(filepath string) (*Config, error) {
	t, err := toml.LoadFile(filepath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conf := &Config{}
	if err := t.Unmarshal(conf); err != nil {
		return nil, trace.Wrap(err)
	}

	if strings.HasPrefix(conf.Discord.Token, "/") {
		conf.Discord.Token, err = lib.ReadPassword(conf.Discord.Token)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if err := conf.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return conf, nil
}
