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

//nolint:goimports // goimports disagree with gci on blank imports
package proxy

import (
	"context"

	"github.com/gravitational/trace"
	// Load kubeconfig auth plugins for gcp and azure.
	// Without this, users can't provide a kubeconfig using those.
	//
	// Note: we don't want to load _all_ plugins. This is a balance between
	// support for popular hosting providers and minimizing attack surface.
	_ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/kube/internal/creds"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
)

// getKubeDetails fetches the kubernetes API credentials.
//
// There are 2 possible sources of credentials:
//   - pod service account credentials: files in hardcoded paths when running
//     inside of a k8s pod; this is used when kubeClusterName is set
//   - kubeconfig: a file with a set of k8s endpoints and credentials mapped to
//     them this is used when kubeconfigPath is set
//
// serviceType changes the loading behavior:
// - LegacyProxyService:
//   - if loading from kubeconfig, only "current-context" is returned; the
//     returned map key matches tpClusterName
//   - if no credentials are loaded, no error is returned
//   - permission self-test failures are only logged
//
// - ProxyService:
//   - no credentials are loaded and no error is returned
//
// - KubeService:
//   - if loading from kubeconfig, all contexts are returned
//   - if no credentials are loaded, returns an error
//   - permission self-test failures cause an error to be returned
func (f *Forwarder) getKubeDetails(ctx context.Context) error {
	serviceType := f.cfg.KubeServiceType
	kubeconfigPath := f.cfg.KubeconfigPath
	kubeClusterName := f.cfg.KubeClusterName
	tpClusterName := f.cfg.ClusterName

	f.log.DebugContext(ctx, "Reading Kubernetes details",
		"kubeconfig_path", kubeconfigPath,
		"kube_cluster_name", kubeClusterName,
		"service_type", serviceType,
	)

	// Proxy service should never have creds, forwards to kube service
	if serviceType == ProxyService {
		return nil
	}

	// Load kubeconfig or local pod credentials.
	loadAll := serviceType == KubeService
	cfg, err := kubeutils.GetKubeConfig(kubeconfigPath, loadAll, kubeClusterName)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	if trace.IsNotFound(err) || len(cfg.Contexts) == 0 {
		switch serviceType {
		case KubeService:
			return trace.BadParameter("no Kubernetes credentials found; Kubernetes_service requires either a valid kubeconfig_file or to run inside of a Kubernetes pod")
		case LegacyProxyService:
			f.log.DebugContext(ctx, "Could not load Kubernetes credentials. This proxy will still handle Kubernetes requests for trusted teleport clusters or Kubernetes nodes in this teleport cluster")
		}
		return nil
	}

	if serviceType == LegacyProxyService {
		// Hack for legacy proxy service - register a k8s cluster named after
		// the teleport cluster name to route legacy requests.
		//
		// Also, remove all other contexts. Multiple kubeconfig entries are
		// only supported for kubernetes_service.
		if currentContext, ok := cfg.Contexts[cfg.CurrentContext]; ok {
			cfg.Contexts = map[string]*rest.Config{
				tpClusterName: currentContext,
			}
		} else {
			return trace.BadParameter("no Kubernetes current-context found; Kubernetes proxy service requires either a valid kubeconfig_file with a current-context or to run inside of a Kubernetes pod")
		}
	}

	// Convert kubeconfig contexts into kubeCreds.
	for cluster, clientCfg := range cfg.Contexts {
		clusterCreds, err := creds.ExtractKubeCreds(ctx, serviceType, cluster, clientCfg, f.log, f.cfg.CheckImpersonationPermissions)
		if err != nil {
			f.log.WarnContext(ctx, "failed to load credentials for cluster",
				"cluster", cluster,
				"error", err,
			)
			continue
		}
		kubeCluster, err := types.NewKubernetesClusterV3(types.Metadata{
			Name: cluster,
		}, types.KubernetesClusterSpecV3{})
		if err != nil {
			f.log.WarnContext(ctx, "failed to create KubernetesClusterV3 from credentials for cluster",
				"cluster", cluster,
				"error", err,
			)
			continue
		}

		details, err := newClusterDetails(ctx,
			clusterDetailsConfig{
				cluster:   kubeCluster,
				kubeCreds: clusterCreds,
				log:       f.log.With("cluster", kubeCluster.GetName()),
				checker:   f.cfg.CheckImpersonationPermissions,
				component: serviceType,
				clock:     f.cfg.Clock,
			})
		if err != nil {
			f.log.WarnContext(ctx, "Failed to create cluster details for cluster",
				"cluster", cluster,
				"error", err,
			)
			return trace.Wrap(err)
		}
		f.clusterDetails[cluster] = details
	}
	return nil
}
