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

package kubeconfig

import (
	"fmt"
	"maps"
	"slices"

	"github.com/gravitational/trace"
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
	return slices.Collect(maps.Keys(teleportClusters))
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
	// OverrideContext is the name of the context or template used when adding a new cluster.
	// If empty, the context name will be generated from the {teleport-cluster}-{kube-cluster}.
	OverrideContext string
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
func CreateLocalProxyConfig(originalKubeConfig *clientcmdapi.Config, localProxyValues *LocalProxyValues) (*clientcmdapi.Config, error) {
	prevContext := originalKubeConfig.CurrentContext

	contextTmpl, err := parseContextOverrideTemplate(localProxyValues.OverrideContext)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Make a deep copy from default config then remove existing Teleport
	// entries before adding the ones for local proxy.
	config := originalKubeConfig.DeepCopy()
	config.CurrentContext = ""
	removeByServerAddr(config, localProxyValues.TeleportKubeClusterAddr)

	for _, cluster := range localProxyValues.Clusters {
		contextName := ContextName(cluster.TeleportCluster, cluster.KubeCluster)
		if contextTmpl != nil {
			if contextName, err = executeKubeContextTemplate(contextTmpl, cluster.TeleportCluster, cluster.KubeCluster); err != nil {
				return nil, trace.Wrap(err)
			}
		}

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
	return config, nil
}

// LocalProxyClustersFromDefaultConfig loads Teleport kube clusters data saved
// by `tsh kube login` in the default kubeconfig.
func LocalProxyClustersFromDefaultConfig(defaultConfig *clientcmdapi.Config, clusterAddr string) (clusters LocalProxyClusters) {
	for teleportClusterName, cluster := range defaultConfig.Clusters {
		if cluster.Server != clusterAddr {
			continue
		}

		for contextName, ctx := range defaultConfig.Contexts {
			if ctx.Cluster != teleportClusterName {
				continue
			}
			auth, found := defaultConfig.AuthInfos[contextName]
			if !found {
				continue
			}

			clusters = append(clusters, LocalProxyCluster{
				TeleportCluster:   teleportClusterName,
				KubeCluster:       KubeClusterFromContext(contextName, ctx, teleportClusterName),
				Namespace:         ctx.Namespace,
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
		KubeCluster:       KubeClusterFromContext(contextName, context, context.Cluster),
		Namespace:         context.Namespace,
		Impersonate:       auth.Impersonate,
		ImpersonateGroups: auth.ImpersonateGroups,
	}, true
}
