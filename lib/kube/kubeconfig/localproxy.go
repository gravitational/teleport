// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubeconfig

import (
	"fmt"

	"golang.org/x/exp/maps"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
)

// LocalProxyCluster contains values for a kube cluster for generating
// local proxy kubeconfig.
type LocalProxyCluster struct {
	// TeleportCluster is the Teleport cluster name.
	TeleportCluster string
	// KubeCluster is the Kubernetes cluster name.
	KubeCluster string
	// Impersonate allows to define the default impersonated user.
	// Must be a subset of kubernetes_users or the Teleport username
	// otherwise Teleport will deny the request.
	Impersonate string
	// ImpersonateGroups allows to define the default values for impersonated groups.
	// Must be a subset of kubernetes_groups otherwise Teleport will deny
	// the request.
	ImpersonateGroups []string
	// Namespace allows to define the default namespace value.
	Namespace string
}

// String implements Stringer interface.
func (v LocalProxyCluster) String() string {
	return fmt.Sprintf("Teleport cluster %q Kubernetes cluster %q", v.TeleportCluster, v.KubeCluster)
}

// LocalProxyClusters is a list of LocalProxyCluster.
type LocalProxyClusters []LocalProxyCluster

// TeleportClusters returns a list of unique Teleport clusters
func (s LocalProxyClusters) TeleportClusters() []string {
	teleportClusters := make(map[string]struct{})
	for _, cluster := range s {
		teleportClusters[cluster.TeleportCluster] = struct{}{}
	}
	return maps.Keys(teleportClusters)
}

// LocalProxyValues contains values for generating local proxy kubeconfig
type LocalProxyValues struct {
	// TeleportKubeClusterAddr is the Teleport Kubernetes access address.
	TeleportKubeClusterAddr string
	// LocalProxyURL is the local forward proxy's URL.
	LocalProxyURL string
	// LocalProxyCAs are the local proxy's self-signed CAs PEM encoded data, by Teleport cluster name.
	LocalProxyCAs map[string][]byte
	// ClientKeyData is self generated private key data used by kubectl and linked to proxy self-signed CA
	ClientKeyData []byte
	// Clusters is a list of Teleport kube clusters to include.
	Clusters LocalProxyClusters
}

// TeleportClusterNames returns all Teleport cluster names.
func (v *LocalProxyValues) TeleportClusterNames() []string {
	names := make([]string, 0, len(v.Clusters))
	for i := range v.Clusters {
		names = append(names, v.Clusters[i].TeleportCluster)
	}
	return utils.Deduplicate(names)
}

// CreateLocalProxyConfig creates a kubeconfig for local proxy.
func CreateLocalProxyConfig(originalKubeConfig *clientcmdapi.Config, localProxyValues *LocalProxyValues) *clientcmdapi.Config {
	prevContext := originalKubeConfig.CurrentContext

	// Make a deep copy from default config then remove existing Teleport
	// entries before adding the ones for local proxy.
	config := originalKubeConfig.DeepCopy()
	config.CurrentContext = ""
	removeByServerAddr(config, localProxyValues.TeleportKubeClusterAddr)

	for _, cluster := range localProxyValues.Clusters {
		contextName := ContextName(cluster.TeleportCluster, cluster.KubeCluster)

		config.Clusters[contextName] = &clientcmdapi.Cluster{
			ProxyURL:                 localProxyValues.LocalProxyURL,
			Server:                   localProxyValues.TeleportKubeClusterAddr,
			CertificateAuthorityData: localProxyValues.LocalProxyCAs[cluster.TeleportCluster],
			TLSServerName:            common.KubeLocalProxySNI(cluster.TeleportCluster, cluster.KubeCluster),
		}
		config.Contexts[contextName] = &clientcmdapi.Context{
			Namespace: cluster.Namespace,
			Cluster:   contextName,
			AuthInfo:  contextName,
		}
		config.AuthInfos[contextName] = &clientcmdapi.AuthInfo{
			ClientCertificateData: localProxyValues.LocalProxyCAs[cluster.TeleportCluster],
			ClientKeyData:         localProxyValues.ClientKeyData,
			Impersonate:           cluster.Impersonate,
			ImpersonateGroups:     cluster.ImpersonateGroups,
		}

		// Set the first as current context or if matching the one from default
		// kubeconfig.
		if config.CurrentContext == "" || prevContext == contextName {
			config.CurrentContext = contextName
		}
	}
	return config
}

// LocalProxyClustersFromDefaultConfig loads Teleport kube clusters data saved
// by `tsh kube login` in the default kubeconfig.
func LocalProxyClustersFromDefaultConfig(defaultConfig *clientcmdapi.Config, clusterAddr string) (clusters LocalProxyClusters) {
	for teleportClusterName, cluster := range defaultConfig.Clusters {
		if cluster.Server != clusterAddr {
			continue
		}

		for contextName, context := range defaultConfig.Contexts {
			if context.Cluster != teleportClusterName {
				continue
			}
			auth, found := defaultConfig.AuthInfos[contextName]
			if !found {
				continue
			}

			clusters = append(clusters, LocalProxyCluster{
				TeleportCluster:   teleportClusterName,
				KubeCluster:       KubeClusterFromContext(contextName, teleportClusterName),
				Namespace:         context.Namespace,
				Impersonate:       auth.Impersonate,
				ImpersonateGroups: auth.ImpersonateGroups,
			})
		}
	}
	return clusters
}

// FindTeleportClusterForLocalProxy finds the Teleport kube cluster based on
// provided cluster address and context name, and prepares a LocalProxyCluster.
//
// When the cluster has a ProxyURL set, it means the provided kubeconfig is
// already pointing to a local proxy through this ProxyURL and thus can be
// skipped as there is no need to create a new local proxy.
func FindTeleportClusterForLocalProxy(defaultConfig *clientcmdapi.Config, clusterAddr, contextName string) (LocalProxyCluster, bool) {
	if contextName == "" {
		contextName = defaultConfig.CurrentContext
	}

	context, found := defaultConfig.Contexts[contextName]
	if !found {
		return LocalProxyCluster{}, false
	}
	cluster, found := defaultConfig.Clusters[context.Cluster]
	if !found || cluster.Server != clusterAddr || cluster.ProxyURL != "" {
		return LocalProxyCluster{}, false
	}
	auth, found := defaultConfig.AuthInfos[context.AuthInfo]
	if !found {
		return LocalProxyCluster{}, false
	}

	return LocalProxyCluster{
		TeleportCluster:   context.Cluster,
		KubeCluster:       KubeClusterFromContext(contextName, context.Cluster),
		Namespace:         context.Namespace,
		Impersonate:       auth.Impersonate,
		ImpersonateGroups: auth.ImpersonateGroups,
	}, true
}
