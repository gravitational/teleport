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

package discord

import (
	"context"
	"net/http"
	"net/url"

	"github.com/go-resty/resty/v2"
	"github.com/gravitational/trace"

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
