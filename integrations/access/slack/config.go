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

package slack

import (
	"net/url"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
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
	Clock               clockwork.Clock
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

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	c.PluginType = types.PluginTypeSlack
	return nil
}

// NewBot initializes the new Slack message generator (SlackBot)
// takes GenericAPIConfig as an argument.
func (c *Config) NewBot(clusterName, webProxyAddr string) (common.MessagingBot, error) {
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	var (
		webProxyURL *url.URL
		err         error
	)
	if webProxyAddr != "" {
		if webProxyURL, err = lib.AddrToURL(webProxyAddr); err != nil {
			return Bot{clock: c.Clock}, trace.Wrap(err)
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
		clock:       c.Clock,
		clusterName: clusterName,
		webProxyURL: webProxyURL,
	}, nil
}
