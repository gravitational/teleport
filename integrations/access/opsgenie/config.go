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

package opsgenie

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/auth"
)

// Config stores the full configuration for the teleport-opsgenie plugin to run.
type Config struct {
	common.BaseConfig
	Opsgenie            common.GenericAPIConfig
	ClientConfig        ClientConfig
	AccessTokenProvider auth.AccessTokenProvider
	StatusSink          common.StatusSink
}

// LoadOpsgenieConfig reads the config file, initializes a new OpsgenieConfig struct object, and returns it.
// Optionally returns an error if the file is not readable, or if file format is invalid.
func LoadOpsgenieConfig(filepath string) (*Config, error) {
	return nil, nil
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

	if len(c.Recipients) == 0 {
		return trace.BadParameter("missing required value role_to_recipients.")
	} else if len(c.Recipients[types.Wildcard]) == 0 {
		return trace.BadParameter("missing required value role_to_recipients[%v].", types.Wildcard)
	}

	return nil
}

// NewBot initializes the new Opsgenie message generator (OpsgenieBot)
func (c *Config) NewBot(clusterName, webProxyAddr string) (common.MessagingBot, error) {
	client, err := NewClient(c.ClientConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Bot{
		client:      client,
		clusterName: c.ClientConfig.ClusterName,
		webProxyURL: c.ClientConfig.WebProxyURL,
	}, nil
}
