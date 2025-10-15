/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package tools

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/autoupdate"
	stacksignal "github.com/gravitational/teleport/lib/utils/signal"
)

// Variables might to be overridden during compilation time for integration tests.
var (
	// Version is the current version of the Teleport.
	// The variable is overloaded during integration tests to emulate different
	// Teleport versions. See `integration/autoupdate/tools/updater/modules.go`.
	Version = teleport.Version
)

// newUpdater inits the updater with default base URL and creates directory
// if it doesn't exist.
func newUpdater(toolsDir string) (*Updater, error) {
	baseURL := autoupdate.DefaultBaseURL
	// Overrides default base URL for custom CDN for downloading updates.
	if envBaseURL := os.Getenv(autoupdate.BaseURLEnvVar); envBaseURL != "" {
		baseURL = envBaseURL
	}

	// Create tools directory if it does not exist.
	if err := os.MkdirAll(toolsDir, 0o755); err != nil {
		return nil, trace.Wrap(err)
	}

	return NewUpdater(toolsDir, Version, WithBaseURL(baseURL)), nil
}

// CheckAndUpdateLocal verifies if the TELEPORT_TOOLS_VERSION environment variable
// is set and whether a version is defined (or explicitly disabled by setting it to "off").
// The environment variable always takes precedence over other version settings.
//
// If `currentProfileName` is specified, the function attempts to find the tools version
// required for the specified cluster in configuration file and re-execute it.
//
// The requested version is compared to the currently running client tools version.
// If they differ, the requested version is downloaded and extracted into the client tools directory,
// the installation is recorded in the configuration file, and the tool is re-executed with the updated version.
func CheckAndUpdateLocal(ctx context.Context, currentProfileName string, reExecArgs []string) error {
	// If client tools updates are explicitly disabled, we want to catch this as soon as possible
	// so we don't try to read te user home directory, fail, and log warnings.
	if os.Getenv(teleportToolsVersionEnv) == teleportToolsVersionEnvDisabled {
		return nil
	}

	var err error
	if currentProfileName == "" {
		home := os.Getenv(types.HomeEnvVar)
		if home != "" {
			home = filepath.Clean(home)
		}
		profilePath := profile.FullProfilePath(home)
		currentProfileName, err = profile.GetCurrentProfileName(profilePath)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}

	toolsDir, err := Dir()
	if err != nil {
		slog.WarnContext(ctx, "Failed to detect the teleport home directory, client tools updates are disabled", "error", err)
		return nil
	}
	updater, err := newUpdater(toolsDir)
	if err != nil {
		slog.WarnContext(ctx, "Failed to create the updater, client tools updates are disabled", "error", err)
		return nil
	}

	slog.DebugContext(ctx, "Attempting to local update", "current_profile_name", currentProfileName)
	resp, err := updater.CheckLocal(ctx, currentProfileName)
	if err != nil {
		slog.WarnContext(ctx, "Failed to check local teleport versions, client tools updates are disabled", "error", err)
		return nil
	}

	if resp.ReExec {
		return trace.Wrap(updateAndReExec(ctx, updater, resp.Version, reExecArgs))
	}

	return nil
}

// CheckAndUpdateRemote verifies the client tools version configured for updates in the cluster
// by making an HTTP request to the `webapi/find` endpoint.
//
// If the TELEPORT_TOOLS_VERSION environment variable is set during the remote check,
// the version specified in the environment variable takes precedence over the version
// provided by the cluster. This version will also be recorded in the configuration for the cluster.
//
// The requested version is compared with the current client tools version.
// If they differ, the requested version is downloaded, extracted into the client tools directory,
// the installed version is recorded in the configuration, and the tool is re-executed
// with the updated version.
func CheckAndUpdateRemote(ctx context.Context, currentProfileName string, insecure bool, reExecArgs []string) error {
	// If client tools updates are explicitly disabled, we want to catch this as soon as possible
	// so we don't try to read te user home directory, fail, and log warnings.
	// If we are re-executed, we ignore the "off" version because some previous Teleport versions
	// are disabling execution too aggressively and this causes stuck updates.
	// If "off" was set by the user, we would not be re-executed.
	if os.Getenv(teleportToolsVersionEnv) == teleportToolsVersionEnvDisabled && os.Getenv(teleportToolsVersionReExecEnv) == "" {
		return nil
	}

	toolsDir, err := Dir()
	if err != nil {
		slog.WarnContext(ctx, "Failed to detect the teleport home directory, client tools updates are disabled", "error", err)
		return nil
	}
	updater, err := newUpdater(toolsDir)
	if err != nil {
		slog.WarnContext(ctx, "Failed to create the updater, client tools updates are disabled", "error", err)
		return nil
	}

	slog.DebugContext(ctx, "Attempting to remote update", "current_profile_name", currentProfileName, "insecure", insecure)
	resp, err := updater.CheckRemote(ctx, currentProfileName, insecure)
	if err != nil {
		slog.WarnContext(ctx, "Failed to check remote teleport versions, client tools updates are disabled", "error", err)
		return nil
	}

	if !resp.Disabled && resp.ReExec {
		return trace.Wrap(updateAndReExec(ctx, updater, resp.Version, reExecArgs))
	}

	return nil
}

// DownloadUpdate checks if a client tools version is set for update in the cluster
// configuration by making an HTTP request to the `webapi/find` endpoint.
// Downloads the new version if it is not already installed without re-execution.
func DownloadUpdate(ctx context.Context, name string, insecure bool) error {
	toolsDir, err := Dir()
	if err != nil {
		slog.WarnContext(ctx, "Client tools update is disabled", "error", err)
		return nil
	}
	updater, err := newUpdater(toolsDir)
	if err != nil {
		return trace.Wrap(err)
	}

	slog.DebugContext(ctx, "Attempting to remote update", "name", name, "insecure", insecure)
	resp, err := updater.CheckRemote(ctx, name, insecure)
	if err != nil {
		return trace.Wrap(err)
	}

	if !resp.Disabled && resp.ReExec {
		ctxUpdate, cancel := stacksignal.GetSignalHandler().NotifyContext(ctx)
		defer cancel()
		err := updater.Update(ctxUpdate, resp.Version)
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, ErrNoBaseURL) {
			return trace.Wrap(err)
		}
	}

	return nil
}

func updateAndReExec(ctx context.Context, updater *Updater, toolsVersion string, args []string) error {
	ctxUpdate, cancel := stacksignal.GetSignalHandler().NotifyContext(ctx)
	defer cancel()
	// Download the version of client tools required by the cluster. This
	// is required if the user passed in the TELEPORT_TOOLS_VERSION
	// explicitly.
	err := updater.Update(ctxUpdate, toolsVersion)
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, ErrNoBaseURL) {
		slog.ErrorContext(ctx, "Failed to update tools version", "error", err, "version", toolsVersion)
		// Continue executing the current version of the client tools (tsh, tctl)
		// to avoid potential issues with update process (timeout, missing version).
		return nil
	}

	// Re-execute client tools with the correct version of client tools.
	code, err := updater.Exec(ctx, toolsVersion, args)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.DebugContext(ctx, "Failed to re-exec client tool", "error", err, "code", code)
		os.Exit(code)
	} else if err == nil {
		os.Exit(code)
	}

	return nil
}
