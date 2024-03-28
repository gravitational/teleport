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

package services

import (
	"slices"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	azureutils "github.com/gravitational/teleport/api/utils/azure"
)

// ResourceMatcher matches cluster resources.
type ResourceMatcher struct {
	// Labels match resource labels.
	Labels types.Labels
	// AWS contains AWS specific settings.
	AWS ResourceMatcherAWS
}

// ResourceMatcherAWS contains AWS specific settings.
type ResourceMatcherAWS struct {
	// AssumeRoleARN is the AWS role to assume for accessing the resource.
	AssumeRoleARN string
	// ExternalID is an optional AWS external ID used to enable assuming an AWS
	// role across accounts.
	ExternalID string
}

// ResourceMatchersToTypes converts []]services.ResourceMatchers into []*types.ResourceMatcher
func ResourceMatchersToTypes(in []ResourceMatcher) []*types.DatabaseResourceMatcher {
	out := make([]*types.DatabaseResourceMatcher, len(in))
	for i, resMatcher := range in {
		resMatcher := resMatcher
		out[i] = &types.DatabaseResourceMatcher{
			Labels: &resMatcher.Labels,
			AWS: types.ResourceMatcherAWS{
				AssumeRoleARN: resMatcher.AWS.AssumeRoleARN,
				ExternalID:    resMatcher.AWS.ExternalID,
			},
		}
	}
	return out
}

// AssumeRoleFromAWSMetadata is a conversion helper function that extracts
// AWS IAM role ARN and external ID from AWS metadata.
func AssumeRoleFromAWSMetadata(meta *types.AWS) types.AssumeRole {
	return types.AssumeRole{
		RoleARN:    meta.AssumeRoleARN,
		ExternalID: meta.ExternalID,
	}
}

// SimplifyAzureMatchers returns simplified Azure Matchers.
// Selectors are deduplicated, wildcard in a selector reduces the selector
// to just the wildcard, and defaults are applied.
func SimplifyAzureMatchers(matchers []types.AzureMatcher) []types.AzureMatcher {
	result := make([]types.AzureMatcher, 0, len(matchers))
	for _, m := range matchers {
		subs := apiutils.Deduplicate(m.Subscriptions)
		groups := apiutils.Deduplicate(m.ResourceGroups)
		regions := apiutils.Deduplicate(m.Regions)
		ts := apiutils.Deduplicate(m.Types)
		if len(subs) == 0 || slices.Contains(subs, types.Wildcard) {
			subs = []string{types.Wildcard}
		}
		if len(groups) == 0 || slices.Contains(groups, types.Wildcard) {
			groups = []string{types.Wildcard}
		}
		if len(regions) == 0 || slices.Contains(regions, types.Wildcard) {
			regions = []string{types.Wildcard}
		} else {
			for i, region := range regions {
				regions[i] = azureutils.NormalizeLocation(region)
			}
		}
		result = append(result, types.AzureMatcher{
			Subscriptions:  subs,
			ResourceGroups: groups,
			Regions:        regions,
			Types:          ts,
			ResourceTags:   m.ResourceTags,
			Params:         m.Params,
		})
	}
	return result
}

// MatchResourceLabels returns true if any of the provided selectors matches the provided database.
func MatchResourceLabels(matchers []ResourceMatcher, labels map[string]string) bool {
	for _, matcher := range matchers {
		if len(matcher.Labels) == 0 {
			return false
		}
		match, _, err := MatchLabels(matcher.Labels, labels)
		if err != nil {
			logrus.WithError(err).Errorf("Failed to match labels %v: %v.",
				matcher.Labels, labels)
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
type ResourceSeenKey struct{ name, kind, addr string }

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
	kind := resource.GetKind()

	// We assume when filtering for services like KubeService, AppServer, and DatabaseServer
	// the user is wanting to filter the contained resource ie. KubeClusters, Application, and Database.
	key := ResourceSeenKey{
		kind: kind,
		name: resource.GetName(),
	}
	switch kind {
	case types.KindNode,
		types.KindDatabaseService,
		types.KindKubernetesCluster,
		types.KindWindowsDesktop, types.KindWindowsDesktopService,
		types.KindUserGroup:
		specResource = resource
	case types.KindKubeServer:
		if seenMap != nil {
			return false, trace.BadParameter("checking for duplicate matches for resource kind %q is not supported", filter.ResourceKind)
		}
		return matchAndFilterKubeClusters(resource, filter)

	case types.KindDatabaseServer:
		server, ok := resource.(types.DatabaseServer)
		if !ok {
			return false, trace.BadParameter("expected types.DatabaseServer, got %T", resource)
		}
		specResource = server.GetDatabase()
		key.name = specResource.GetName()
	case types.KindAppServer, types.KindSAMLIdPServiceProvider, types.KindAppOrSAMLIdPServiceProvider:
		switch appOrSP := resource.(type) {
		case types.AppServer:
			app := appOrSP.GetApp()
			specResource = app
			key.addr = app.GetPublicAddr()
			key.name = app.GetName()
		case types.SAMLIdPServiceProvider:
			specResource = appOrSP
			key.name = specResource.GetName()
		default:
			return false, trace.BadParameter("expected types.SAMLIdPServiceProvider or types.AppServer, got %T", resource)
		}
	default:
		// We check if the resource kind is a Kubernetes resource kind to reduce the amount of
		// of cases we need to handle. If the resource type didn't match any arm before
		// and it is not a Kubernetes resource kind, we return an error.
		if !slices.Contains(types.KubernetesResourcesKinds, filter.ResourceKind) {
			return false, trace.NotImplemented("filtering for resource kind %q not supported", kind)
		}
		specResource = resource
	}

	var match bool

	if filter.IsSimple() {
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
		if _, exists := seenMap[key]; exists {
			return false, nil
		}
		seenMap[key] = struct{}{}
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

	if !types.MatchKinds(resource, filter.Kinds) {
		return false, nil
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
//  1. handles kube service having a 1-N relationship (service-clusters)
//     so each kube cluster goes through the filters
//  2. filters out the non-matched clusters on the kube service and the kube service is
//     modified in place with only the matched clusters
//  3. only returns true if the service contained any matched cluster
func matchAndFilterKubeClusters(resource types.ResourceWithLabels, filter MatchResourceFilter) (bool, error) {
	if filter.IsSimple() {
		return true, nil
	}

	switch server := resource.(type) {
	case types.KubeServer:
		kubeCluster := server.GetCluster()
		if kubeCluster == nil {
			return false, nil
		}
		match, err := matchResourceByFilters(kubeCluster, filter)
		return match, trace.Wrap(err)
	default:
		return false, trace.BadParameter("unexpected kube server of type %T", resource)
	}
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
	// Kinds is a list of resourceKinds to be used when doing a unified resource query.
	// It will filter out any kind not present in the list. If the list is not present or empty
	// then all kinds are valid and will be returned (still subject to other included filters)
	Kinds []string
}

// IsSimple is used to short-circuit matching when a filter doesn't specify anything more
// specific than resource kind.
func (m *MatchResourceFilter) IsSimple() bool {
	return len(m.Labels) == 0 &&
		len(m.SearchKeywords) == 0 &&
		m.PredicateExpression == "" &&
		len(m.Kinds) == 0
}
