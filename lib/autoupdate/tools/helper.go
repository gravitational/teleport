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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	stacksignal "github.com/gravitational/teleport/lib/utils/signal"
)

// Variables might to be overridden during compilation time for integration tests.
var (
	// version is the current version of the Teleport.
	version = teleport.Version
	// baseURL is CDN URL for downloading official Teleport packages.
	baseURL = defaultBaseURL
)

// CheckAndUpdateLocal verifies if the TELEPORT_TOOLS_VERSION environment variable
// is set and a version is defined (or disabled by setting it to "off"). The requested
// version is compared with the current client tools version. If they differ, the version
// package is downloaded, extracted to the client tools directory, and re-executed
// with the updated version.
// If $TELEPORT_HOME/bin contains downloaded client tools, it always re-executes
// using the version from the home directory.
func CheckAndUpdateLocal(ctx context.Context, reExecArgs []string) error {
	toolsDir, err := Dir()
	if err != nil {
		slog.WarnContext(ctx, "Client tools update is disabled", "error", err)
		return nil
	}

	updater := NewUpdater(toolsDir, version, WithBaseURL(baseURL))
	// At process startup, check if a version has already been downloaded to
	// $TELEPORT_HOME/bin or if the user has set the TELEPORT_TOOLS_VERSION
	// environment variable. If so, re-exec that version of client tools.
	toolsVersion, reExec, err := updater.CheckLocal()
	if err != nil {
		return trace.Wrap(err)
	}
	if reExec {
		return trace.Wrap(updateAndReExec(ctx, updater, toolsVersion, reExecArgs))
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
	toolsDir, err := Dir()
	if err != nil {
		slog.WarnContext(ctx, "Client tools update is disabled", "error", err)
		return nil
	}
	updater := NewUpdater(toolsDir, version, WithBaseURL(baseURL))
	// The user has typed a command like `tsh ssh ...` without being logged in,
	// if the running binary needs to be updated, update and re-exec.
	//
	// If needed, download the new version of client tools and re-exec. Make
	// sure to exit this process with the same exit code as the child process.
	toolsVersion, reExec, err := updater.CheckRemote(ctx, proxy, insecure)
	if err != nil {
		return trace.Wrap(err)
	}
	if reExec {
		return trace.Wrap(updateAndReExec(ctx, updater, toolsVersion, reExecArgs))
	}

	return nil
}

func updateAndReExec(ctx context.Context, updater *Updater, toolsVersion string, args []string) error {
	ctxUpdate, cancel := stacksignal.GetSignalHandler().NotifyContext(ctx)
	defer cancel()
	// Download the version of client tools required by the cluster. This
	// is required if the user passed in the TELEPORT_TOOLS_VERSION
	// explicitly.
	err := updater.UpdateWithLock(ctxUpdate, toolsVersion)
	if err != nil && !errors.Is(err, context.Canceled) {
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
