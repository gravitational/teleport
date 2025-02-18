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
	"errors"
	"regexp"
	"strings"
)

// PodmanParser parses the cgroup mount path for Podman pods and containers.
func PodmanParser(mountPath string) (*Info, error) {
	info := &Info{}

	switch {
	case strings.HasPrefix(mountPath, "/libpod_parent"):
		// Rootful cgroupfs.
		//
		// https://github.com/containers/podman/blob/62cde17193e1469de15995ba78bd909fd07dffc6/libpod/container.go#L28-L29
		info.Rootfulness = Rootful
	case strings.HasPrefix(mountPath, "/machine.slice"):
		// Rootful systemd.
		//
		// https://github.com/containers/podman/blob/62cde17193e1469de15995ba78bd909fd07dffc6/libpod/container.go#L31-L33
		info.Rootfulness = Rootful
	case strings.HasPrefix(mountPath, "/user.slice"):
		// Rootless systemd and cgroupfs.
		//
		// https://github.com/containers/podman/blob/62cde17193e1469de15995ba78bd909fd07dffc6/libpod/container.go#L35-L37
		info.Rootfulness = Rootless
	}

	// Container is running in a pod.
	matches := podmanPodIDRegex.FindStringSubmatch(mountPath)
	if len(matches) == 4 {
		info.ID = matches[3]
		info.PodID = matches[1]
		return info, nil
	}

	// Container is running on its own.
	matches = podmanContainerIDRegex.FindStringSubmatch(mountPath)
	if len(matches) == 2 {
		info.ID = matches[1]
		return info, nil
	}

	// Unusual edge case: the machine has systemd enabled but the user explicitly
	// created the container with `--cgroup-manager cgroupfs` - in which case the
	// cgroup name contains a PID unrelated to the container. Only affects non-root
	// users.
	//
	// We could probably recover from this by reading a different mount point.
	//
	// https://github.com/containers/podman/blob/62cde17193e1469de15995ba78bd909fd07dffc6/pkg/domain/infra/abi/system_linux.go#L54
	if podmanInvalidCgroupRegex.MatchString(mountPath) {
		return nil, errors.New(
			"unable to determine container id from cgroup, this may be because the container was created with `--cgroup-manager cgroupfs` on a systemd-enabled machine",
		)
	}

	return nil, errors.New("cgroup path does not include a container id")
}

var (
	podmanPodIDRegex         = regexp.MustCompile(`libpod_pod_(?P<podID>[[:xdigit:]]{64})(.+)libpod-(?P<containerID>[[:xdigit:]]{64})`)
	podmanContainerIDRegex   = regexp.MustCompile(`libpod-(?P<containerID>[[:xdigit:]]{64})`)
	podmanInvalidCgroupRegex = regexp.MustCompile(`podman-(\d+).scope`)
)
