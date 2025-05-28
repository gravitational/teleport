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

package helper

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/autoupdate"
	"github.com/gravitational/teleport/lib/autoupdate/tools"
	"github.com/gravitational/teleport/lib/utils"
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

// NewDefaultUpdater inits the updater with default base ULR and tools directory
// from Teleport home directory.
func NewDefaultUpdater() (*tools.Updater, error) {
	toolsDir, err := tools.Dir()
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

	return tools.NewUpdater(toolsDir, version, tools.WithBaseURL(baseURL)), nil
}

// CheckAndUpdateLocal verifies if the TELEPORT_TOOLS_VERSION environment variable
// is set and a version is defined (or disabled by setting it to "off"). The requested
// version is compared with the current client tools version. If they differ, the version
// package is downloaded, extracted to the client tools directory, and re-executed
// with the updated version.
// If $TELEPORT_HOME/bin contains downloaded client tools, it always re-executes
// using the version from the home directory.
func CheckAndUpdateLocal(ctx context.Context, currentProfileName string, reExecArgs []string) error {
	updater, err := NewDefaultUpdater()
	if errors.Is(err, ErrDisabled) {
		slog.WarnContext(ctx, "Client tools update is disabled")
		return nil
	} else if err != nil {
		return trace.Wrap(err)
	}

	resp, err := updater.CheckLocal()
	if err != nil {
		return trace.Wrap(err)
	}

	config, err := updater.LoadConfig(currentProfileName)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	if !resp.IsLocal && config != nil && config.Disabled {
		return nil
	}

	if resp.ReExec {
		err := UpdateAndReExec(ctx, updater, resp.Version, reExecArgs)
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
// If $TELEPORT_HOME/bin contains downloaded client tools, it always re-executes
// using the version from the home directory.
func CheckAndUpdateRemote(ctx context.Context, proxy string, insecure bool, reExecArgs []string) error {
	updater, err := NewDefaultUpdater()
	if errors.Is(err, ErrDisabled) {
		slog.WarnContext(ctx, "Client tools update is disabled", "error", err)
		return nil
	} else if err != nil {
		return trace.Wrap(err)
	}

	resp, err := updater.CheckRemote(ctx, proxy, insecure)
	if err != nil {
		return trace.Wrap(err)
	}

	profileName, err := utils.Host(proxy)
	if err != nil {
		return trace.Wrap(err)
	}

	config, err := updater.LoadConfig(profileName)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	if config == nil {
		config = &tools.Config{}
	}

	config.Version = resp.Version
	config.Disabled = resp.Disabled
	if err := updater.SaveConfig(profileName, config); err != nil {
		return trace.Wrap(err)
	}

	if !config.Disabled && resp.ReExec {
		err := UpdateAndReExec(ctx, updater, resp.Version, reExecArgs)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func UpdateAndReExec(ctx context.Context, updater *tools.Updater, toolsVersion string, args []string) error {
	ctxUpdate, cancel := stacksignal.GetSignalHandler().NotifyContext(ctx)
	defer cancel()
	// Download the version of client tools required by the cluster. This
	// is required if the user passed in the TELEPORT_TOOLS_VERSION
	// explicitly.
	err := updater.UpdateWithLock(ctxUpdate, toolsVersion)
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, tools.ErrNoBaseURL) {
		return trace.Wrap(err)
	}

	// Re-execute client tools with the correct version of client tools.
	code, err := updater.Exec(args)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.DebugContext(ctx, "Failed to re-exec client tool", "error", err)
		os.Exit(code)
	} else if err == nil {
		os.Exit(code)
	}

	return nil
}
