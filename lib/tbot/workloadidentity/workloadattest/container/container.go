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

package container

import (
	"path"
	"strconv"

	"github.com/gravitational/trace"
	"k8s.io/utils/mount"
)

// Rootfulness describes whether a container was started by an unprivileged user
// as in "rootless" Podman or by root. It's a best guess based on information
// gleaned from procfs, not authoratative.
type Rootfulness int

const (
	// RootfulnessUnknown means we were unable to infer whether the container is
	// rootless or not.
	RootfulnessUnknown Rootfulness = iota

	// Rootful means the container was probably started by root.
	Rootful

	// Rootless means the container was probably started by an unprivileged user.
	Rootless
)

// Info holds the information discovered about the container.
type Info struct {
	// ID is the container's  ID.
	ID string

	// PodID identifies to which "pod" the container belongs, in engines that
	// support pods such as Kubernetes and Podman.
	PodID string

	// Rootfulness describes whether a container was started by an unprivileged
	// user as in "rootless" Podman or by root. It's a best guess based on
	// information gleaned from procfs, not authoratative.
	Rootfulness Rootfulness
}

// Parser parses the cgroup mount path to extract the container and pod IDs.
//
// This information is encoded differently by the container runtimes.
type Parser func(mountPath string) (*Info, error)

// LookupPID discovers information about the container in which the process with
// the given PID is running, by interrogating procfs.
//
// rootPath allows you to optionally override the system root path in tests,
// pass an empty string to use the real root.
func LookupPID(rootPath string, pid int, parser Parser) (*Info, error) {
	info, err := mount.ParseMountInfo(
		path.Join(rootPath, "/proc", strconv.Itoa(pid), "mountinfo"),
	)
	if err != nil {
		return nil, trace.Wrap(err, "parsing mountinfo")
	}

	// Find the cgroup or cgroupv2 mount.
	//
	// For cgroup v2, we expect a single mount. But for cgroup v1, there will
	// be one mount per subsystem, but regardless, they will all contain the
	// same container ID/pod ID.
	var cgroupMount mount.MountInfo
	for _, m := range info {
		if m.FsType == "cgroup" || m.FsType == "cgroup2" {
			cgroupMount = m
			break
		}
	}

	ids, err := parser(cgroupMount.Root)
	if err != nil {
		return nil, trace.Wrap(
			err, "parsing cgroup mount (root: %q)", cgroupMount.Root,
		)
	}
	return ids, nil
}
