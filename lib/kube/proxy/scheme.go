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
	"errors"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/gravitational/teleport/lib/utils"
)

const (
	// listSuffix is the suffix added to the name of the type to create the name
	// of the list type.
	// For example: "Role" -> "RoleList"
	listSuffix = "List"
)

var (
	// globalKubeScheme is the runtime Scheme that holds information about supported
	// message types.
	globalKubeScheme = runtime.NewScheme()
	// globalKubeCodecs creates a serializer/deserizalier for the different codecs
	// supported by the Kubernetes API.
	globalKubeCodecs = serializer.NewCodecFactory(globalKubeScheme)
)

// Register all groups in the schema's registry.
// It manually registers support for `metav1.Table` because go-client does not
// support it but `kubectl` calls require support for it.
func init() {
	// Register external types for Scheme
	utilruntime.Must(registerDefaultKubeTypes(globalKubeScheme))
}

// registerDefaultKubeTypes registers the default types for the Kubernetes API into
// the given scheme.
func registerDefaultKubeTypes(s *runtime.Scheme) error {
	// Register external types for Scheme
	metav1.AddToGroupVersion(s, schema.GroupVersion{Version: "v1"})
	if err := metav1.AddMetaToScheme(s); err != nil {
		return trace.Wrap(err)
	}
	if err := metav1beta1.AddMetaToScheme(s); err != nil {
		return trace.Wrap(err)
	}
	if err := scheme.AddToScheme(s); err != nil {
		return trace.Wrap(err)
	}
	err := s.SetVersionPriority(corev1.SchemeGroupVersion)
	return trace.Wrap(err)
}

// newClientNegotiator creates a negotiator that based on `Content-Type` header
// from the Kubernetes API response is able to create a different encoder/decoder.
// Supported content types:
// - "application/json"
// - "application/yaml"
// - "application/vnd.kubernetes.protobuf"
func newClientNegotiator(codecFactory *serializer.CodecFactory) runtime.ClientNegotiator {
	return runtime.NewClientNegotiator(
		codecFactory.WithoutConversion(),
		schema.GroupVersion{
			// create a serializer for Kube API v1
			Version: "v1",
		},
	)
}

// gvkSupportedResourcesKey is the key used in gvkSupportedResources
// to map from a parsed API path to the corresponding resource GVK.
type gvkSupportedResourcesKey struct {
	name     string
	apiGroup string
	version  string
}

// gvkSupportedResources maps a parsed API path to the corresponding resource GVK.
type gvkSupportedResources map[gvkSupportedResourcesKey]*schema.GroupVersionKind

// newClusterSchemaBuilder creates a new schema builder for the given cluster.
// This schema includes all well-known Kubernetes types and all namespaced
// custom resources.
// It also returns a map of resources that we support RBAC restrictions for.
func newClusterSchemaBuilder(log *slog.Logger, client kubernetes.Interface) (*serializer.CodecFactory, rbacSupportedResources, gvkSupportedResources, error) {
	kubeScheme := runtime.NewScheme()
	kubeCodecs := serializer.NewCodecFactory(kubeScheme)
	supportedResources := maps.Clone(defaultRBACResources)
	gvkSupportedRes := make(gvkSupportedResources)
	if err := registerDefaultKubeTypes(kubeScheme); err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	// discoveryErr is returned when the discovery of one or more API groups fails.
	var discoveryErr *discovery.ErrGroupDiscoveryFailed
	// register all namespaced custom resources
	_, apiGroups, err := client.Discovery().ServerGroupsAndResources()
	switch {
	case errors.As(err, &discoveryErr):
		// If the discovery of one or more API groups fails, we still want to
		// register the well-known Kubernetes types.
		// This is because the discovery of API groups can fail if the APIService
		// is not available. Usually, this happens when the API service is not local
		// to the cluster (e.g. when API is served by a pod) and the service is not
		// reachable.
		// In this case, we still want to register the other resources that are
		// available in the cluster.
		log.DebugContext(context.Background(), "Failed to discover some API groups",
			"groups", slices.Collect(maps.Keys(discoveryErr.Groups)),
			"error", err,
		)
	case err != nil:
		return nil, nil, nil, trace.Wrap(err)
	}

	for _, apiGroup := range apiGroups {
		group, version := getKubeAPIGroupAndVersion(apiGroup.GroupVersion)

		for _, apiResource := range apiGroup.APIResources {
			// register all types
			gvkSupportedRes[gvkSupportedResourcesKey{
				name:     apiResource.Name, /* pods, configmaps, ... */
				apiGroup: group,
				version:  version,
			}] = &schema.GroupVersionKind{
				Group:   group,
				Version: version,
				Kind:    apiResource.Kind, /* Pod, ConfigMap ...*/
			}
		}

		// Skip well-known Kubernetes API groups because they are already registered
		// in the scheme.
		if _, ok := knownKubernetesGroups[group]; ok {
			continue
		}
		groupVersion := schema.GroupVersion{Group: group, Version: version}
		for _, apiResource := range apiGroup.APIResources {
			// build the resource key to be able to look it up later and check if
			// if we support RBAC restrictions for it.
			resourceKey := allowedResourcesKey{
				apiGroup:     group,
				resourceKind: apiResource.Name,
			}
			// Namespaced custom resources are allowed if the user has access to
			// the namespace where the resource is located.
			// This means that we need to map the resource to the namespace kind.
			supportedResources[resourceKey] = utils.KubeCustomResource
			// create the group version kind for the resource
			gvk := groupVersion.WithKind(apiResource.Kind)
			// check if the resource is already registered in the scheme
			// if it is, we don't need to register it again.
			if _, err := kubeScheme.New(gvk); err == nil {
				continue
			}
			// register the resource with the scheme to be able to decode it
			// into an unstructured object
			kubeScheme.AddKnownTypeWithName(
				gvk,
				&unstructured.Unstructured{},
			)
			// register the resource list with the scheme to be able to decode it
			// into an unstructured object.
			// Resource lists follow the naming convention: <resource-kind>List
			kubeScheme.AddKnownTypeWithName(
				groupVersion.WithKind(apiResource.Kind+listSuffix),
				&unstructured.Unstructured{},
			)
		}
	}

	return &kubeCodecs, supportedResources, gvkSupportedRes, nil
}

// getKubeAPIGroupAndVersion returns the API group and version from the given
// groupVersion string.
// The groupVersion string can be in the following formats:
// - "v1" -> group: "", version: "v1"
// - "<group>/<version>" -> group: "<group>", version: "<version>"
func getKubeAPIGroupAndVersion(groupVersion string) (group string, version string) {
	splits := strings.Split(groupVersion, "/")
	switch {
	case len(splits) == 1:
		return "", splits[0]
	case len(splits) >= 2:
		return splits[0], splits[1]
	default:
		return "", ""
	}
}

// knownKubernetesGroups is a map of well-known Kubernetes API groups that
// are already registered in the scheme and we don't need to register them
// again.
var knownKubernetesGroups = map[string]struct{}{
	// core group
	"":                             {},
	"apiregistration.k8s.io":       {},
	"apps":                         {},
	"events.k8s.io":                {},
	"authentication.k8s.io":        {},
	"authorization.k8s.io":         {},
	"autoscaling":                  {},
	"batch":                        {},
	"certificates.k8s.io":          {},
	"networking.k8s.io":            {},
	"policy":                       {},
	"rbac.authorization.k8s.io":    {},
	"storage.k8s.io":               {},
	"admissionregistration.k8s.io": {},
	"apiextensions.k8s.io":         {},
	"scheduling.k8s.io":            {},
	"coordination.k8s.io":          {},
	"node.k8s.io":                  {},
	"discovery.k8s.io":             {},
	"flowcontrol.apiserver.k8s.io": {},
	"metrics.k8s.io":               {},
}
