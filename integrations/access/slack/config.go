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
	Review              ReviewConfig
	AccessTokenProvider auth.AccessTokenProvider `json:"-"`
	AppTokenProvider    auth.AccessTokenProvider `json:"-"`
	StatusSink          common.StatusSink
	Clock               clockwork.Clock
}

type ReviewConfig struct {
	Enabled bool `toml:"enabled"`
	// AppToken is the app-level token used for Slack Socket Mode API.
	AppToken string `toml:"app_token"`
	// SlackUserIDTrait is the name of the Teleport trait to resolve Slack user to Teleport user.
	SlackUserIDTrait string `toml:"slack_user_id_trait"`
	// AllowEmailUsernameMatch allows exact Slack email to Teleport username match
	// for Slack user resolution.
	AllowEmailUsernameMatch bool `toml:"allow_email_username_match"`
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

	if strings.HasPrefix(conf.Review.AppToken, "/") {
		conf.Review.AppToken, err = lib.ReadPassword(conf.Review.AppToken)
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

	if c.Slack.APIURL == "" {
		c.Slack.APIURL = slackAPIURL
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

	// Native review config defaults.
	if c.Review.Enabled {
		if c.AppTokenProvider == nil {
			if c.Review.AppToken == "" {
				return trace.BadParameter("missing required value review.app_token")
			}
			c.AppTokenProvider = auth.NewStaticAccessTokenProvider(c.Review.AppToken)
		} else {
			if c.Review.AppToken != "" {
				return trace.BadParameter("exactly one of review.app_token and AppTokenProvider must be set")
			}
		}

		// Consider `review.slack_user_id_trait` a critical value that should return error if missing,
		// since it governs Slack user to Teleport user binding.
		// However, if user sets `review.allow_email_username_match` to true, and the field is unset,
		// we assume no trait is set up in Teleport and user wants the fallback logic.
		if c.Review.SlackUserIDTrait == "" && !c.Review.AllowEmailUsernameMatch {
			return trace.BadParameter("missing required value review.slack_user_id_trait")
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

	client := makeSlackClient(c.Slack.APIURL).
		OnBeforeRequest(func(_ *resty.Client, r *resty.Request) error {
			botToken, err := c.AccessTokenProvider.GetAccessToken()
			if err != nil {
				return trace.Wrap(err)
			}
			r.SetAuthToken(botToken)
			return nil
		}).
		OnAfterResponse(onAfterResponseSlack(c.StatusSink))

	// For native review, we require a client with a separate auth header using the app-level token.
	appClient := makeSlackClient(c.Slack.APIURL).
		OnBeforeRequest(func(_ *resty.Client, r *resty.Request) error {
			appToken, err := c.AppTokenProvider.GetAccessToken()
			if err != nil {
				return trace.Wrap(err)
			}
			r.SetAuthToken(appToken)
			return nil
		}).
		OnAfterResponse(onAfterResponseSlack(c.StatusSink))

	return Bot{
		client:       client,
		appClient:    appClient,
		clusterName:  clusterName,
		webProxyURL:  webProxyURL,
		reviewConfig: c.Review,
		clock:        c.Clock,
	}, nil
}
