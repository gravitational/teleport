/*
Copyright 2020-2021 Gravitational, Inc.

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

package pagerduty

import (
	"strings"

	"github.com/gravitational/trace"
	"github.com/pelletier/go-toml"

	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

type Config struct {
	Teleport  lib.TeleportConfig `toml:"teleport"`
	Pagerduty PagerdutyConfig    `toml:"pagerduty"`
	Log       logger.Config      `toml:"log"`
}

type PagerdutyConfig struct {
	APIEndpoint        string `toml:"-"`
	APIKey             string `toml:"api_key"`
	UserEmail          string `toml:"user_email"`
	RequestAnnotations struct {
		NotifyService string `toml:"notify_service"`
		Services      string `toml:"services"`
	}
}

const NotifyServiceDefaultAnnotation = "pagerduty_notify_service"
const ServicesDefaultAnnotation = "pagerduty_services"

const exampleConfig = `# example teleport-pagerduty configuration TOML file
[teleport]
# Teleport Auth/Proxy Server address.
#
# Should be port 3025 for Auth Server and 3080 or 443 for Proxy.
# For Teleport Cloud, should be in the form "your-account.teleport.sh:443".
addr = "example.com:3025"

# Credentials.
#
# When using --format=file:
# identity = "/var/lib/teleport/plugins/pagerduty/auth_id"    # Identity file
#
# When using --format=tls:
# client_key = "/var/lib/teleport/plugins/pagerduty/auth.key" # Teleport TLS secret key
# client_crt = "/var/lib/teleport/plugins/pagerduty/auth.crt" # Teleport TLS certificate
# root_cas = "/var/lib/teleport/plugins/pagerduty/auth.cas"   # Teleport CA certs

[pagerduty]
api_key = "key"               # PagerDuty API Key
user_email = "me@example.com" # PagerDuty bot user email (Could be admin email)

[log]
output = "stderr" # Logger output. Could be "stdout", "stderr" or "/var/lib/teleport/pagerduty.log"
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
	if strings.HasPrefix(conf.Pagerduty.APIKey, "/") {
		conf.Pagerduty.APIKey, err = lib.ReadPassword(conf.Pagerduty.APIKey)
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
	if c.Pagerduty.APIKey == "" {
		return trace.BadParameter("missing required value pagerduty.api_key")
	}
	if c.Pagerduty.UserEmail == "" {
		return trace.BadParameter("missing required value pagerduty.user_email")
	}
	if c.Pagerduty.RequestAnnotations.NotifyService == "" {
		c.Pagerduty.RequestAnnotations.NotifyService = NotifyServiceDefaultAnnotation
	}
	if c.Pagerduty.RequestAnnotations.Services == "" {
		c.Pagerduty.RequestAnnotations.Services = ServicesDefaultAnnotation
	}
	if c.Log.Output == "" {
		c.Log.Output = "stderr"
	}
	if c.Log.Severity == "" {
		c.Log.Severity = "info"
	}
	return nil
}
