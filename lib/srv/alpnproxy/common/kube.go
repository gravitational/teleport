/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package common

import (
	"strings"

	"github.com/gravitational/teleport/lib/kube/labelhash"
)

// KubeLocalProxySNI generates the SNI used for Kube local proxy.
//
// The kube cluster is encoded as a fixed-length hash prefixed with "k" to
// keep the first DNS label within RFC 1035's 63-byte limit for any input.
// Earlier versions hex-encoded the kube cluster name, which could exceed the
// limit and break strict TLS clients (e.g. the official Python kubernetes
// client). See issue #61439.
func KubeLocalProxySNI(teleportCluster, kubeCluster string) string {
	// "k" prefix keeps the label from starting with a digit
	// (compatibility with the historical convention in api/utils.EncodeClusterName).
	return "k" + labelhash.Encode(teleportCluster, kubeCluster) + "." + teleportCluster
}

// TeleportClusterFromKubeLocalProxySNI returns Teleport cluster name from SNI.
func TeleportClusterFromKubeLocalProxySNI(serverName string) string {
	_, teleportCluster, _ := strings.Cut(serverName, ".")
	return teleportCluster
}

// KubeLocalProxyWildcardDomain returns the wildcard domain used to generate
// local self-signed CA for provided Teleport cluster.
func KubeLocalProxyWildcardDomain(teleportCluster string) string {
	return "*." + teleportCluster
}
