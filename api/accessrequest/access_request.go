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

package accessrequest

import (
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

type ListResourcesRequestOption func(*proto.ListResourcesRequest)

// GetResourcesByKind is an alternative to client.GetResourcesWithFilters
// that searches with the resource kinds used in access requests instead of the
// resource types expected by ListResources.
//
// The ResourceType field in the request should not be set by the caller, as
// it will be overridden.
func GetResourcesByKind(ctx context.Context, clt client.ListResourcesClient, req proto.ListResourcesRequest, kind string) ([]types.ResourceWithLabels, error) {
	req.ResourceType = mapResourceKindToListResourcesType(kind)
	results, err := client.GetResourcesWithFilters(ctx, clt, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resources := make([]types.ResourceWithLabels, 0, len(results))
	for _, result := range results {
		leafResource, err := mapListResourcesResultToLeafResource(result, kind)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, leafResource)
	}
	return resources, nil
}

// GetResourceDetails gets extra details for a list of resources in a given cluster.
func GetResourceDetails(ctx context.Context, clusterName string, lister client.ListResourcesClient, ids []types.ResourceID) (map[string]types.ResourceDetails, error) {
	var resourceIDs []types.ResourceID
	for _, resourceID := range ids {
		// We're interested in hostname or friendly name details. These apply to
		// nodes, app servers, user groups and Identity Center resources.
		switch resourceID.Kind {
		case types.KindNode, types.KindApp, types.KindUserGroup, types.KindIdentityCenterAccount:
			resourceIDs = append(resourceIDs, resourceID)
		}
	}

	withExtraRoles := func(req *proto.ListResourcesRequest) {
		req.UseSearchAsRoles = true
		req.UsePreviewAsRoles = true
	}

	resources, err := GetResourcesByResourceIDs(ctx, lister, resourceIDs, withExtraRoles)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result := make(map[string]types.ResourceDetails)
	for _, resource := range resources {
		friendlyName := types.FriendlyName(resource)

		// No friendly name was found, so skip to the next resource.
		if friendlyName == "" {
			continue
		}

		id := types.ResourceID{
			ClusterName: clusterName,
			Kind:        resource.GetKind(),
			Name:        resource.GetName(),
		}
		result[types.ResourceIDToString(id)] = types.ResourceDetails{
			FriendlyName: friendlyName,
		}
	}

	return result, nil
}

// GetResourceIDsByCluster will return resource IDs grouped by cluster.
func GetResourceIDsByCluster(r types.AccessRequest) map[string][]types.ResourceID {
	resourceIDsByCluster := make(map[string][]types.ResourceID)
	for _, resourceID := range r.GetRequestedResourceIDs() {
		resourceIDsByCluster[resourceID.ClusterName] = append(resourceIDsByCluster[resourceID.ClusterName], resourceID)
	}
	return resourceIDsByCluster
}

// GetResourcesByResourceID gets a list of resources by their resource IDs.
func GetResourcesByResourceIDs(ctx context.Context, lister client.ListResourcesClient, resourceIDs []types.ResourceID, opts ...ListResourcesRequestOption) ([]types.ResourceWithLabels, error) {
	resourceNamesByKind := make(map[string][]string)
	for _, resourceID := range resourceIDs {
		resourceNamesByKind[resourceID.Kind] = append(resourceNamesByKind[resourceID.Kind], resourceID.Name)
	}
	var resources []types.ResourceWithLabels
	for kind, resourceNames := range resourceNamesByKind {
		req := proto.ListResourcesRequest{
			PredicateExpression: anyNameMatcher(resourceNames),
			Limit:               int32(len(resourceNames)),
		}
		for _, opt := range opts {
			opt(&req)
		}
		resp, err := GetResourcesByKind(ctx, lister, req, kind)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, resp...)
	}
	return resources, nil
}

// anyNameMatcher returns a PredicateExpression which matches any of a given list
// of names. Given names will be escaped and quoted when building the expression.
func anyNameMatcher(names []string) string {
	matchers := make([]string, len(names))
	for i := range names {
		matchers[i] = fmt.Sprintf(`resource.metadata.name == %q`, names[i])
	}
	return strings.Join(matchers, " || ")
}

// mapResourceKindToListResourcesType returns the value to use for ResourceType in a
// ListResourcesRequest based on the kind of resource you're searching for.
// Necessary because some resource kinds don't support ListResources directly,
// so you have to list the parent kind. Use MapListResourcesResultToLeafResource to map back
// to the given kind.
func mapResourceKindToListResourcesType(kind string) string {
	switch kind {
	case types.KindApp:
		return types.KindAppServer
	case types.KindDatabase:
		return types.KindDatabaseServer
	case types.KindKubernetesCluster:
		return types.KindKubeServer
	default:
		return kind
	}
}

// mapListResourcesResultToLeafResource is the inverse of
// MapResourceKindToListResourcesType, after the ListResources call it maps the
// result back to the kind we really want. `hint` should be the name of the
// desired resource kind, used to disambiguate normal SSH nodes and kubernetes
// services which are both returned as `types.Server`.
func mapListResourcesResultToLeafResource(resource types.ResourceWithLabels, hint string) (types.ResourceWithLabels, error) {
	switch r := resource.(type) {
	case types.AppServer:
		return r.GetApp(), nil
	case types.KubeServer:
		return r.GetCluster(), nil
	case types.DatabaseServer:
		return r.GetDatabase(), nil
	case types.Server:
		if hint == types.KindKubernetesCluster {
			return nil, trace.BadParameter("expected kubernetes server, got server")
		}
	default:
	}
	return resource, nil
}
