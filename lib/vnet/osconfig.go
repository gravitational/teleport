// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package vnet

import (
	"context"
	"time"

	"github.com/gravitational/trace"
)

type osConfig struct {
	tunName    string
	tunIPv4    string
	tunIPv6    string
	cidrRanges []string
	dnsAddr    string
	dnsZones   []string
}

func configureOS(ctx context.Context, osConfig *osConfig) error {
	return trace.Wrap(platformConfigureOS(ctx, osConfig))
}

type targetOSConfigProvider interface {
	targetOSConfig(context.Context) (*osConfig, error)
}

type osConfigurator struct {
	targetOSConfigProvider targetOSConfigProvider
}

func newOSConfigurator(targetOSConfigProvider targetOSConfigProvider) *osConfigurator {
	return &osConfigurator{
		targetOSConfigProvider: targetOSConfigProvider,
	}
}

func (c *osConfigurator) updateOSConfiguration(ctx context.Context) error {
	desiredOSConfig, err := c.targetOSConfigProvider.targetOSConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := configureOS(ctx, desiredOSConfig); err != nil {
		return trace.Wrap(err, "configuring OS")
	}
	return nil
}

func (c *osConfigurator) deconfigureOS(ctx context.Context) error {
	// configureOS is meant to be called with an empty config to deconfigure anything necessary.
	return trace.Wrap(configureOS(ctx, &osConfig{}))
}

// osConfigurationLoop will keep running until ctx is canceled or an unrecoverable error is encountered, in
// order to keep the host OS configuration up to date.
func (c *osConfigurator) runOSConfigurationLoop(ctx context.Context) error {
	// Clean up any stale configuration left by a previous VNet instance that
	// may have failed to clean up. This is necessary in case any stale DNS
	// configuration is still present, we need to make sure the proxy is
	// reachable without hitting the VNet DNS resolver which is not ready yet.
	if err := c.deconfigureOS(ctx); err != nil {
		return trace.Wrap(err, "cleaning up OS configuration on startup")
	}

	if err := c.updateOSConfiguration(ctx); err != nil {
		return trace.Wrap(err, "applying initial OS configuration")
	}

	defer func() {
		// Shutting down, deconfigure OS. Pass context.Background because ctx has likely been canceled
		// already but we still need to clean up.
		if err := c.deconfigureOS(context.Background()); err != nil {
			log.ErrorContext(ctx, "Error deconfiguring host OS before shutting down.", "error", err)
		}
	}()

	// Re-configure the host OS every 10 seconds to pick up any newly logged-in
	// clusters or updated DNS zones or CIDR ranges.
	const osConfigurationInterval = 10 * time.Second
	tick := time.Tick(osConfigurationInterval)
	for {
		select {
		case <-tick:
			if err := c.updateOSConfiguration(ctx); err != nil {
				return trace.Wrap(err, "updating OS configuration")
			}
		case <-ctx.Done():
			return trace.Wrap(ctx.Err(), "context canceled, shutting down os configuration loop")
		}
	}
}
