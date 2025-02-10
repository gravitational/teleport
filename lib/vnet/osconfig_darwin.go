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
	"context"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
)

// platformOSConfigState is not used on darwin.
type platformOSConfigState struct{}

// platformConfigureOS configures the host OS according to cfg. It is safe to
// call repeatedly, and it is meant to be called with an empty osConfig to
// deconfigure anything necessary before exiting.
func platformConfigureOS(ctx context.Context, cfg *osConfig, _ *platformOSConfigState) error {
	// There is no need to remove IP addresses or routes, they will automatically be cleaned up when the
	// process exits and the TUN is deleted.

	if cfg.tunIPv4 != "" {
		log.InfoContext(ctx, "Setting IPv4 address for the TUN device.",
			"device", cfg.tunName, "address", cfg.tunIPv4)
		if err := runCommand(ctx,
			"ifconfig", cfg.tunName, cfg.tunIPv4, cfg.tunIPv4, "up",
		); err != nil {
			return trace.Wrap(err)
		}
	}
	for _, cidrRange := range cfg.cidrRanges {
		log.InfoContext(ctx, "Setting an IP route for the VNet.", "netmask", cidrRange)
		if err := runCommand(ctx,
			"route", "add", "-net", cidrRange, "-interface", cfg.tunName,
		); err != nil {
			return trace.Wrap(err)
		}
	}

	if cfg.tunIPv6 != "" {
		log.InfoContext(ctx, "Setting IPv6 address for the TUN device.", "device", cfg.tunName, "address", cfg.tunIPv6)
		if err := runCommand(ctx,
			"ifconfig", cfg.tunName, "inet6", cfg.tunIPv6, "prefixlen", "64",
		); err != nil {
			return trace.Wrap(err)
		}

		log.InfoContext(ctx, "Setting an IPv6 route for the VNet.")
		if err := runCommand(ctx,
			"route", "add", "-inet6", cfg.tunIPv6, "-prefixlen", "64", "-interface", cfg.tunName,
		); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := configureDNS(ctx, cfg.dnsAddr, cfg.dnsZones); err != nil {
		return trace.Wrap(err, "configuring DNS")
	}

	return nil
}

const resolverFileComment = "# automatically installed by Teleport VNet"

var resolverPath = filepath.Join("/", "etc", "resolver")

func configureDNS(ctx context.Context, nameserver string, zones []string) error {
	if len(nameserver) == 0 && len(zones) > 0 {
		return trace.BadParameter("empty nameserver with non-empty zones")
	}

	log.DebugContext(ctx, "Configuring DNS.", "nameserver", nameserver, "zones", zones)
	if err := os.MkdirAll(resolverPath, os.FileMode(0755)); err != nil {
		return trace.Wrap(err, "creating %s", resolverPath)
	}

	managedFiles, err := vnetManagedResolverFiles()
	if err != nil {
		return trace.Wrap(err, "finding VNet managed files in /etc/resolver")
	}

	// Always attempt to write or clean up all files below, even if encountering errors with one or more of
	// them.
	var allErrors []error

	for _, zone := range zones {
		fileName := filepath.Join(resolverPath, zone)
		contents := resolverFileComment + "\nnameserver " + nameserver
		if err := os.WriteFile(fileName, []byte(contents), 0644); err != nil {
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
