/*
Copyright 2023 Gravitational, Inc.

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

package opsgenie

import (
	"net/url"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/auth"
)

// Config stores the full configuration for the teleport-opsgenie plugin to run.
type Config struct {
	common.BaseConfig
	// Opsgenie contains the opsgenie specific configuration.
	Opsgenie common.GenericAPIConfig
	// ClientConfig contains the config for the opsgenie client.
	ClientConfig ClientConfig
	// AccessTokenProvider provides a method to get the bearer token
	// for use when authorizing to a 3rd-party provider API.
	AccessTokenProvider auth.AccessTokenProvider
	// StatusSink defines a destination for PluginStatus
	StatusSink common.StatusSink
}

// CheckAndSetDefaults checks the config struct for any logical errors, and sets default values
// if some values are missing.
// If critical values are missing and we can't set defaults for them, this will return an error.
func (c *Config) CheckAndSetDefaults() error {
	if err := c.Teleport.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if c.AccessTokenProvider == nil {
		if c.Opsgenie.Token == "" {
			return trace.BadParameter("missing required value opsgenie.token")
		}
		c.AccessTokenProvider = auth.NewStaticAccessTokenProvider(c.Opsgenie.Token)
	} else {
		if c.Opsgenie.Token != "" {
			return trace.BadParameter("exactly one of opsgenie.token and AccessTokenProvider must be set")
		}
	}

	if c.Log.Output == "" {
		c.Log.Output = "stderr"
	}
	if c.Log.Severity == "" {
		c.Log.Severity = "info"
	}

	c.PluginType = types.PluginTypeOpsgenie
	return nil
}

// NewBot initializes the new Opsgenie message generator (OpsgenieBot)
func (c *Config) NewBot(clusterName, webProxyAddr string) (common.MessagingBot, error) {
	webProxyURL, err := url.Parse(webProxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.ClientConfig.WebProxyURL = webProxyURL
	c.ClientConfig.ClusterName = clusterName
	c.ClientConfig.APIKey = c.Opsgenie.Token
	client, err := NewClient(c.ClientConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Bot{
		client:      client,
		clusterName: clusterName,
		webProxyURL: webProxyURL,
	}, nil
}
