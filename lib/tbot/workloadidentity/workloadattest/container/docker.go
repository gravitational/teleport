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

var (
	// Note: this regex has been tested against the systemd and cgroupfs drivers
	// on systemd-enabled machines. It hasn't yet been tested on a system without
	// systemd at all but that's quite an uncommon configuration.
	dockerContainerIDRegex = regexp.MustCompile(`(docker-|docker/|app.slice/)(?P<containerID>[[:xdigit:]]{64})`)
)

// DockerParser parses the cgroup mount path for Docker containers.
func DockerParser(mountPath string) (*Info, error) {
	matches := dockerContainerIDRegex.FindStringSubmatch(mountPath)
	if len(matches) != 3 {
		return nil, errors.New("cgroup path does not include a container id")
	}

	info := &Info{ID: matches[2]}

	switch {
	case strings.HasPrefix(mountPath, "/system.slice"):
		// Rootful systemd.
		info.Rootfulness = Rootful
	case strings.HasPrefix(mountPath, "/docker"):
		// Rootful cgroupfs.
		info.Rootfulness = Rootful
	case strings.HasPrefix(mountPath, "/user.slice"):
		// Rootless systemd (and cgroupfs on systemd-enabled machines).
		info.Rootfulness = Rootless
	}

	return info, nil
}
