// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package hoststep

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

// UninstallRequest is what the uninstall step needs.
type UninstallRequest struct {
	// Proxy is the target cluster's proxy address.
	Proxy string `json:"proxy"`
	// SourceHostUUID is the source agent the scoped agent replaces.
	SourceHostUUID string `json:"source_host_uuid"`
}

// uninstall removes the scoped agent's install, once the scoped config proves
// it belongs to the migration of req's proxy and source host. It holds the
// install lock, so it cannot run beside an install that outlived its command.
// The update timer and service stop before the agent unit, so an update cannot
// fire mid-removal. A unit that fails to stop, or is still active after the
// stop, is refused, and so is a failed teleport-update uninstall, which removes
// the install and its units; each refusal leaves the data dir and config in
// place. The config goes last, so a removal cut short leaves it for a person
// to find. The lock file stays: removing it while an install waits on it would
// let a later install lock a new file beside the waiting one. The fetch cache
// stays, so a later apply reuses it.
func (h hostEnv) uninstall(ctx context.Context, req UninstallRequest) (struct{}, error) {
	// teleport-update uninstall can leave a disabled unit file behind, which
	// does not count as an install.
	if !slices.ContainsFunc([]string{scopedConfigPath, scopedDataDir, scopedInstallDir}, h.exists) {
		return struct{}{}, nil
	}
	if !h.exists(scopedConfigPath) {
		return struct{}{}, trace.CompareFailed("nothing at %s proves the install under teleport_migrated is this migration's; roll it back by hand", scopedConfigPath)
	}
	unlock, err := utils.FSTryWriteLock(h.path(scopedConfigPath + ".lock"))
	if errors.Is(err, utils.ErrUnsuccessfulLockTry) {
		return struct{}{}, trace.AlreadyExists("another migrate command is installing on this host; wait for it, then roll back")
	}
	if err != nil {
		return struct{}{}, trace.Wrap(err, "locking %q", scopedConfigPath+".lock")
	}
	defer unlock()
	if err := h.checkScopedConfigOwner(req.Proxy, req.SourceHostUUID, "roll it back by hand"); err != nil {
		return struct{}{}, trace.Wrap(err)
	}
	units := []string{scopedUpdaterTimer, scopedUpdaterService, scopedUnit}
	for _, unit := range units {
		if !h.exists(path.Join("/etc/systemd/system", unit)) {
			continue
		}
		// A failed stop is refused here, not left to the is-active check: the
		// updater service is a oneshot, which is-active does not report as
		// active while an update runs.
		if err := runCommand(ctx, "systemctl", "stop", unit); err != nil {
			return struct{}{}, trace.Wrap(err, "stopping %s; stop it, then roll back", unit)
		}
		if err := runCommand(ctx, "systemctl", "disable", unit); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		}
	}
	for _, unit := range units {
		if unitActive(ctx, unit) {
			return struct{}{}, trace.CompareFailed("%s is still active; stop it and retry", unit)
		}
	}
	if h.exists(scopedInstallDir) {
		updater := "teleport-update"
		if bundled := h.path(path.Join(scopedInstallDir, "bin/teleport-update")); isExecutable(bundled) {
			updater = bundled
		}
		if err := runCommand(ctx, updater, "--install-suffix", scopedInstallSuffix, "uninstall"); err != nil {
			return struct{}{}, trace.Wrap(err, "teleport-update uninstall failed with %s stopped and disabled; run teleport-update --install-suffix %s uninstall by hand, then roll back", scopedUnit, scopedInstallSuffix)
		}
	}
	if err := os.RemoveAll(h.path(scopedDataDir)); err != nil {
		return struct{}{}, trace.ConvertSystemError(err)
	}
	if err := os.Remove(h.path(scopedConfigPath)); err != nil && !os.IsNotExist(err) {
		return struct{}{}, trace.ConvertSystemError(err)
	}
	return struct{}{}, nil
}
