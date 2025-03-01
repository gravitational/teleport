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
	"regexp"
	"strings"

	"github.com/gravitational/trace"
)

var (
	// A container ID is usually a 64 character hex string, so this regex just
	// selects for that.
	k8sContainerIDRegex = regexp.MustCompile(`(?P<containerID>[[:xdigit:]]{64})`)

	// A pod ID is usually a UUID prefaced with "pod".
	// There are two main cgroup drivers:
	// - systemd , the dashes are replaced with underscores
	// - cgroupfs, the dashes are kept.
	k8sPodIDRegex = regexp.MustCompile(`pod(?P<podID>[[:xdigit:]]{8}[_-][[:xdigit:]]{4}[_-][[:xdigit:]]{4}[_-][[:xdigit:]]{4}[_-][[:xdigit:]]{12})`)
)

// KubernetesParser parses the cgroup mount path for Kubernetes pods.
func KubernetesParser(source string) (*Info, error) {
	matches := k8sContainerIDRegex.FindStringSubmatch(source)
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

	matches = k8sPodIDRegex.FindStringSubmatch(source)
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

	return &Info{
		ID:    containerID,
		PodID: podID,
	}, nil
}
