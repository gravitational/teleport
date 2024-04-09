//go:build darwin
// +build darwin

package vnet

import (
	"bufio"
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gravitational/trace"
)

func configureOS(ctx context.Context, cfg *osConfig) error {
	if cfg.tunIP != "" {
		slog.With("device", cfg.tunName, "address", cfg.tunIP).Info("Setting IP address for the TUN device.")
		cmd := exec.CommandContext(ctx, "ifconfig", cfg.tunName, cfg.tunIP, cfg.tunIP, "up")
		if err := cmd.Run(); err != nil {
			return trace.Wrap(err, "running %v", cmd.Args)
		}
	}

	for _, mask := range cfg.vnetNetmasks {
		slog.With("netmask", mask).Info("Setting an IP route for the VNet.")
		cmd := exec.CommandContext(ctx, "route", "add", "-net", mask, "-interface", cfg.tunName)
		if err := cmd.Run(); err != nil {
			return trace.Wrap(err, "running %v", cmd.Args)
		}
	}

	if err := setupDNS(ctx, cfg.vnetNameserverAddress, cfg.dnsZones); err != nil {
		return trace.Wrap(err, "configuring DNS")
	}

	return nil
}

const resolverFileComment = "# automatically installed by Teleport VNet"

var resolverPath = filepath.Join("/", "etc", "resolver")

func setupDNS(ctx context.Context, nameserver string, zones []string) error {
	slog.With("nameserver", nameserver, "zones", zones).Debug("Setting up DNS.")
	if err := os.MkdirAll(resolverPath, os.FileMode(0755)); err != nil {
		return trace.Wrap(err, "creating %s", resolverPath)
	}

	managedFiles, err := vnetManagedResolverFiles()
	if err != nil {
		return trace.Wrap(err, "finding VNet managed files in /etc/resolver")
	}
	for _, zone := range zones {
		fileName := filepath.Join(resolverPath, zone)
		delete(managedFiles, fileName)
		contents := resolverFileComment + "\nnameserver " + nameserver
		if err := os.WriteFile(fileName, []byte(contents), 0644); err != nil {
			return trace.Wrap(err, "writing DNS configuration file %s", fileName)
		}
	}
	// Delete stale files.
	for fileName := range managedFiles {
		if err := os.Remove(fileName); err != nil {
			return trace.Wrap(err, "deleting VNet managed file %s", fileName)
		}
	}
	return nil
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
		scanner := bufio.NewScanner(file)
		if scanner.Scan() {
			if resolverFileComment == scanner.Text() {
				matchingFiles[filePath] = struct{}{}
			}
		}
	}
	return matchingFiles, nil
}
