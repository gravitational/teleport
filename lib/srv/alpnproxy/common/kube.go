/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"encoding/hex"
	"fmt"
	"strings"
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

// KubeLocalProxyWildcardDomain returns the wildcard domain used to generate
// local self-signed CA for provided Teleport cluster.
func KubeLocalProxyWildcardDomain(teleportCluster string) string {
	return "*." + teleportCluster
}
