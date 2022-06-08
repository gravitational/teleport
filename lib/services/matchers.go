/*
Copyright 2021 Gravitational, Inc.

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

package services

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
)

// ResourceMatcher matches cluster resources.
type ResourceMatcher struct {
	// Labels match resource labels.
	Labels types.Labels
}

// AWSMatcher matches AWS databases.
type AWSMatcher struct {
	// Types are AWS database types to match, "rds" or "redshift".
	Types []string
	// Regions are AWS regions to query for databases.
	Regions []string
	// Tags are AWS tags to match.
	Tags types.Labels
}

// MatchResourceLabels returns true if any of the provided selectors matches the provided database.
func MatchResourceLabels(matchers []ResourceMatcher, resource types.ResourceWithLabels) bool {
	for _, matcher := range matchers {
		if len(matcher.Labels) == 0 {
			return false
		}
		match, _, err := MatchLabels(matcher.Labels, resource.GetAllLabels())
		if err != nil {
			logrus.WithError(err).Errorf("Failed to match labels %v: %v.",
				matcher.Labels, resource)
			return false
		}
		if match {
			return true
		}
	}
	return false
}

// ResourceSeenKey is used as a key for a map that keeps track
// of unique resource names and address. Currently "addr"
// only applies to resource Application.
type ResourceSeenKey struct{ name, addr string }

// MatchResourceByFilters returns true if all filter values given matched against the resource.
//
// If no filters were provided, we will treat that as a match.
//
// If a `seenMap` is provided, this will be treated as a request to filter out duplicate matches.
// The map will be modified in place as it adds new keys. Seen keys will return match as false.
//
// Resource KubeService is handled differently b/c of its 1-N relationhip with service-clusters,
// it filters out the non-matched clusters on the kube service and the kube service
// is modified in place with only the matched clusters. Deduplication for resource `KubeService`
// is not provided but is provided for kind `KubernetesCluster`.
func MatchResourceByFilters(resource types.ResourceWithLabels, filter MatchResourceFilter, seenMap map[ResourceSeenKey]struct{}) (bool, error) {
	var specResource types.ResourceWithLabels

	// We assume when filtering for services like KubeService, AppServer, and DatabaseServer
	// the user is wanting to filter the contained resource ie. KubeClusters, Application, and Database.
	resourceKey := ResourceSeenKey{}
	switch filter.ResourceKind {
	case types.KindNode, types.KindWindowsDesktop, types.KindKubernetesCluster:
		specResource = resource
		resourceKey.name = specResource.GetName()

	case types.KindKubeService:
		if seenMap != nil {
			return false, trace.BadParameter("checking for duplicate matches for resource kind %q is not supported", filter.ResourceKind)
		}
		return matchAndFilterKubeClusters(resource, filter)

	case types.KindAppServer:
		server, ok := resource.(types.AppServer)
		if !ok {
			return false, trace.BadParameter("expected types.AppServer, got %T", resource)
		}
		specResource = server.GetApp()
		app := server.GetApp()
		resourceKey.name = app.GetName()
		resourceKey.addr = app.GetPublicAddr()

	case types.KindDatabaseServer:
		server, ok := resource.(types.DatabaseServer)
		if !ok {
			return false, trace.BadParameter("expected types.DatabaseServer, got %T", resource)
		}
		specResource = server.GetDatabase()
		resourceKey.name = specResource.GetName()

	default:
		return false, trace.NotImplemented("filtering for resource kind %q not supported", filter.ResourceKind)
	}

	var match bool

	if len(filter.Labels) == 0 && len(filter.SearchKeywords) == 0 && filter.PredicateExpression == "" {
		match = true
	}

	if !match {
		var err error
		match, err = matchResourceByFilters(specResource, filter)
		if err != nil {
			return false, trace.Wrap(err)
		}
	}

	// Deduplicate matches.
	if match && seenMap != nil {
		if _, exists := seenMap[resourceKey]; exists {
			return false, nil
		}
		seenMap[resourceKey] = struct{}{}
	}

	return match, nil
}

func matchResourceByFilters(resource types.ResourceWithLabels, filter MatchResourceFilter) (bool, error) {
	if filter.PredicateExpression != "" {
		parser, err := NewResourceParser(resource)
		if err != nil {
			return false, trace.Wrap(err)
		}

		switch match, err := parser.EvalBoolPredicate(filter.PredicateExpression); {
		case err != nil:
			return false, trace.BadParameter("failed to parse predicate expression: %s", err.Error())
		case !match:
			return false, nil
		}
	}

	if !types.MatchLabels(resource, filter.Labels) {
		return false, nil
	}

	if !resource.MatchSearch(filter.SearchKeywords) {
		return false, nil
	}

	return true, nil
}

// matchAndFilterKubeClusters is similar to MatchResourceByFilters, but does two things in addition:
//  1) handles kube service having a 1-N relationship (service-clusters)
//     so each kube cluster goes through the filters
//  2) filters out the non-matched clusters on the kube service and the kube service is
//     modified in place with only the matched clusters
//  3) only returns true if the service contained any matched cluster
func matchAndFilterKubeClusters(resource types.ResourceWithLabels, filter MatchResourceFilter) (bool, error) {
	if len(filter.Labels) == 0 && len(filter.SearchKeywords) == 0 && filter.PredicateExpression == "" {
		return true, nil
	}

	server, ok := resource.(types.Server)
	if !ok {
		return false, trace.BadParameter("expected types.Server, got %T", resource)
	}

	kubeClusters := server.GetKubernetesClusters()

	// Apply filter to each kube cluster.
	filtered := make([]*types.KubernetesCluster, 0, len(kubeClusters))
	for _, kube := range kubeClusters {
		kubeResource, err := types.NewKubernetesClusterV3FromLegacyCluster(server.GetNamespace(), kube)
		if err != nil {
			return false, trace.Wrap(err)
		}

		match, err := matchResourceByFilters(kubeResource, filter)
		if err != nil {
			return false, trace.Wrap(err)
		}
		if match {
			filtered = append(filtered, kube)
		}
	}

	// Update in place with the filtered clusters.
	server.SetKubernetesClusters(filtered)

	// Length of 0 means this service does not contain any matches.
	return len(filtered) > 0, nil
}

// MatchResourceFilter holds the filter values to match against a resource.
type MatchResourceFilter struct {
	// ResourceKind is the resource kind and is used to fine tune the filtering.
	ResourceKind string
	// Labels are the labels to match.
	Labels map[string]string
	// SearchKeywords is a list of search keywords to match.
	SearchKeywords []string
	// PredicateExpression holds boolean conditions that must be matched.
	PredicateExpression string
}

const (
	// AWSMatcherRDS is the AWS matcher type for RDS databases.
	AWSMatcherRDS = "rds"
	// AWSMatcherRedshift is the AWS matcher type for Redshift databases.
	AWSMatcherRedshift = "redshift"
	// AWSMatcherElastiCache is the AWS matcher type for ElastiCache databases.
	AWSMatcherElastiCache = "elasticache"
)
