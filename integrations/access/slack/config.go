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

package slack

import (
	"net/url"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/gravitational/trace"
	"github.com/pelletier/go-toml"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/auth"
	"github.com/gravitational/teleport/integrations/lib"
)

// Config stores the full configuration for the teleport-slack plugin to run.
type Config struct {
	common.BaseConfig
	Slack               common.GenericAPIConfig
	AccessTokenProvider auth.AccessTokenProvider
	StatusSink          common.StatusSink
}

// LoadSlackConfig reads the config file, initializes a new SlackConfig struct object, and returns it.
// Optionally returns an error if the file is not readable, or if file format is invalid.
func LoadSlackConfig(filepath string) (*Config, error) {
	t, err := toml.LoadFile(filepath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conf := &Config{}
	if err := t.Unmarshal(conf); err != nil {
		return nil, trace.Wrap(err)
	}

	if strings.HasPrefix(conf.Slack.Token, "/") {
		conf.Slack.Token, err = lib.ReadPassword(conf.Slack.Token)
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
// If critical values are missing and we can't set defaults for them, this will return an error.
func (c *Config) CheckAndSetDefaults() error {
	if err := c.Teleport.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if c.AccessTokenProvider == nil {
		if c.Slack.Token == "" {
			return trace.BadParameter("missing required value slack.token")
		}
		c.AccessTokenProvider = auth.NewStaticAccessTokenProvider(c.Slack.Token)
	} else {
		if c.Slack.Token != "" {
			return trace.BadParameter("exactly one of slack.token and AccessTokenProvider must be set")
		}
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
	c.PluginType = types.PluginTypeSlack
	return nil
}

// NewBot initializes the new Slack message generator (SlackBot)
// takes GenericAPIConfig as an argument.
func (c *Config) NewBot(clusterName, webProxyAddr string) (common.MessagingBot, error) {
	var (
		webProxyURL *url.URL
		err         error
	)
	if webProxyAddr != "" {
		if webProxyURL, err = lib.AddrToURL(webProxyAddr); err != nil {
			return Bot{}, trace.Wrap(err)
		}
	}

	var apiURL = slackAPIURL
	if endpoint := c.Slack.APIURL; endpoint != "" {
		apiURL = endpoint
	}

	client := makeSlackClient(apiURL).
		OnBeforeRequest(func(_ *resty.Client, r *resty.Request) error {
			token, err := c.AccessTokenProvider.GetAccessToken()
			if err != nil {
				return trace.Wrap(err)
			}
			r.SetHeader("Authorization", "Bearer "+token)
			return nil
		}).
		OnAfterResponse(onAfterResponseSlack(c.StatusSink))

	return Bot{
		client:      client,
		clusterName: clusterName,
		webProxyURL: webProxyURL,
	}, nil
}
