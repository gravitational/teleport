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
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/auth"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/trace"
	"github.com/pelletier/go-toml"
)

const discordAPIUrl = "https://discord.com/api/"

type DiscordConfig struct {
	common.BaseConfig
	AccessTokenProvider auth.AccessTokenProvider
	Discord             common.GenericAPIConfig
	StatusSink          common.StatusSink
}

// LoadDiscordConfig reads the config file, initializes a new DiscordConfig struct object, and returns it.
// Optionally returns an error if the file is not readable, or if file format is invalid.
func LoadDiscordConfig(filepath string) (*DiscordConfig, error) {
	t, err := toml.LoadFile(filepath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conf := &DiscordConfig{}
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

// CheckAndSetDefaults checks the config struct for any logical errors, and sets default values
// if some values are missing.
// If critical values are missing and we can't set defaults for them — this will return an error.
func (c *DiscordConfig) CheckAndSetDefaults() error {
	if err := c.Teleport.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if c.Discord.Token == "" {
		return trace.BadParameter("missing required value discord.token")
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

// NewBot initializes the new Discord message generator (DiscordBot)
func (c *DiscordConfig) NewBot(clusterName, webProxyAddr string) (common.MessagingBot, error) {
	var (
		webProxyURL *url.URL
		err         error
	)
	if webProxyAddr != "" {
		if webProxyURL, err = lib.AddrToURL(webProxyAddr); err != nil {
			return DiscordBot{}, trace.Wrap(err)
		}
	}

	//	token := "Bot " + c.Discord.Token

	client := resty.
		NewWithClient(&http.Client{
			Timeout: discordHTTPTimeout,
			Transport: &http.Transport{
				MaxConnsPerHost:     discordMaxConns,
				MaxIdleConnsPerHost: discordMaxConns,
			},
		}).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		OnBeforeRequest(func(_ *resty.Client, r *resty.Request) error {
			token, err := c.AccessTokenProvider.GetAccessToken()
			if err != nil {
				return trace.Wrap(err)
			}
			fmt.Println("=== ACCESS TOKEN ===", token)
			r.SetHeader("Authorization", "Bot "+token)
			return nil
		})

	// APIURL parameter is set only in tests
	if endpoint := c.Discord.APIURL; endpoint != "" {
		client.SetBaseURL(endpoint)
	} else {
		client.SetBaseURL(discordAPIUrl)
		client.OnAfterResponse(onAfterResponseDiscord)
	}

	return DiscordBot{
		client:      client,
		clusterName: clusterName,
		webProxyURL: webProxyURL,
	}, nil
}
