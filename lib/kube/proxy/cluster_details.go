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

package proxy

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/kube/internal/creds"
	"github.com/gravitational/teleport/lib/labels"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
)

// kubeDetails contain the cluster-related details including authentication.
type kubeDetails struct {
	creds.KubeCreds

	// dynamicLabels is the dynamic labels executor for this cluster.
	dynamicLabels *labels.Dynamic
	// kubeCluster is the dynamic kube_cluster or a static generated from kubeconfig and that only has the name populated.
	kubeCluster types.KubeCluster
	// kubeClusterVersion is the version of the kube_cluster's related Kubernetes server.
	kubeClusterVersion *version.Info

	// rwMu is the mutex to protect the kubeCodecs, gvkSupportedResources, and rbacSupportedTypes.
	rwMu sync.RWMutex
	// kubeCodecs is the codec factory for the cluster resources.
	// The codec factory includes the default resources and the namespaced resources
	// that are supported by the cluster.
	// The codec factory is updated periodically to include the latest custom resources
	// that are added to the cluster.
	kubeCodecs *serializer.CodecFactory
	// rbacSupportedTypes is the list of supported types for RBAC for the cluster.
	// The list is updated periodically to include the latest custom resources
	// that are added to the cluster.
	rbacSupportedTypes rbacSupportedResources
	// gvkSupportedResources is the list of registered API path resources and their
	// GVK definition.
	gvkSupportedResources gvkSupportedResources
	// isClusterOffline is true if the cluster is offline.
	// An offline cluster will not be able to serve any requests until it comes back online.
	// The cluster is marked as offline if the cluster schema cannot be created
	// and the list of supported types for RBAC cannot be generated.
	isClusterOffline bool

	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
}

type clusterCredentialsGetter interface {
	GetKubeClusterCredentials(ctx context.Context, cluster types.KubeCluster) (creds.KubeCreds, error)
}

// clusterDetailsConfig contains the configuration for creating a proxied cluster.
type clusterDetailsConfig struct {
	// kubeCreds is the credentials to use for the cluster.
	kubeCreds creds.KubeCreds
	// cluster is the cluster to create a proxied cluster for.
	cluster types.KubeCluster
	// log is the logger to use.
	log *slog.Logger
	// checker is the permissions checker to use.
	checker servicecfg.ImpersonationPermissionsChecker
	// resourceMatchers is the list of resource matchers to match the cluster against
	// to determine if we should assume the role or not for AWS.
	resourceMatchers []services.ResourceMatcher
	// clock is the clock to use.
	clock clockwork.Clock
	// component is the Kubernetes component that serves this cluster.
	component         KubeServiceType
	credentialsGetter clusterCredentialsGetter
}

const (
	defaultRefreshPeriod = 5 * time.Minute
	backoffRefreshStep   = 10 * time.Second
)

// newClusterDetails creates a proxied kubeDetails structure given a dynamic cluster.
func newClusterDetails(ctx context.Context, cfg clusterDetailsConfig) (_ *kubeDetails, err error) {
	creds := cfg.kubeCreds
	if creds == nil {
		creds, err = cfg.credentialsGetter.GetKubeClusterCredentials(ctx, cfg.cluster)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	var dynLabels *labels.Dynamic
	if len(cfg.cluster.GetDynamicLabels()) > 0 {
		dynLabels, err = labels.NewDynamic(
			ctx,
			&labels.DynamicConfig{
				Labels: cfg.cluster.GetDynamicLabels(),
				Log:    cfg.log,
			})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		dynLabels.Sync()
		go dynLabels.Start()
	}

	var isClusterOffline bool
	// Create the codec factory and the list of supported types for RBAC.
	codecFactory, rbacSupportedTypes, gvkSupportedRes, err := newClusterSchemaBuilder(cfg.log, creds.GetKubeClient())
	if err != nil {
		cfg.log.WarnContext(ctx, "Failed to create cluster schema, the cluster may be offline", "error", err)
		// If the cluster is offline, we will not be able to create the codec factory
		// and the list of supported types for RBAC.
		// We mark the cluster as offline and continue to create the kubeDetails but
		// the offline cluster will not be able to serve any requests until it comes back online.
		isClusterOffline = true
	}

	kubeVersion, err := creds.GetKubeClient().Discovery().ServerVersion()
	if err != nil {
		cfg.log.WarnContext(ctx, "Failed to get Kubernetes cluster version, the cluster may be offline", "error", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	k := &kubeDetails{
		KubeCreds:             creds,
		dynamicLabels:         dynLabels,
		kubeCluster:           cfg.cluster,
		kubeClusterVersion:    kubeVersion,
		kubeCodecs:            codecFactory,
		rbacSupportedTypes:    rbacSupportedTypes,
		cancelFunc:            cancel,
		isClusterOffline:      isClusterOffline,
		gvkSupportedResources: gvkSupportedRes,
	}

	// If cluster is online and there's no errors, we refresh details seldom (every 5 minutes),
	// but if the cluster is offline, we try to refresh details more often to catch it getting back online earlier.
	firstPeriod := defaultRefreshPeriod
	if isClusterOffline {
		firstPeriod = backoffRefreshStep
	}
	refreshDelay, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  firstPeriod,
		Step:   backoffRefreshStep,
		Max:    defaultRefreshPeriod,
		Jitter: retryutils.SeventhJitter,
		Clock:  cfg.clock,
	})
	if err != nil {
		k.Close()
		return nil, trace.Wrap(err)
	}

	k.wg.Add(1)
	// Start the periodic update of the codec factory and the list of supported types for RBAC.
	go func() {
		defer k.wg.Done()

		for {
			select {
			case <-ctx.Done():
				return
			case <-refreshDelay.After():
				codecFactory, rbacSupportedTypes, gvkSupportedResources, err := newClusterSchemaBuilder(cfg.log, creds.GetKubeClient())
				if err != nil {
					// If this is first time we get an error, we reset retry mechanism so it will start trying to refresh details quicker, with linear backoff.
					if refreshDelay.First == defaultRefreshPeriod {
						refreshDelay.First = backoffRefreshStep
						refreshDelay.Reset()
					} else {
						refreshDelay.Inc()
					}
					cfg.log.ErrorContext(ctx, "Failed to update cluster schema", "error", err)
					continue
				}

				kubeVersion, err := creds.GetKubeClient().Discovery().ServerVersion()
				if err != nil {
					cfg.log.WarnContext(ctx, "Failed to get Kubernetes cluster version, the cluster may be offline", "error", err)
				}

				// Restore details refresh delay to the default value, in case previously cluster was offline.
				refreshDelay.First = defaultRefreshPeriod

				k.rwMu.Lock()
				k.kubeCodecs = codecFactory
				k.rbacSupportedTypes = rbacSupportedTypes
				k.gvkSupportedResources = gvkSupportedResources
				k.isClusterOffline = false
				k.kubeClusterVersion = kubeVersion
				k.rwMu.Unlock()
			}
		}
	}()
	return k, nil
}

func (k *kubeDetails) Close() {
	// send a close signal and wait for the close to finish.
	k.cancelFunc()
	k.wg.Wait()
	if k.dynamicLabels != nil {
		k.dynamicLabels.Close()
	}
	// it is safe to call close even for static creds.
	k.KubeCreds.Close()
}

// getClusterSupportedResources returns the codec factory and the list of supported types for RBAC.
func (k *kubeDetails) getClusterSupportedResources() (*serializer.CodecFactory, rbacSupportedResources, error) {
	k.rwMu.RLock()
	defer k.rwMu.RUnlock()
	// If the cluster is offline, return an error because we don't have the schema
	// for the cluster.
	if k.isClusterOffline {
		return nil, nil, trace.ConnectionProblem(nil, "kubernetes cluster %q is offline", k.kubeCluster.GetName())
	}
	return k.kubeCodecs, k.rbacSupportedTypes, nil
}

// getObjectGVK returns the default GVK (if any) registered for the specified request path.
func (k *kubeDetails) getObjectGVK(resource apiResource) *schema.GroupVersionKind {
	k.rwMu.RLock()
	defer k.rwMu.RUnlock()
	return k.gvkSupportedResources[gvkSupportedResourcesKey{
		name:     strings.Split(resource.resourceKind, "/")[0],
		apiGroup: resource.apiGroup,
		version:  resource.apiGroupVersion,
	}]
}

// getStaticCredentialsFromKubeconfig loads a kubeconfig from the cluster and returns the access credentials for the cluster.
// If the config defines multiple contexts, it will pick one (the order is not guaranteed).
func getStaticCredentialsFromKubeconfig(ctx context.Context, component KubeServiceType, cluster types.KubeCluster, log *slog.Logger, checker servicecfg.ImpersonationPermissionsChecker) (*creds.StaticKubeCreds, error) {
	config, err := clientcmd.Load(cluster.GetKubeconfig())
	if err != nil {
		return nil, trace.WrapWithMessage(err, "unable to parse kubeconfig for cluster %q", cluster.GetName())
	}
	if len(config.CurrentContext) == 0 && len(config.Contexts) > 0 {
		// select the first context key as default context
		for k := range config.Contexts {
			config.CurrentContext = k
			break
		}
	}
	restConfig, err := clientcmd.NewDefaultClientConfig(*config, nil).ClientConfig()
	if err != nil {
		return nil, trace.WrapWithMessage(err, "unable to create client from kubeconfig for cluster %q", cluster.GetName())
	}

	out, err := creds.ExtractKubeCreds(ctx, component, cluster.GetName(), restConfig, log, checker)
	return out, trace.Wrap(err)
}
