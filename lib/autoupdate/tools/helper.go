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
	// version is the current version of the Teleport.
	version = teleport.Version
	// baseURL is CDN URL for downloading official Teleport packages.
	baseURL = autoupdate.DefaultBaseURL
	// ErrDisabled returns when home folder isn't set
	ErrDisabled = errors.New("client tools update is disabled")
)

// newUpdater inits the updater with default base ULR and tools directory
// from Teleport home directory.
func newUpdater() (*Updater, error) {
	toolsDir, err := Dir()
	if err != nil {
		return nil, ErrDisabled
	}

	// Overrides default base URL for custom CDN for downloading updates.
	if envBaseURL := os.Getenv(autoupdate.BaseURLEnvVar); envBaseURL != "" {
		baseURL = envBaseURL
	}

	// Create tools directory if it does not exist.
	if err := os.MkdirAll(toolsDir, 0o755); err != nil {
		return nil, trace.Wrap(err)
	}

	return NewUpdater(toolsDir, version, WithBaseURL(baseURL)), nil
}

// CheckAndUpdateLocal verifies if the TELEPORT_TOOLS_VERSION environment variable
// is set and a version is defined (or disabled by setting it to "off"). The requested
// version is compared with the current client tools version. If they differ, the version
// package is downloaded, extracted to the client tools directory, and re-executed
// with the updated version.
func CheckAndUpdateLocal(ctx context.Context, name string, reExecArgs []string) error {
	var err error
	if name == "" {
		home := os.Getenv(types.HomeEnvVar)
		if home != "" {
			home = filepath.Clean(home)
		}
		profilePath := profile.FullProfilePath(home)
		name, err = profile.GetCurrentProfileName(profilePath)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}

	updater, err := newUpdater()
	if errors.Is(err, ErrDisabled) {
		slog.WarnContext(ctx, "Client tools update is disabled")
		return nil
	} else if err != nil {
		return trace.Wrap(err)
	}

	slog.DebugContext(ctx, "Attempting to local update", "name", name)
	resp, err := updater.CheckLocal(name)
	if err != nil {
		return trace.Wrap(err)
	}

	if resp.ReExec {
		err := updateAndReExec(ctx, updater, resp.Version, reExecArgs)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// CheckAndUpdateRemote verifies client tools version is set for update in cluster
// configuration by making the http request to `webapi/find` endpoint. The requested
// version is compared with the current client tools version. If they differ, the version
// package is downloaded, extracted to the client tools directory, and re-executed
// with the updated version.
func CheckAndUpdateRemote(ctx context.Context, name string, insecure bool, reExecArgs []string) error {
	updater, err := newUpdater()
	if errors.Is(err, ErrDisabled) {
		slog.WarnContext(ctx, "Client tools update is disabled", "error", err)
		return nil
	} else if err != nil {
		return trace.Wrap(err)
	}

	slog.DebugContext(ctx, "Attempting to remote update", "name", name, "insecure", insecure)
	resp, err := updater.CheckRemote(ctx, name, insecure)
	if err != nil {
		return trace.Wrap(err)
	}

	if !resp.Disabled && resp.ReExec {
		err := updateAndReExec(ctx, updater, resp.Version, reExecArgs)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// DownloadUpdate checks if a client tools version is set for update in the cluster
// configuration by making an HTTP request to the `webapi/find` endpoint.
// Downloads the new version if it is not already installed without re-execution.
func DownloadUpdate(ctx context.Context, name string, insecure bool) error {
	updater, err := newUpdater()
	if errors.Is(err, ErrDisabled) {
		slog.WarnContext(ctx, "Client tools update is disabled", "error", err)
		return nil
	} else if err != nil {
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
	code, err := updater.Exec(toolsVersion, args)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.DebugContext(ctx, "Failed to re-exec client tool", "error", err, "code", code)
		os.Exit(code)
	} else if err == nil {
		os.Exit(code)
	}

	return nil
}
