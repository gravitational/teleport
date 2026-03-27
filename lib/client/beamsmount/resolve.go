/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package beamsmount

import (
	"fmt"
	"io"
	"os"

	"github.com/gravitational/trace"
)

// UmountMode controls how the target argument to Umount is interpreted.
type UmountMode string

const (
	UmountModeAuto UmountMode = "auto"
	UmountModePath UmountMode = "path"
	UmountModeBeam UmountMode = "beam"
)

// UmountOptions holds the parameters for an unmount operation.
type UmountOptions struct {
	// Target is either a mount point path or a beam ID/alias.
	Target string
	// Force enables force unmount.
	Force bool
	// Mode controls target resolution (auto/path/beam).
	Mode UmountMode
	// All unmounts all tracked mounts for the current cluster.
	All bool
	// StateFile is the path to the cluster-scoped mount state file.
	StateFile string
	// Stdout is the writer for user-facing output.
	Stdout io.Writer
	// Stderr is the writer for warnings.
	Stderr io.Writer
}

// UmountAll unmounts all tracked mounts for the current cluster.
func UmountAll(opts UmountOptions) error {
	var mounts []MountEntry
	if err := WithStateLock(opts.StateFile, func(s *MountState) error {
		for _, w := range PruneStale(s) {
			fmt.Fprintln(opts.Stderr, "WARNING:", w)
		}
		mounts = append(mounts, s.Mounts...)
		return nil
	}); err != nil {
		return trace.Wrap(err)
	}

	if len(mounts) == 0 {
		fmt.Fprintln(opts.Stdout, "No tracked mounts to unmount.")
		return nil
	}

	var errors []error
	for _, m := range mounts {
		if err := unmountOne(m, opts); err != nil {
			errors = append(errors, err)
			fmt.Fprintf(opts.Stderr, "ERROR: failed to unmount %s: %v\n", m.MountPoint, err)
		}
	}

	if len(errors) > 0 {
		return trace.Errorf("failed to unmount %d of %d mounts", len(errors), len(mounts))
	}
	return nil
}

// UmountTarget unmounts the given target (path or beam reference).
func UmountTarget(opts UmountOptions) error {
	// Prune stale entries first.
	if err := WithStateLock(opts.StateFile, func(state *MountState) error {
		for _, w := range PruneStale(state) {
			fmt.Fprintln(opts.Stderr, "WARNING:", w)
		}
		return nil
	}); err != nil {
		return trace.Wrap(err)
	}

	switch opts.Mode {
	case UmountModePath:
		return trace.Wrap(umountByPath(opts))
	case UmountModeBeam:
		return trace.Wrap(umountByBeam(opts))
	case UmountModeAuto:
		return trace.Wrap(umountAuto(opts))
	default:
		return trace.BadParameter("unknown unmount mode %q", opts.Mode)
	}
}

// umountAuto tries path first; if the path doesn't exist on disk, falls back
// to beam ID/alias lookup.
func umountAuto(opts UmountOptions) error {
	if _, err := os.Stat(opts.Target); err == nil {
		return trace.Wrap(umountByPath(opts))
	}
	return trace.Wrap(umountByBeam(opts))
}

func umountByPath(opts UmountOptions) error {
	var entry *MountEntry
	if err := WithStateLock(opts.StateFile, func(state *MountState) error {
		entry = state.FindByMountPoint(opts.Target)
		return nil
	}); err != nil {
		return trace.Wrap(err)
	}

	if entry == nil {
		return trace.NotFound("no tracked mount at %s", opts.Target)
	}

	return trace.Wrap(unmountOne(*entry, opts))
}

func umountByBeam(opts UmountOptions) error {
	var entries []MountEntry
	if err := WithStateLock(opts.StateFile, func(state *MountState) error {
		entries = state.FindByBeam(opts.Target)
		return nil
	}); err != nil {
		return trace.Wrap(err)
	}

	if len(entries) == 0 {
		return trace.NotFound("no tracked mounts for beam %q", opts.Target)
	}

	var errors []error
	for _, m := range entries {
		if err := unmountOne(m, opts); err != nil {
			errors = append(errors, err)
			fmt.Fprintf(opts.Stderr, "ERROR: failed to unmount %s: %v\n", m.MountPoint, err)
		}
	}

	if len(errors) > 0 {
		return trace.Errorf("failed to unmount %d of %d mounts for beam %q",
			len(errors), len(entries), opts.Target)
	}
	return nil
}

func unmountOne(entry MountEntry, opts UmountOptions) error {
	fmt.Fprintf(opts.Stdout, "Unmounting %s (beam %s)\n", entry.MountPoint, beamDisplayName(entry))

	if err := Unmount(entry.MountPoint, opts.Force); err != nil {
		return trace.Wrap(err)
	}

	// Remove from state file after successful unmount.
	if err := WithStateLock(opts.StateFile, func(state *MountState) error {
		state.RemoveByMountPoint(entry.MountPoint)
		return nil
	}); err != nil {
		fmt.Fprintf(opts.Stderr, "WARNING: unmount succeeded but failed to update state file: %v\n", err)
	}

	return nil
}
