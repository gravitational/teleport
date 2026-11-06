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
	"path/filepath"
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
	// Promote is set when promote runs this step, after the promoted agent has
	// joined, to remove the scoped install. It changes three things:
	//   - A scoped install from a by-hand migration is accepted. Its config
	//     carries the teleport.dev/reconfigure-source-uuid label instead of the
	//     tool's own.
	//   - A refusal gives only the reason. Rollback's advice would be wrong
	//     here, and promote adds its own.
	//   - The fetch cache is removed last. Nothing on the host needs it once
	//     the host is promoted.
	Promote bool `json:"promote,omitempty"`
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
// stays for a later apply, unless promote is the caller.
func (h hostEnv) uninstall(ctx context.Context, req UninstallRequest) (struct{}, error) {
	// teleport-update uninstall can leave a disabled unit file behind, which
	// does not count as an install.
	if !slices.ContainsFunc([]string{scopedConfigPath, scopedDataDir, scopedInstallDir}, h.exists) {
		return struct{}{}, nil
	}
	// remedy ends a refusal with rollback's advice. Promote's caller gives its
	// own, so under promote a refusal is only the reason.
	remedy := func(advice string) string {
		if req.Promote {
			return ""
		}
		return "; " + advice
	}
	if !h.exists(scopedConfigPath) {
		return struct{}{}, trace.CompareFailed("nothing at %s proves the install under teleport_migrated is this migration's%s", scopedConfigPath, remedy("roll it back by hand"))
	}
	unlock, err := utils.FSTryWriteLock(h.path(scopedConfigPath + ".lock"))
	if errors.Is(err, utils.ErrUnsuccessfulLockTry) {
		return struct{}{}, trace.AlreadyExists("another migrate command is installing on this host%s", remedy("wait for it, then roll back"))
	}
	if err != nil {
		return struct{}{}, trace.Wrap(err, "locking %q", scopedConfigPath+".lock")
	}
	defer unlock()
	ownerRemedy := "roll it back by hand"
	if req.Promote {
		ownerRemedy = ""
	}
	if err := h.checkConfigOwner("scoped", scopedConfigPath, req.Proxy, req.SourceHostUUID, ownerRemedy, req.Promote); err != nil {
		return struct{}{}, trace.Wrap(err)
	}
	units := []string{scopedUpdaterTimer, scopedUpdaterService, scopedUnit}
	for _, unit := range units {
		if !h.exists(path.Join("/etc/systemd/system", unit)) {
			continue
		}
		// A failed stop is refused here, not left to the active check below: the
		// updater service is a oneshot, which systemd reports as activating, not
		// active, while an update runs.
		if err := runCommand(ctx, "systemctl", "stop", unit); err != nil {
			return struct{}{}, trace.Wrap(err, "stopping %s%s", unit, remedy("stop it, then roll back"))
		}
		if err := runCommand(ctx, "systemctl", "disable", unit); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		}
	}
	// The removal below is recursive, so these must be stopped, not just not
	// active: a start queued behind the stop reports activating.
	for _, unit := range units {
		stopped, state, err := unitStopped(ctx, unit)
		if err != nil {
			return struct{}{}, trace.Wrap(err, "the scoped units are stopped and disabled and nothing was removed; enable and start %s and %s to keep the scoped agent", scopedUnit, scopedUpdaterTimer)
		}
		if !stopped {
			return struct{}{}, trace.CompareFailed("%s is %s, not stopped; stop it and retry", unit, state)
		}
	}
	if h.exists(scopedInstallDir) {
		updater := "teleport-update"
		if bundled := h.path(scopedUpdaterPath); isExecutable(bundled) {
			updater = bundled
		}
		if err := runCommand(ctx, updater, "--install-suffix", scopedInstallSuffix, "uninstall"); err != nil {
			return struct{}{}, trace.Wrap(err, "teleport-update uninstall failed with %s stopped and disabled%s", scopedUnit, remedy("run teleport-update --install-suffix "+scopedInstallSuffix+" uninstall by hand, then roll back"))
		}
	}
	if err := os.RemoveAll(h.path(scopedDataDir)); err != nil {
		return struct{}{}, trace.ConvertSystemError(err)
	}
	if err := os.Remove(h.path(scopedConfigPath)); err != nil && !os.IsNotExist(err) {
		return struct{}{}, trace.ConvertSystemError(err)
	}
	if req.Promote {
		h.removeFetchCache()
	}
	return struct{}{}, nil
}

// removeFetchCache removes the fetch cache directory this binary runs from,
// which no later step on the host needs, and the cache root once it is empty.
// A binary run from anywhere else is left alone. A failure is a warning on
// stderr, not an error, since the step's real work is already done and a rerun
// could not resume.
func (h hostEnv) removeFetchCache() {
	dir := filepath.Dir(h.exe)
	if filepath.Dir(dir) != h.path(CacheRoot) {
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		fmt.Fprintf(os.Stderr, "removing the fetch cache %s: %v\n", dir, err)
		return
	}
	// Remove fails while another version's dir remains, which is fine.
	_ = os.Remove(filepath.Dir(dir))
}
