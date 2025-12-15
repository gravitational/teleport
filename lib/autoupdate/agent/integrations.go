/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package agent

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

var (
	// ErrConfigNotFound is returned by HellorUpdaterInfo when the updater config file cannot be found.
	ErrConfigNotFound = errors.New("updater config file not found")

	// ErrUnstableExecutable is returned by StableExecutable when no stable path can be found.
	ErrUnstableExecutable = errors.New("executable has unstable path")
)

const updateConfigFileEnvVar = "TELEPORT_UPDATE_CONFIG_FILE"

// IsManagedByUpdater returns true if the local Teleport binary is managed by teleport-update.
// Note that true may be returned even if auto-updates is disabled or the version is pinned.
// The binary is considered managed if it lives under /opt/teleport, but not within the package
// path at /opt/teleport/system.
func IsManagedByUpdater() (bool, error) {
	systemd, err := hasSystemD()
	if err != nil {
		return false, trace.Wrap(err)
	}
	if !systemd {
		return false, nil
	}
	teleportPath, err := os.Executable()
	if err != nil {
		return false, trace.Wrap(err, "cannot find Teleport binary")
	}
	// Consider all installations that have an update.yaml file to be
	// managed by teleport-update (the "binary" updater).
	p := findParentMatching(teleportPath, versionsDirName, 4)
	if p == "" {
		return false, nil
	}
	_, err = os.Stat(filepath.Join(p, UpdateConfigName))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, trace.Wrap(err)
	}
	return true, nil
}

// StableExecutable returns a stable path to Teleport binaries that may or may not be managed by Agent Managed Updates.
// Note that StableExecutable is not guaranteed to return the same binary, as the binary may have been updated
// since it started running. If a stable path cannot be found, an unstable path is returned with ErrUnstableExecutable.
// The unstable path returned along with ErrUnstableExecutable will always be the result of os.Executable.
func StableExecutable() (string, error) {
	origPath, err := os.Executable()
	if err != nil {
		return origPath, trace.Wrap(err)
	}
	p, err := stablePathForBinary(origPath, defaultPathDir)
	return p, trace.Wrap(err)
}

func stablePathForBinary(origPath, defaultPath string) (string, error) {
	_, name := filepath.Split(origPath)

	// If we are a package-based install, always use /usr/local/bin if it is valid.
	// This ensures that the path is stable if Managed Updates is enabled/disabled.
	if filepath.Join(packageSystemDir, "bin", name) == origPath {
		// Verify that /usr/local/bin/[name] exists and resolves.
		// If /opt/system/bin/[name] exists, /usr/local/bin/[name] is always
		// the best candidate path, regardless of where it points.
		linkPath := filepath.Join(defaultPath, name)
		if _, err := os.Stat(linkPath); err == nil {
			return linkPath, nil
		}
		return origPath, ErrUnstableExecutable
	}

	// If we are a Managed Updates install, find the correct path from Managed Updates config.
	// This is determined by looking for ../../../update.yaml, if we are in ../../../versions.
	// update.yaml will always have the target path if Managed Updates are enabled.
	if p := findParentMatching(origPath, versionsDirName, 4); p != "" {
		cfgPath := filepath.Join(p, UpdateConfigName)
		cfg, err := readConfig(cfgPath)
		if err == nil && cfg.Spec.Path != "" {
			// If the path exists and resolves, it is always the best candidate path,
			// regardless of where it points. The running binary may be outdated.
			linkPath := filepath.Join(cfg.Spec.Path, name)
			if _, err := os.Stat(linkPath); err == nil {
				return linkPath, nil
			}
		}
		// If the config exists, but we cannot find a working binary, return the unstable path.
		if _, err := os.Stat(cfgPath); err == nil {
			return origPath, ErrUnstableExecutable
		}
	}
	return origPath, nil
}

// findParentMatching returns the directory above name, if name is at rpos.
// Otherwise, it returns empty string.
func findParentMatching(path, parent string, rpos int) string {
	if parent == "" {
		return ""
	}
	var base string
	for range rpos {
		path, base = filepath.Split(filepath.Clean(path))
	}
	if base == parent {
		return filepath.Clean(path)
	}
	return ""
}

// findConfigFile returns the path to the Teleport installation's update.yaml file.
func findConfigFile() (string, error) {
	// Note that if the install dir or install suffix changes before the installation
	// is hard restarted, this path will be incorrect.
	configPath := os.Getenv(updateConfigFileEnvVar)
	if configPath != "" {
		return configPath, nil
	}
	teleportPath, err := os.Executable()
	if err != nil {
		return "", trace.Wrap(err, "cannot find Teleport binary")
	}
	p := findParentMatching(teleportPath, versionsDirName, 4)
	if p == "" {
		return "", trace.Wrap(ErrConfigNotFound)
	}
	return filepath.Join(p, UpdateConfigName), nil
}

// ReadHelloUpdaterInfo reads the updater config and generates a proto.UpdaterV2Info
// that can be reported in the inventory hello message.
// This function performs io operations, its usage must be cached
// (the downstream inventory handler does this for us).
func ReadHelloUpdaterInfo(ctx context.Context, log *slog.Logger, hostUUID string) (*types.UpdaterV2Info, error) {
	info := &types.UpdaterV2Info{}

	configPath, err := findConfigFile()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cfg, err := readConfig(configPath)
	if err != nil {
		return nil, trace.Wrap(err, "reading config file %s", configPath)
	}

	// Note that only IDFile may be read from Status on the initial HELLO.
	// Any fields set after the agent starts (e.g., active version) will be
	// outdated until the agent is confirmed healthy.

	info.UpdateGroup = cfg.Spec.Group
	if info.UpdateGroup == "" {
		info.UpdateGroup = defaultSetting
	}
	if p := cfg.Status.IDFile; p != "" {
		machineID, err := os.ReadFile(systemdMachineIDFile)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			log.WarnContext(ctx, "Failed to read systemd machine ID.", "path", systemdMachineIDFile, errorKey, err)
			log.WarnContext(ctx, "Updater ID may be inaccurate for tracking.")
			machineID = nil
		}
		id, err := findDBPIDUUID(p, []byte(hostUUID), machineID, true)
		if err != nil {
			log.ErrorContext(ctx, "Failed to determine updater ID.", "path", p, errorKey, err)
			log.ErrorContext(ctx, "Updater ID cannot be used for tracking this agent.")
		} else {
			info.UpdateUUID = id[:]
		}
	} else {
		log.ErrorContext(ctx, "Updater ID is not available to the updater and cannot be used to track this agent.")
	}

	switch {
	case !cfg.Spec.Enabled:
		info.UpdaterStatus = types.UpdaterStatus_UPDATER_STATUS_DISABLED
	case cfg.Spec.Pinned:
		info.UpdaterStatus = types.UpdaterStatus_UPDATER_STATUS_PINNED
	default:
		info.UpdaterStatus = types.UpdaterStatus_UPDATER_STATUS_OK
	}
	return info, nil
}

func findDBPIDUUID(path string, systemID, namespaceID []byte, persist bool) (uuid.UUID, error) {
	id, err := FindDBPID(path, systemID, namespaceID, persist)
	if err != nil {
		return uuid.Nil, trace.Wrap(err)
	}
	v, err := uuid.Parse(id)
	return v, trace.Wrap(err)
}
