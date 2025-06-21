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
	"net"
	"net/netip"
	"os/exec"
	"strings"
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

type osConfigState struct {
	platformOSConfigState platformOSConfigState
}

func configureOS(ctx context.Context, osConfig *osConfig, osConfigState *osConfigState) error {
	return trace.Wrap(platformConfigureOS(ctx, osConfig, &osConfigState.platformOSConfigState))
}

type osConfigurator struct {
	remoteOSConfigProvider *osConfigProvider
	osConfigState          osConfigState
}

func newOSConfigurator(remoteOSConfigProvider *osConfigProvider) *osConfigurator {
	return &osConfigurator{
		remoteOSConfigProvider: remoteOSConfigProvider,
	}
}

func (c *osConfigurator) updateOSConfiguration(ctx context.Context) error {
	desiredOSConfig, err := c.remoteOSConfigProvider.targetOSConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := configureOS(ctx, desiredOSConfig, &c.osConfigState); err != nil {
		return trace.Wrap(err, "configuring OS")
	}
	return nil
}

func (c *osConfigurator) deconfigureOS(ctx context.Context) error {
	// configureOS is meant to be called with an empty config to deconfigure anything necessary.
	return trace.Wrap(configureOS(ctx, &osConfig{}, &c.osConfigState))
}

// runOSConfigurationLoop will keep running until ctx is canceled or an
// unrecoverable error is encountered, in order to keep the host OS
// configuration up to date.
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

// tunIPv6ForPrefix returns the IPv6 address to assign to the TUN interface under
// ipv6Prefix. It always returns the second address in the range because the
// first address (ending with all zeros) is the subnet router anycast address.
func tunIPv6ForPrefix(ipv6Prefix string) (string, error) {
	addr, err := netip.ParseAddr(ipv6Prefix)
	if err != nil {
		return "", trace.Wrap(err, "parsing IPv6 prefix %s", ipv6Prefix)
	}
	if !addr.Is6() {
		return "", trace.BadParameter("IPv6 prefix %s is not an IPv6 address", ipv6Prefix)
	}
	return addr.Next().String(), nil
}

// tunIPv4ForCIDR returns the IPv4 address to use for the TUN interface in
// cidrRange. It always returns the second address in the range.
func tunIPv4ForCIDR(cidrRange string) (string, error) {
	_, ipnet, err := net.ParseCIDR(cidrRange)
	if err != nil {
		return "", trace.Wrap(err, "parsing CIDR %q", cidrRange)
	}
	// ipnet.IP is the network address, ending in 0s, like 100.64.0.0
	// Add 1 to assign the TUN address, like 100.64.0.1
	tunAddress := ipnet.IP
	tunAddress[len(tunAddress)-1]++
	return tunAddress.String(), nil
}

func runCommand(ctx context.Context, path string, args ...string) error {
	cmdString := strings.Join(append([]string{path}, args...), " ")
	log.DebugContext(ctx, "Running command", "cmd", cmdString)
	cmd := exec.CommandContext(ctx, path, args...)
	var output strings.Builder
	cmd.Stderr = &output
	cmd.Stdout = &output
	if err := cmd.Run(); err != nil {
		return trace.Wrap(err, `running "%s" output: %s`, cmdString, output.String())
	}
	return nil
}
