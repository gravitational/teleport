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

package automaticupgrades

import (
	"context"
	"net/url"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/maintenance"
	"github.com/gravitational/teleport/integrations/kube-agent-updater/pkg/version"
)

// Channels is a map of Channel objects.
type Channels map[string]*Channel

// CheckAndSetDefaults checks that every Channel is valid and initializes them.
func (c Channels) CheckAndSetDefaults() error {
	var errs []error
	var err error
	for name, channel := range c {
		// Wrapping is not mandatory here, but it adds the channel name in the
		// error, which makes troubleshooting easier.
		err = trace.Wrap(channel.CheckAndSetDefaults(), "failed to create channel %s", name)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return trace.NewAggregate(errs...)
}

// Channel describes an automatic update channel configuration.
// It can be configured to serve a static version, or forward version requests
// to an upstream version server. Forwarded results are cached for 1 minute.
type Channel struct {
	// ForwardURL is the URL of the upstream version server providing the channel version/criticality.
	ForwardURL string `yaml:"forward_url,omitempty"`
	// StaticVersion is a static version the channel must serve. With or without the leading "v".
	StaticVersion string `yaml:"static_version,omitempty"`
	// Critical is whether the static version channel should be marked as a critical update.
	Critical bool `yaml:"critical"`

	// versionGetter gets the version of the channel. It is populated by CheckAndSetDefaults.
	versionGetter version.Getter
	// criticalTrigger gets the criticality of the channel. It is populated by CheckAndSetDefaults.
	criticalTrigger maintenance.Trigger
}

// CheckAndSetDefaults checks that the Channel configuration is valid and inits
// the version getter and maintenance trigger of the Channel based on its
// configuration. This function must be called before using the channel.
func (c *Channel) CheckAndSetDefaults() error {
	switch {
	case c.ForwardURL != "" && (c.StaticVersion != "" || c.Critical):
		return trace.BadParameter("cannot set both ForwardURL and (StaticVersion or Critical)")
	case c.ForwardURL != "":
		baseURL, err := url.Parse(c.ForwardURL)
		if err != nil {
			return trace.Wrap(err)
		}
		c.versionGetter = version.NewBasicHTTPVersionGetter(baseURL)
		c.criticalTrigger = maintenance.NewBasicHTTPMaintenanceTrigger("remote", baseURL)
	case c.StaticVersion != "":
		c.versionGetter = version.NewStaticGetter(c.StaticVersion, nil)
		c.criticalTrigger = maintenance.NewMaintenanceStaticTrigger("remote", c.Critical)
	default:
		return trace.BadParameter("either ForwardURL or StaticVersion must be set")
	}
	return nil
}

// GetVersion returns the current version of the channel. If io is involved,
// this function implements cache and is safe to call frequently.
func (c *Channel) GetVersion(ctx context.Context) (string, error) {
	return c.versionGetter.GetVersion(ctx)
}

// GetCritical returns the current criticality of the channel. If io is involved,
// this function implements cache and is safe to call frequently.
func (c *Channel) GetCritical(ctx context.Context) (bool, error) {
	return c.criticalTrigger.CanStart(ctx, nil)
}
