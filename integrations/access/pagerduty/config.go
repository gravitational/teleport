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

package pagerduty

import (
	"strings"

	"github.com/gravitational/trace"
	"github.com/pelletier/go-toml"

	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

type Config struct {
	Teleport  lib.TeleportConfig `toml:"teleport"`
	Pagerduty PagerdutyConfig    `toml:"pagerduty"`
	Log       logger.Config      `toml:"log"`

	// Teleport is a handle to the client to use when communicating with
	// the Teleport auth server. The PagerDuty app will create a gRPC-
	// based client on startup if this is not set.
	Client teleport.Client

	// StatusSink receives any status updates from the plugin for
	// further processing. Status updates will be ignored if not set.
	StatusSink common.StatusSink

	// TeleportUser is the name of the Teleport user that will act
	// as the access request approver
	TeleportUser string
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

const (
	APIEndpointDefaultURL          = "https://api.pagerduty.com"
	NotifyServiceDefaultAnnotation = "pagerduty_notify_service"
	ServicesDefaultAnnotation      = "pagerduty_services"
)

func (c *Config) CheckAndSetDefaults() error {
	if err := c.Teleport.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if c.Pagerduty.APIEndpoint == "" {
		c.Pagerduty.APIEndpoint = APIEndpointDefaultURL
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
