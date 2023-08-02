/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package mattermost

import (
	"strings"

	"github.com/gravitational/trace"
	"github.com/pelletier/go-toml"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib"
)

type Config struct {
	common.BaseConfig
	Mattermost MattermostConfig `toml:"mattermost"`
	StatusSink common.StatusSink
}

type MattermostConfig struct {
	URL        string   `toml:"url"`
	Recipients []string `toml:"recipients"` // optional
	Token      string   `toml:"token"`
}

func LoadConfig(filepath string) (*Config, error) {
	t, err := toml.LoadFile(filepath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	conf := &Config{}
	if err := t.Unmarshal(conf); err != nil {
		return nil, trace.Wrap(err)
	}
	if strings.HasPrefix(conf.Mattermost.Token, "/") {
		conf.Mattermost.Token, err = lib.ReadPassword(conf.Mattermost.Token)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if err := conf.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return conf, nil
}

func (c *Config) CheckAndSetDefaults() error {
	if err := c.Teleport.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if c.Mattermost.Token == "" {
		return trace.BadParameter("missing required value mattermost.token")
	}
	if c.Mattermost.URL == "" {
		return trace.BadParameter("missing required value mattermost.url")
	}

	// Optional field.
	if len(c.Mattermost.Recipients) > 0 {
		c.Recipients = common.RawRecipientsMap{
			"*": c.Mattermost.Recipients,
		}
	}

	if c.Log.Output == "" {
		c.Log.Output = "stderr"
	}
	if c.Log.Severity == "" {
		c.Log.Severity = "info"
	}

	c.PluginType = types.PluginTypeMattermost
	return nil
}

func (c *Config) NewBot(clusterName, webProxyAddr string) (common.MessagingBot, error) {
	bot, err := NewBot(*c, clusterName, webProxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return bot, nil
}
