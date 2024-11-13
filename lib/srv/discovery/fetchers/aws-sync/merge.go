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

package aws_sync

import "github.com/gravitational/teleport/lib/srv/discovery/common"

// MergeResources merges multiple resources into a single Resources object.
// This is used to merge resources from multiple accounts and regions
// into a single object.
// It does not check for duplicates, so it is possible to have duplicates.
func MergeResources(results ...*Resources) *Resources {
	if len(results) == 0 {
		return &Resources{}
	}
	if len(results) == 1 {
		return results[0]
	}
	result := &Resources{}
	for _, r := range results {
		result.Users = append(result.Users, r.Users...)
		result.UserInlinePolicies = append(result.UserInlinePolicies, r.UserInlinePolicies...)
		result.UserAttachedPolicies = append(result.UserAttachedPolicies, r.UserAttachedPolicies...)
		result.UserGroups = append(result.UserGroups, r.UserGroups...)
		result.Groups = append(result.Groups, r.Groups...)
		result.GroupInlinePolicies = append(result.GroupInlinePolicies, r.GroupInlinePolicies...)
		result.GroupAttachedPolicies = append(result.GroupAttachedPolicies, r.GroupAttachedPolicies...)
		result.Instances = append(result.Instances, r.Instances...)
		result.Policies = append(result.Policies, r.Policies...)
		result.S3Buckets = append(result.S3Buckets, r.S3Buckets...)
		result.Roles = append(result.Roles, r.Roles...)
		result.RoleInlinePolicies = append(result.RoleInlinePolicies, r.RoleInlinePolicies...)
		result.RoleAttachedPolicies = append(result.RoleAttachedPolicies, r.RoleAttachedPolicies...)
		result.InstanceProfiles = append(result.InstanceProfiles, r.InstanceProfiles...)
		result.AssociatedAccessPolicies = append(result.AssociatedAccessPolicies, r.AssociatedAccessPolicies...)
		result.EKSClusters = append(result.EKSClusters, r.EKSClusters...)
		result.AccessEntries = append(result.AccessEntries, r.AccessEntries...)
		result.RDSDatabases = append(result.RDSDatabases, r.RDSDatabases...)
		result.SAMLProviders = append(result.SAMLProviders, r.SAMLProviders...)
		result.OIDCProviders = append(result.OIDCProviders, r.OIDCProviders...)
	}

	deduplicateResources(result)
	return result
}

func deduplicateResources(result *Resources) {
	result.Users = common.DeduplicateSlice(result.Users, usersKey)
	result.UserInlinePolicies = common.DeduplicateSlice(result.UserInlinePolicies, userInlinePolKey)
	result.UserAttachedPolicies = common.DeduplicateSlice(result.UserAttachedPolicies, userAttchPolKey)
	result.UserGroups = common.DeduplicateSlice(result.UserGroups, userGroupKey)
	result.Groups = common.DeduplicateSlice(result.Groups, groupKey)
	result.GroupInlinePolicies = common.DeduplicateSlice(result.GroupInlinePolicies, grpInlinePolKey)
	result.GroupAttachedPolicies = common.DeduplicateSlice(result.GroupAttachedPolicies, grpAttchPolKey)
	result.Instances = common.DeduplicateSlice(result.Instances, instanceKey)
	result.Policies = common.DeduplicateSlice(result.Policies, policyKey)
	result.S3Buckets = common.DeduplicateSlice(result.S3Buckets, s3bucketKey)
	result.Roles = common.DeduplicateSlice(result.Roles, roleKey)
	result.RoleInlinePolicies = common.DeduplicateSlice(result.RoleInlinePolicies, roleInlinePolKey)
	result.RoleAttachedPolicies = common.DeduplicateSlice(result.RoleAttachedPolicies, roleAttchPolKey)
	result.InstanceProfiles = common.DeduplicateSlice(result.InstanceProfiles, instanceProfKey)
	result.AssociatedAccessPolicies = common.DeduplicateSlice(result.AssociatedAccessPolicies, assocAccPolKey)
	result.EKSClusters = common.DeduplicateSlice(result.EKSClusters, eksClusterKey)
	result.AccessEntries = common.DeduplicateSlice(result.AccessEntries, accessEntryKey)
	result.RDSDatabases = common.DeduplicateSlice(result.RDSDatabases, rdsDbKey)
	result.SAMLProviders = common.DeduplicateSlice(result.SAMLProviders, samlProvKey)
	result.OIDCProviders = common.DeduplicateSlice(result.OIDCProviders, oidcProvKey)
}
