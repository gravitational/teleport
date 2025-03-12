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
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/mod/semver"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/automaticupgrades/maintenance"
	"github.com/gravitational/teleport/lib/automaticupgrades/version"
)

const (
	DefaultChannelName        = "default"
	DefaultCloudChannelName   = "stable/cloud"
	stableCloudVersionBaseURL = "https://updates.releases.teleport.dev/v1/stable/cloud"
)

// Channels is a map of Channel objects.
type Channels map[string]*Channel

// CheckAndSetDefaults checks that every Channel is valid and initializes them.
// It also creates default channels if they are not already present.
func (c Channels) CheckAndSetDefaults() error {
	defaultChannel, err := NewDefaultChannel()
	if err != nil {
		return trace.Wrap(err)
	}

	// Create the "default" channel

	// If the default channel is specified in the config, we use it.
	// Else if cloud/stable channel is specified in the config, we use it as default.
	// Else, we build a default channel based on the teleport binary version.
	if _, ok := c[DefaultChannelName]; ok {
		slog.DebugContext(context.Background(), "'default' automatic update channel manually specified, honoring it")
	} else if cloudDefaultChannel, ok := c[DefaultCloudChannelName]; ok {
		slog.DebugContext(context.Background(), "'default' automatic update channel not specified, but 'stable/cloud' is, using the cloud default channel by default")
		c[DefaultChannelName] = cloudDefaultChannel
	} else {
		slog.DebugContext(context.Background(), "'default' automatic update channel not specified, teleport will serve its version by default")
		c[DefaultChannelName] = defaultChannel
	}

	// Create the "stable/cloud" channel

	// At this point, we know that we have a default channel
	// If we don't already have a "stable/cloud" channel we create one based on
	// the default one (for compatibility with old updaters).
	if _, ok := c[DefaultCloudChannelName]; !ok {
		c[DefaultCloudChannelName] = c[DefaultChannelName]
	}

	// Checking each channel. We'll double-check the 'default' one, but
	// channel.CheckAndSetDefaults is idempotent.
	var errs []error
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

// DefaultVersion returns the version served by the default upgrade channel.
func (c Channels) DefaultVersion(ctx context.Context) (string, error) {
	channel, ok := c[DefaultChannelName]
	if !ok {
		return "", trace.NotFound("default version channel not found")
	}
	targetVersion, err := channel.GetVersion(ctx)
	return targetVersion, trace.Wrap(err)
}

// DefaultChannel returns the default upgrade channel.
func (c Channels) DefaultChannel() (*Channel, error) {
	defaultChannel, ok := c[DefaultChannelName]
	if ok && defaultChannel != nil {
		return defaultChannel, nil
	}
	return NewDefaultChannel()
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
	// teleportMajor stores the current teleport major for comparison.
	// This field is initialized during CheckAndSetDefaults.
	teleportMajor int
	// mutex protects versionGetter, criticalTrigger, and teleportMajor
	mutex sync.Mutex
}

// CheckAndSetDefaults checks that the Channel configuration is valid and inits
// the version getter and maintenance trigger of the Channel based on its
// configuration. This function must be called before using the channel.
func (c *Channel) CheckAndSetDefaults() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
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

	var err error
	c.teleportMajor, err = parseMajorFromVersionString(teleport.Version)
	if err != nil {
		return trace.Wrap(err, "failed to process teleport version")
	}

	return nil
}

// GetVersion returns the current version of the channel. If io is involved,
// this function implements cache and is safe to call frequently.
// If the target version major is higher than the Teleport version (the one
// in the Teleport binary, this is usually the proxy version), this function
// returns the Teleport version instead.
// If the version source intentionally did not specify a version, a
// NoNewVersionError is returned.
func (c *Channel) GetVersion(ctx context.Context) (string, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	targetVersion, err := c.versionGetter.GetVersion(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	targetMajor, err := parseMajorFromVersionString(targetVersion)
	if err != nil {
		return "", trace.Wrap(err, "failed to process target version")
	}

	// The target version is officially incompatible with our version,
	// we prefer returning our version rather than having a broken client
	if targetMajor > c.teleportMajor {
		targetVersion, err = version.EnsureSemver(teleport.Version)
		if err != nil {
			return "", trace.Wrap(err, "ensuring current teleport version is semver-compatible")
		}
	}

	return targetVersion, nil
}

// GetCritical returns the current criticality of the channel. If io is involved,
// this function implements cache and is safe to call frequently.
func (c *Channel) GetCritical(ctx context.Context) (bool, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.criticalTrigger.CanStart(ctx, nil)
}

var newDefaultChannel = sync.OnceValues[*Channel, error](
	func() (*Channel, error) {
		var channel *Channel
		if forwardURL := GetChannel(); forwardURL != "" {
			channel = &Channel{
				ForwardURL: forwardURL,
			}
		} else {
			channel = &Channel{
				StaticVersion: teleport.Version,
			}
		}
		if err := channel.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		return channel, nil
	})

// NewDefaultChannel creates a default automatic upgrade channel
// It looks up the TELEPORT_AUTOMATIC_UPGRADES_CHANNEL environment variable for
// backward compatibility, and if not found uses binary version.
// This default channel can be used in the proxy (to back its own version server)
// or in other Teleport processes such as integration services deploying and
// updating teleport agents.
// Pre-release versions such as 1.2.3-testbuild.1 will be served AS-IS.
// If you run a test teleport release, agents will attempt to install
// the same release. If you don't want this to happen, you must set the
// 'default' channel to a static version of your choice.
func NewDefaultChannel() (*Channel, error) {
	return newDefaultChannel()
}

func parseMajorFromVersionString(v string) (int, error) {
	v, err := version.EnsureSemver(v)
	if err != nil {
		return 0, trace.Wrap(err, "invalid semver: %s", v)
	}
	majorStr := semver.Major(v)
	if majorStr == "" {
		return 0, trace.BadParameter("cannot detect version major")
	}

	major, err := strconv.Atoi(strings.TrimPrefix(majorStr, "v"))
	return major, trace.Wrap(err, "cannot convert version major to int")
}
