/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package resources

import (
	"slices"

	"github.com/go-logr/logr"
	"github.com/gravitational/trace"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/integrations/operator/controllers"
)

// ReconcilerFactory is a function that creates a reconciler from Kubernetes and Teleport clients.
type ReconcilerFactory func(client kclient.Client, tClient *client.Client) (controllers.Reconciler, error)

// Add new reconcilers here.
var supportedReconcilers = []ReconcilerFactory{
	NewAccessListReconciler,
	NewAccessMonitoringRuleV1Reconciler,
	NewAppV3Reconciler,
	NewAutoUpdateConfigV1Reconciler,
	NewAutoUpdateVersionV1Reconciler,
	NewBotV1Reconciler,
	NewDatabaseV3Reconciler,
	NewFooV1Reconciler,
	NewGithubConnectorReconciler,
	NewInferenceModelReconciler,
	NewInferencePolicyReconciler,
	NewInferenceSecretReconciler,
	NewLockV2Reconciler,
	NewLoginRuleReconciler,
	NewOIDCConnectorReconciler,
	NewOktaImportRuleReconciler,
	NewOpenSSHEICEServerV2Reconciler,
	NewOpenSSHServerV2Reconciler,
	NewProvisionTokenReconciler,
	NewRetrievalModelV1Reconciler,
	NewRoleReconciler,
	NewRoleV6Reconciler,
	NewRoleV7Reconciler,
	NewRoleV8Reconciler,
	NewSAMLConnectorReconciler,
	NewSAMLIdPServiceProviderV1Reconciler,
	NewScopedRoleV1Reconciler,
	NewScopedRoleAssignmentV1Reconciler,
	NewScopedTokenV1Reconciler,
	NewTrustedClusterV2Reconciler,
	NewUserReconciler,
	NewWorkloadIdentityV1Reconciler,
}

// SetupAllControllers sets up all controllers.
// A reconciler is enabled if:
// - its CRD exists in the clusters (supports a newer operator running against odler CRDs)
// - the operator is not running in scoped mode OR the operator is in scoped mode and the reconciler is scoped.
// - the reconciler support the cluster features (e.g. don't start a enterprise reconciler against an OSS cluster)
func SetupAllControllers(config Config, mgr manager.Manager, discoveryClient discovery.DiscoveryInterface) error {
	reconcilers, err := filterEnabledReconcilers(config, supportedReconcilers, discoveryClient)
	if err != nil {
		return trace.Wrap(err)
	}

	// Setup all enabled reconcilers.
	if len(reconcilers) == 0 {
		return trace.NotFound("No reconciler enabled, this is likely a mistake")
	}
	for _, reconciler := range reconcilers {
		if err := reconciler.SetupWithManager(mgr); err != nil {
			return trace.Wrap(err, "failed to setup controller for: %s", reconciler.GVK().Kind)
		}
		config.Log.Info("Reconciler setup successfully", "kubernetes_kind", reconciler.GVK().Kind, "teleport_kind", reconciler.TeleportKind())
	}

	return nil
}

// Config contains the configuration required to setup the resource reconcilers.
type Config struct {
	// Log is the logger used to send logs regarding the controller setup.
	// The controllers themselves use the logger from the query context.
	Log logr.Logger
	// TeleportClient is passed to controllers so they can interact with the Teleport cluster.
	TeleportClient *client.Client
	// KubeClient is used by the setup process to detect which CRDs are deployed in the cluster.
	// This is also passed to the controllers so they can get resources and write their status back.
	KubeClient kclient.Client
	// Scoped indicates that the operator is running in scoped mode.
	Scoped bool
	// Features are the features advertised by the Teleport cluster.
	// This is used to know which reconcilers should be started.
	Features *proto.Features
}

// gvkCache is a minimal opportunistic cache around the [discovery.DiscoveryInterface]
// so we don't query the entire GV for every GVK we want to check.
// This cache is not safe for concurrent access.
// Note: it is intentional to not use [discovery.CachedDiscoveryInterface] because the default
// cached implementation caches on the filesystem, and the memory implementation is not lazy
// and resolves every GVK on startup, doing potentially hundreds of calls.
// The RESTMapper is also not a good fit as it caches by GK and not by GV.
type gvkCache struct {
	cache map[schema.GroupVersion][]string
	clt   discovery.DiscoveryInterface
}

// lookupGVK checks if the GVK is supported by the cluster.
func (c *gvkCache) lookupGVK(gvk schema.GroupVersionKind) (bool, error) {
	gv := gvk.GroupVersion()
	if cached, ok := c.cache[gv]; ok {
		return slices.Contains(cached, gvk.Kind), nil
	}
	resp, err := c.clt.ServerResourcesForGroupVersion(gv.String())
	if err != nil {
		// GV not found, we cache the empty GVK list.
		if apierrors.IsNotFound(err) {
			c.cache[gv] = make([]string, 0)
			return false, nil
		}
		return false, trace.Wrap(err, "looking up resources for GroupVersion %s", gv.String())
	}
	// GV found, we cache the GVKs.
	kinds := make([]string, len(resp.APIResources))
	for i, r := range resp.APIResources {
		kinds[i] = r.Kind
	}
	c.cache[gv] = kinds
	return slices.Contains(c.cache[gv], gvk.Kind), nil
}

func filterEnabledReconcilers(c Config, reconcilers []ReconcilerFactory, discoveryClient discovery.DiscoveryInterface) ([]controllers.Reconciler, error) {
	gvks := &gvkCache{
		cache: make(map[schema.GroupVersion][]string),
		clt:   discoveryClient,
	}

	var enabledReconcilers []controllers.Reconciler
	// Check which reconcilers can and should be enabled.
	for i, factory := range reconcilers {
		reconciler, err := factory(c.KubeClient, c.TeleportClient)
		if err != nil {
			return nil, trace.Wrap(err, "creating reconciler[%d]", i)
		}
		log := c.Log.WithValues(
			"group", reconciler.GVK().Group,
			"version", reconciler.GVK().Version,
			"kind", reconciler.GVK().Kind,
			"teleport_kind", reconciler.TeleportKind())
		crdDeployed, err := gvks.lookupGVK(reconciler.GVK())
		if err != nil {
			return nil, trace.Wrap(err, "looking up GVK for reconciler[%d]", i)
		}
		if c.Scoped && !reconciler.Scoped() {
			if crdDeployed {
				log.Info("CRD deployed but operator running in scoped mode, CR will not be reconciled")
			}
			continue
		}
		if !crdDeployed {
			log.Info("CRD not deployed in the cluster, reconciler skipped")
			continue
		}
		if !reconciler.CheckFeatures(c.Features) {
			log.Info("Teleport cluster doesn't advertise the required features, reconciler skipped")
			continue
		}
		enabledReconcilers = append(enabledReconcilers, reconciler)
	}
	return enabledReconcilers, nil
}
