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
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"

	"github.com/google/renameio/v2"
	"github.com/gravitational/trace"
)

// platformOSConfigState tracks which OS configuration calls have already
// been applied so platformConfigureOS can skip them on subsequent ticks.
//
// The OS configuration loop runs every 10 seconds to pick up new
// clusters and CIDR ranges. Without this gate, the loop re-issues
// `ifconfig` and `route add` every tick. macOS's `SIOCSIFADDR` performs
// a delete-then-add internally even when the address is unchanged, so
// every tick emits an `RTM_DELADDR`/`RTM_NEWADDR` pair on the kernel
// route socket. Other linkmon subscribers (notably Tailscale's
// magicsock, embedded by Coder) read that pair as a major link change
// and tear down their connections.
//
// Mirrors the gating already in place on Linux. See `osconfig_linux.go`
// for the equivalent state struct; field names match.
type platformOSConfigState struct {
	configuredIPv6       bool
	configuredIPv4       bool
	configuredCidrRanges []string
}

// platformConfigureOS configures the host OS according to cfg. It is
// safe to call repeatedly: subsequent calls with an unchanged cfg do
// not re-issue ifconfig or route add for already-configured addresses
// or CIDR ranges; only newly-added CIDR ranges trigger a route add.
// DNS resolver files are reconciled on every call. An empty osConfig
// deconfigures everything by resetting the cached state so the next
// non-empty call re-applies.
func platformConfigureOS(ctx context.Context, cfg *osConfig, state *platformOSConfigState) error {
	// There is no need to remove IP addresses or routes; they are
	// cleaned up when the process exits and the TUN is deleted. An
	// empty cfg signals deconfigure: reconcile DNS first, then reset
	// the cached state on success so a future re-apply is not silently
	// skipped.
	if cfg.tunName == "" {
		if err := configureDNS(ctx, cfg.dnsAddrs, cfg.dnsZones); err != nil {
			return trace.Wrap(err, "configuring DNS")
		}
		*state = platformOSConfigState{}
		return nil
	}

	if cfg.tunIPv4 != "" && !state.configuredIPv4 {
		log.InfoContext(ctx, "Setting IPv4 address for the TUN device.",
			"device", cfg.tunName, "address", cfg.tunIPv4)
		if err := runCommand(ctx,
			"ifconfig", cfg.tunName, cfg.tunIPv4, cfg.tunIPv4, "up",
		); err != nil {
			return trace.Wrap(err)
		}
		state.configuredIPv4 = true
	}
	for _, cidrRange := range cfg.cidrRanges {
		if slices.Contains(state.configuredCidrRanges, cidrRange) {
			continue
		}
		log.InfoContext(ctx, "Setting an IP route for the VNet.", "netmask", cidrRange)
		if err := runCommand(ctx,
			"route", "add", "-net", cidrRange, "-interface", cfg.tunName,
		); err != nil {
			return trace.Wrap(err)
		}
		state.configuredCidrRanges = append(state.configuredCidrRanges, cidrRange)
	}

	if cfg.tunIPv6 != "" && !state.configuredIPv6 {
		log.InfoContext(ctx, "Setting IPv6 address for the TUN device.",
			"device", cfg.tunName, "address", cfg.tunIPv6)
		if err := runCommand(ctx,
			"ifconfig", cfg.tunName, "inet6", cfg.tunIPv6, "prefixlen", "64",
		); err != nil {
			return trace.Wrap(err)
		}
		// Mark IPv6 configured as soon as the alias is set, so a
		// failure of the route add below does not cause a retry of
		// the ifconfig on the next tick (which would re-trigger the
		// alias flap this gate exists to prevent).
		state.configuredIPv6 = true

		log.InfoContext(ctx, "Setting an IPv6 route for the VNet.")
		if err := runCommand(ctx,
			"route", "add", "-inet6", cfg.tunIPv6, "-prefixlen", "64", "-interface", cfg.tunName,
		); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := configureDNS(ctx, cfg.dnsAddrs, cfg.dnsZones); err != nil {
		return trace.Wrap(err, "configuring DNS")
	}

	return nil
}

const resolverFileComment = "# automatically installed by Teleport VNet"

var resolverPath = filepath.Join("/", "etc", "resolver")

func configureDNS(ctx context.Context, nameservers []string, zones []string) error {
	if len(nameservers) == 0 {
		// There are no nameservers so VNet can't handle any DNS zones. Continue
		// so that any VNet-managed resolver files can be deleted.
		zones = nil
	}

	log.DebugContext(ctx, "Configuring DNS.", "nameservers", nameservers, "zones", zones)
	if err := os.MkdirAll(resolverPath, os.FileMode(0755)); err != nil {
		return trace.Wrap(err, "creating %s", resolverPath)
	}

	managedFiles, err := vnetManagedResolverFiles()
	if err != nil {
		return trace.Wrap(err, "finding VNet managed files in /etc/resolver")
	}

	// Always attempt to write or clean up all files below, even if encountering
	// errors with one or more of them.
	var allErrors []error

	var fileContents bytes.Buffer
	fileContents.WriteString(resolverFileComment)
	fileContents.WriteByte('\n')
	for _, nameserver := range nameservers {
		fileContents.WriteString("nameserver ")
		fileContents.WriteString(nameserver)
		fileContents.WriteByte('\n')
	}

	for _, zone := range zones {
		fileName := filepath.Join(resolverPath, zone)
		if err := renameio.WriteFile(fileName, fileContents.Bytes(), 0644); err != nil {
			allErrors = append(allErrors, trace.Wrap(err, "writing DNS configuration file %s", fileName))
		} else {
			// Successfully wrote this file, don't clean it up below.
			delete(managedFiles, fileName)
		}
	}

	// Delete stale files.
	for fileName := range managedFiles {
		if err := os.Remove(fileName); err != nil {
			allErrors = append(allErrors, trace.Wrap(err, "deleting VNet managed file %s", fileName))
		}
	}

	return trace.NewAggregate(allErrors...)
}

func vnetManagedResolverFiles() (map[string]struct{}, error) {
	entries, err := os.ReadDir(resolverPath)
	if err != nil {
		return nil, trace.Wrap(err, "reading %s", resolverPath)
	}

	matchingFiles := make(map[string]struct{})
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filePath := filepath.Join(resolverPath, entry.Name())
		file, err := os.Open(filePath)
		if err != nil {
			return nil, trace.Wrap(err, "opening %s", filePath)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		if scanner.Scan() {
			if resolverFileComment == scanner.Text() {
				matchingFiles[filePath] = struct{}{}
			}
		}
	}
	return matchingFiles, nil
}
