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

package main

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows/svc"

	autoupdate "github.com/gravitational/teleport/lib/autoupdate/agent"
	libutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/windowsservice"
)

const (
	// serviceInitialDelay is the delay before the first update attempt after the service starts.
	// Matches OnActiveSec of the systemd timer on Linux.
	serviceInitialDelay = time.Minute
	// serviceInterval is the interval between update attempts.
	// Matches OnUnitActiveSec of the systemd timer on Linux.
	serviceInterval = 5 * time.Minute
	// serviceJitter is the maximum random delay added to each interval.
	// Matches RandomizedDelaySec of the systemd timer on Linux.
	serviceJitter = time.Minute
)

// errUpdaterReplaced is returned by the service loop after the updater updated itself,
// so that the service exits non-zero and SCM restarts it using the new binPath.
var errUpdaterReplaced = errors.New("updater binary was replaced, restarting service")

// cmdService runs the updater loop as the TeleportUpdater Windows service.
// This is the Windows equivalent of the systemd timer: the update subcommand
// is executed periodically for as long as the service runs.
// When not started by the service control manager, the loop runs in the foreground.
func cmdService(ctx context.Context, ccfg *cliConfig) error {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return trace.Wrap(err, "determining if running as a Windows service")
	}
	handler := &updaterService{ccfg: ccfg}
	if !isService {
		plog.InfoContext(ctx, "Not running as a Windows service, running update loop in the foreground.")
		return trace.Wrap(handler.Execute(ctx, nil))
	}

	// Log to the Windows Event Log, as there is no console attached to a service.
	// The event source is registered by the setup subcommand.
	closeLog, err := windowsservice.InitSlogEventLogger(autoupdate.EventSourceName)
	if err != nil {
		return trace.Wrap(err, "initializing Event Log logging")
	}
	defer closeLog()

	return trace.Wrap(windowsservice.Run(&windowsservice.RunConfig{
		Name:    autoupdate.UpdaterServiceName,
		Handler: handler,
		Logger:  plog,
	}))
}

// updaterService implements windowsservice.ServiceHandler.
type updaterService struct {
	ccfg *cliConfig
}

// Execute runs the update loop until ctx is canceled (SCM stop or shutdown),
// or until the updater binary is replaced by an update.
func (s *updaterService) Execute(ctx context.Context, _ []string) error {
	updater, lockFile, err := initConfig(ctx, s.ccfg)
	if err != nil {
		return trace.Wrap(err, "failed to initialize updater")
	}
	plog.InfoContext(ctx, "Updater service started.", "initial_delay", serviceInitialDelay, "interval", serviceInterval)

	timer := time.NewTimer(serviceInitialDelay + rand.N(serviceJitter))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			plog.InfoContext(ctx, "Updater service stopping.")
			return ctx.Err()
		case <-timer.C:
		}
		replaced, err := s.tick(ctx, updater, lockFile)
		if err != nil && !errors.Is(err, context.Canceled) {
			plog.ErrorContext(ctx, "Update failed.", "error", err)
		}
		if replaced {
			plog.InfoContext(ctx, "Updater binary was replaced. Exiting so that the service is restarted on the new version.")
			return errUpdaterReplaced
		}
		timer.Reset(serviceInterval + rand.N(serviceJitter))
	}
}

// tick performs a single update attempt, skipping it if another updater command holds the lock.
// tick returns true if the active revision changed, which means the updater binary itself was replaced.
func (s *updaterService) tick(ctx context.Context, updater *autoupdate.Updater, lockFile string) (replaced bool, err error) {
	unlock, err := libutils.FSTryWriteLock(lockFile)
	if errors.Is(err, libutils.ErrUnsuccessfulLockTry) {
		plog.InfoContext(ctx, "Skipping update, another updater command is running.", "lock", lockFile)
		return false, nil
	} else if err != nil {
		return false, trace.Wrap(err)
	}
	defer func() {
		if err := unlock(); err != nil {
			plog.DebugContext(ctx, "Failed to close lock file", "error", err)
		}
	}()

	before := activeRevision(ctx, updater)
	if err := updater.Update(ctx, false); err != nil {
		return false, trace.Wrap(err)
	}
	after := activeRevision(ctx, updater)
	return before != after && after != (autoupdate.Revision{}), nil
}

// activeRevision returns the currently active revision from local state, or
// the zero Revision if it cannot be determined.
func activeRevision(ctx context.Context, updater *autoupdate.Updater) autoupdate.Revision {
	rev, err := updater.ActiveRevision()
	if err != nil {
		plog.DebugContext(ctx, "Unable to determine active revision.", "error", err)
		return autoupdate.Revision{}
	}
	return rev
}

var _ windowsservice.ServiceHandler = (*updaterService)(nil)
