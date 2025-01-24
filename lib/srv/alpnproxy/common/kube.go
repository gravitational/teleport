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
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/gravitational/trace"
)

// KubeLocalProxySNI generates the SNI used for Kube local proxy.
func KubeLocalProxySNI(teleportCluster, kubeCluster string) string {
	// Hex encode to hide "." in kube cluster name so wildcard cert can be used:
	// <hex-encoded-kube-cluster>.<teleport-cluster>
	return fmt.Sprintf("%s.%s", hex.EncodeToString([]byte(kubeCluster)), teleportCluster)
}

// TeleportClusterFromKubeLocalProxySNI returns Teleport cluster name from SNI.
func TeleportClusterFromKubeLocalProxySNI(serverName string) string {
	_, teleportCluster, _ := strings.Cut(serverName, ".")
	return teleportCluster
}

// KubeClusterFromKubeLocalProxySNI returns Kubernetes cluster name from SNI.
func KubeClusterFromKubeLocalProxySNI(serverName string) (string, error) {
	kubeCluster, _, _ := strings.Cut(serverName, ".")
	str, err := hex.DecodeString(kubeCluster)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(str), nil
}

// KubeLocalProxyWildcardDomain returns the wildcard domain used to generate
// local self-signed CA for provided Teleport cluster.
func KubeLocalProxyWildcardDomain(teleportCluster string) string {
	return "*." + teleportCluster
}
