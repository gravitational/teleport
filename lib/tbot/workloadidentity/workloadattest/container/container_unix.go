//go:build unix

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
	"regexp"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
	"k8s.io/utils/mount"
)

// LookupPID finds the container and pod to which the process with the given PID
// belongs by examining its mountinfo and cgroup.
//
// rootPath allows you to optionally override the system root path in tests,
// pass an empty string to use the real root.
func LookupPID(rootPath string, pid int) (*Identifiers, error) {
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

	ids, err := parseCgroupMount(cgroupMount.Root)
	if err != nil {
		return nil, trace.Wrap(
			err, "parsing cgroup mount (root: %q)", cgroupMount.Root,
		)
	}
	return ids, nil
}

// parseCgroupMount takes the source of the cgroup mountpoint and extracts the
// container ID and pod ID from it.
//
// Note: this is a fairly naive implementation, we may need to make further
// improvements to account for other distributions of Kubernetes.
//
// There's a collection of real world mountfiles in testdata/mountfile.
func parseCgroupMount(source string) (*Identifiers, error) {
	matches := containerIDRegex.FindStringSubmatch(source)
	if len(matches) != 2 {
		return nil, trace.BadParameter(
			"expected 2 matches searching for container ID but found %d",
			len(matches),
		)
	}

	containerID := matches[1]
	if containerID == "" {
		return nil, trace.BadParameter(
			"source does not contain container ID",
		)
	}

	matches = podIDRegex.FindStringSubmatch(source)
	if len(matches) != 2 {
		return nil, trace.BadParameter(
			"expected 2 matches searching for pod ID but found %d",
			len(matches),
		)
	}
	podID := matches[1]
	if podID == "" {
		return nil, trace.BadParameter(
			"source does not contain pod ID",
		)
	}

	// When using the `systemd` cgroup driver, the dashes are replaced with
	// underscores. So let's correct that.
	podID = strings.ReplaceAll(podID, "_", "-")

	return &Identifiers{
		ContainerID: containerID,
		PodID:       podID,
	}, nil
}

var (
	// A container ID is usually a 64 character hex string, so this regex just
	// selects for that.
	containerIDRegex = regexp.MustCompile(`(?P<containerID>[[:xdigit:]]{64})`)

	// A pod ID is usually a UUID prefaced with "pod".
	// There are two main cgroup drivers:
	// - systemd , the dashes are replaced with underscores
	// - cgroupfs, the dashes are kept.
	podIDRegex = regexp.MustCompile(`pod(?P<podID>[[:xdigit:]]{8}[_-][[:xdigit:]]{4}[_-][[:xdigit:]]{4}[_-][[:xdigit:]]{4}[_-][[:xdigit:]]{12})`)
)
