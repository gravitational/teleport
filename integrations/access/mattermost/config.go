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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/trace"
	"github.com/pelletier/go-toml"
)

type Config struct {
	common.BaseConfig
	Mattermost MattermostConfig
	StatusSink common.StatusSink
}

type MattermostConfig struct {
	URL        string   `toml:"url"`
	Recipients []string `toml:"recipients"`
	Token      string   `toml:"token"`
}

const exampleConfig = `# example mattermost configuration TOML file
 [teleport]
 # Teleport Auth/Proxy Server address.
 #
 # Should be port 3025 for Auth Server and 3080 or 443 for Proxy.
 # For Teleport Cloud, should be in the form "your-account.teleport.sh:443".
 addr = "example.com:3025"
 
 # Credentials.
 #
 # When using --format=file:
 # identity = "/var/lib/teleport/plugins/mattermost/auth_id"    # Identity file
 #
 # When using --format=tls:
 # client_key = "/var/lib/teleport/plugins/mattermost/auth.key" # Teleport TLS secret key
 # client_crt = "/var/lib/teleport/plugins/mattermost/auth.crt" # Teleport TLS certificate
 # root_cas = "/var/lib/teleport/plugins/mattermost/auth.cas"   # Teleport CA certs
 
 [mattermost]
 url = "https://mattermost.example.com" # Mattermost Server URL
 token = "api-token"                    # Mattermost Bot OAuth token

 # Notify recipients
 #
 # The value is an array of strings, where each element is either:
 # - A channel name in the format 'team/channel', where / separates the 
 #   name of the team and the name of the channel
 # - The email address of a Mattermost user to notify via a direct message 
 #   when the plugin receives an Access Request event
 recipients = [
  "my-team-name/channel-name",
  "first.last@example.com"
 ]
 
 [log]
 output = "stderr" # Logger output. Could be "stdout", "stderr" or "/var/lib/teleport/mattermost.log"
 severity = "INFO" # Logger severity. Could be "INFO", "ERROR", "DEBUG" or "WARN".
 `

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
	if c.Log.Output == "" {
		c.Log.Output = "stderr"
	}
	if c.Log.Severity == "" {
		c.Log.Severity = "info"
	}

	if len(c.Recipients) == 0 {
		return trace.BadParameter("missing required value mattermost.recipients")
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
