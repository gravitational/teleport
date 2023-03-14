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
	"github.com/gravitational/trace"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils"
)

// LocalProxyClusterValues contains values for a kube cluster for generating
// local proxy kubeconfig.
type LocalProxyClusterValues struct {
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
	// KubeClusters is a list of kubernetes clusters to generate contexts for.
	KubeClusters []string
}

// ContextName returns a kubeconfig context name for this kube cluster.
func (v *LocalProxyClusterValues) ContextName() string {
	return ContextName(v.TeleportCluster, v.KubeCluster)
}

// TLSServerName returns the TLSServerName  for this kube cluster.
func (v *LocalProxyClusterValues) TLSServerName() string {
	return v.KubeCluster + constants.KubeTeleportLocalProxyDelimiter + v.TeleportCluster
}

// LocalProxyValues contains values for generating local proxy kubeconfig
type LocalProxyValues struct {
	// LocalProxyCAPath is the path to local proxy's self-signed CA.
	LocalProxyCAPath string
	// LocalProxyAddr is the local proxy's server address.
	LocalProxyAddr string
	// ClientKeyPath is the path to the client key.
	ClientKeyPath string
	// ClientKeyPath is the path to the client client.
	CliertCertPath string
	// Clusters is a list of Teleport kube clusters to include.
	Clusters []LocalProxyClusterValues
}

// TeleportClusterNames returns all Teleport cluster names.
func (v *LocalProxyValues) TeleportClusterNames() []string {
	names := make([]string, 0, len(v.Clusters))
	for i := range v.Clusters {
		names = append(names, v.Clusters[i].TeleportCluster)
	}
	return utils.Deduplicate(names)
}

// SaveLocalProxyValues creates a kubeconfig for local proxy.
func SaveLocalProxyValues(path, clusterAddr string, defaultConfig *clientcmdapi.Config, localProxyValues *LocalProxyValues) error {
	prevContext := defaultConfig.CurrentContext

	// Make a deep copy from default config then remove existing Teleport
	// entries before adding the ones for local proxy.
	config := defaultConfig.DeepCopy()
	config.CurrentContext = ""
	removeByServerAddr(config, clusterAddr)

	for _, cluster := range localProxyValues.Clusters {
		contextName := cluster.ContextName()

		config.Clusters[contextName] = &clientcmdapi.Cluster{
			Server:               localProxyValues.LocalProxyAddr,
			CertificateAuthority: localProxyValues.LocalProxyCAPath,
			TLSServerName:        cluster.TLSServerName(),
		}
		config.Contexts[contextName] = &clientcmdapi.Context{
			Namespace: cluster.Namespace,
			Cluster:   contextName,
			AuthInfo:  contextName,
		}
		config.AuthInfos[contextName] = &clientcmdapi.AuthInfo{
			ClientCertificate: localProxyValues.CliertCertPath,
			ClientKey:         localProxyValues.ClientKeyPath,
			Impersonate:       cluster.Impersonate,
			ImpersonateGroups: cluster.ImpersonateGroups,
		}

		// Set the first as current context or if matching the one from default
		// kubeconfig.
		if config.CurrentContext == "" || prevContext == contextName {
			config.CurrentContext = contextName
		}
	}
	return trace.Wrap(Save(path, *config))
}

// LocalProxyClustersFromDefaultConfig loads Teleport kube clusters data saved
// by `tsh kube login` in the default kubeconfig.
func LocalProxyClustersFromDefaultConfig(defaultConfig *clientcmdapi.Config, clusterAddr string) (clusters []LocalProxyClusterValues) {
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

			clusters = append(clusters, LocalProxyClusterValues{
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
