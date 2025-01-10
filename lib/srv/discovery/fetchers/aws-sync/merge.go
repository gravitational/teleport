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
	result.Users = deduplicateSlice(result.Users, usersKey)
	result.UserInlinePolicies = deduplicateSlice(result.UserInlinePolicies, userInlinePolKey)
	result.UserAttachedPolicies = deduplicateSlice(result.UserAttachedPolicies, userAttchPolKey)
	result.UserGroups = deduplicateSlice(result.UserGroups, userGroupKey)
	result.Groups = deduplicateSlice(result.Groups, groupKey)
	result.GroupInlinePolicies = deduplicateSlice(result.GroupInlinePolicies, grpInlinePolKey)
	result.GroupAttachedPolicies = deduplicateSlice(result.GroupAttachedPolicies, grpAttchPolKey)
	result.Instances = deduplicateSlice(result.Instances, instanceKey)
	result.Policies = deduplicateSlice(result.Policies, policyKey)
	result.S3Buckets = deduplicateSlice(result.S3Buckets, s3bucketKey)
	result.Roles = deduplicateSlice(result.Roles, roleKey)
	result.RoleInlinePolicies = deduplicateSlice(result.RoleInlinePolicies, roleInlinePolKey)
	result.RoleAttachedPolicies = deduplicateSlice(result.RoleAttachedPolicies, roleAttchPolKey)
	result.InstanceProfiles = deduplicateSlice(result.InstanceProfiles, instanceProfKey)
	result.AssociatedAccessPolicies = deduplicateSlice(result.AssociatedAccessPolicies, assocAccPolKey)
	result.EKSClusters = deduplicateSlice(result.EKSClusters, eksClusterKey)
	result.AccessEntries = deduplicateSlice(result.AccessEntries, accessEntryKey)
	result.RDSDatabases = deduplicateSlice(result.RDSDatabases, rdsDbKey)
	result.SAMLProviders = deduplicateSlice(result.SAMLProviders, samlProvKey)
	result.OIDCProviders = deduplicateSlice(result.OIDCProviders, oidcProvKey)
}
