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

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

func newResourceList() *accessgraphv1alpha.AWSResourceList {
	return &accessgraphv1alpha.AWSResourceList{
		Resources: make([]*accessgraphv1alpha.AWSResource, 0),
	}
}

// ReconcileResults reconciles two Resources objects and returns the operations
// required to reconcile them into the new state.
// It returns two AWSResourceList objects, one for resources to upsert and one
// for resources to delete.
func ReconcileResults(old *Resources, new *Resources) (upsert, delete *accessgraphv1alpha.AWSResourceList) {
	upsert, delete = newResourceList(), newResourceList()
	reconciledResources := []*reconcilePair{
		reconcile(old.Users, new.Users, usersKey, usersWrap),
		reconcile(old.UserInlinePolicies, new.UserInlinePolicies, userInlinePolKey, userInlinePolWrap),
		reconcile(old.UserAttachedPolicies, new.UserAttachedPolicies, userAttchPolKey, userAttchPolWrap),
		reconcile(old.UserGroups, new.UserGroups, userGroupKey, userGroupWrap),
		reconcile(old.Groups, new.Groups, groupKey, groupWrap),
		reconcile(old.GroupInlinePolicies, new.GroupInlinePolicies, grpInlinePolKey, grpInlinePolWrap),
		reconcile(old.GroupAttachedPolicies, new.GroupAttachedPolicies, grpAttchPolKey, grpAttchPolWrap),
		reconcile(old.Policies, new.Policies, policyKey, policyWrap),
		reconcile(old.Instances, new.Instances, instanceKey, instanceWrap),
		reconcile(old.S3Buckets, new.S3Buckets, s3bucketKey, s3bucketWrap),
		reconcile(old.Roles, new.Roles, roleKey, roleWrap),
		reconcile(old.RoleInlinePolicies, new.RoleInlinePolicies, roleInlinePolKey, roleInlinePolWrap),
		reconcile(old.RoleAttachedPolicies, new.RoleAttachedPolicies, roleAttchPolKey, roleAttchPolWrap),
		reconcile(old.InstanceProfiles, new.InstanceProfiles, instanceProfKey, instanceProfWrap),
		reconcile(old.EKSClusters, new.EKSClusters, eksClusterKey, eksClusterWrap),
		reconcile(old.AssociatedAccessPolicies, new.AssociatedAccessPolicies, assocAccPolKey, assocAccPolWrap),
		reconcile(old.AccessEntries, new.AccessEntries, accessEntryKey, accessEntryWrap),
		reconcile(old.RDSDatabases, new.RDSDatabases, rdsDbKey, rdsDbWrap),
		reconcile(old.SAMLProviders, new.SAMLProviders, samlProvKey, samlProvWrap),
		reconcile(old.OIDCProviders, new.OIDCProviders, oidcProvKey, oidcProvWrap),
	}
	for _, res := range reconciledResources {
		upsert.Resources = append(upsert.Resources, res.upsert.Resources...)
		delete.Resources = append(delete.Resources, res.delete.Resources...)
	}
	return upsert, delete
}

type reconcilePair struct {
	upsert, delete *accessgraphv1alpha.AWSResourceList
}

func deduplicateSlice[T any](s []T, key func(T) string) []T {
	out := make([]T, 0, len(s))
	seen := make(map[string]struct{})
	for _, v := range s {
		if _, ok := seen[key(v)]; ok {
			continue
		}
		seen[key(v)] = struct{}{}
		out = append(out, v)
	}
	return out
}

func reconcile[T proto.Message](
	oldItems []T,
	newItems []T,
	keyFn func(T) string,
	wrapFn func(T) *accessgraphv1alpha.AWSResource,
) *reconcilePair {
	// Remove duplicates from the new items
	newItems = deduplicateSlice(newItems, keyFn)
	upsertRes := newResourceList()
	deleteRes := newResourceList()

	// Delete all old items if there are no new items
	if len(newItems) == 0 {
		for _, item := range oldItems {
			deleteRes.Resources = append(deleteRes.Resources, wrapFn(item))
		}
		return &reconcilePair{upsertRes, deleteRes}
	}

	// Create all new items if there are no old items
	if len(oldItems) == 0 {
		for _, item := range newItems {
			upsertRes.Resources = append(upsertRes.Resources, wrapFn(item))
		}
		return &reconcilePair{upsertRes, deleteRes}
	}

	// Map old and new items by their key
	oldMap := make(map[string]T, len(oldItems))
	for _, item := range oldItems {
		oldMap[keyFn(item)] = item
	}
	newMap := make(map[string]T, len(newItems))
	for _, item := range newItems {
		newMap[keyFn(item)] = item
	}

	// Append new or modified items to the upsert list
	for _, item := range newItems {
		if oldItem, ok := oldMap[keyFn(item)]; !ok || !proto.Equal(oldItem, item) {
			upsertRes.Resources = append(upsertRes.Resources, wrapFn(item))
		}
	}

	// Append removed items to the delete list
	for _, item := range oldItems {
		if _, ok := newMap[keyFn(item)]; !ok {
			deleteRes.Resources = append(deleteRes.Resources, wrapFn(item))
		}
	}
	return &reconcilePair{upsertRes, deleteRes}
}

func instanceKey(instance *accessgraphv1alpha.AWSInstanceV1) string {
	return fmt.Sprintf("%s;%s", instance.InstanceId, instance.Region)
}

func instanceWrap(instance *accessgraphv1alpha.AWSInstanceV1) *accessgraphv1alpha.AWSResource {
	return &accessgraphv1alpha.AWSResource{Resource: &accessgraphv1alpha.AWSResource_Instance{Instance: instance}}
}

func usersKey(user *accessgraphv1alpha.AWSUserV1) string {
	return fmt.Sprintf("%s;%s", user.AccountId, user.Arn)
}

func usersWrap(user *accessgraphv1alpha.AWSUserV1) *accessgraphv1alpha.AWSResource {
	return &accessgraphv1alpha.AWSResource{Resource: &accessgraphv1alpha.AWSResource_User{User: user}}
}

func userInlinePolKey(policy *accessgraphv1alpha.AWSUserInlinePolicyV1) string {
	return fmt.Sprintf("%s;%s;%s", policy.AccountId, policy.GetUser().GetUserName(), policy.PolicyName)
}

func userInlinePolWrap(policy *accessgraphv1alpha.AWSUserInlinePolicyV1) *accessgraphv1alpha.AWSResource {
	return &accessgraphv1alpha.AWSResource{Resource: &accessgraphv1alpha.AWSResource_UserInlinePolicy{UserInlinePolicy: policy}}
}

func userAttchPolKey(policy *accessgraphv1alpha.AWSUserAttachedPolicies) string {
	return fmt.Sprintf("%s;%s", policy.AccountId, policy.User.Arn)
}

func userAttchPolWrap(policy *accessgraphv1alpha.AWSUserAttachedPolicies) *accessgraphv1alpha.AWSResource {
	return &accessgraphv1alpha.AWSResource{Resource: &accessgraphv1alpha.AWSResource_UserAttachedPolicies{UserAttachedPolicies: policy}}
}

func userGroupKey(group *accessgraphv1alpha.AWSUserGroupsV1) string {
	return fmt.Sprintf("%s;%s", group.User.AccountId, group.User.Arn)
}

func userGroupWrap(group *accessgraphv1alpha.AWSUserGroupsV1) *accessgraphv1alpha.AWSResource {
	return &accessgraphv1alpha.AWSResource{Resource: &accessgraphv1alpha.AWSResource_UserGroups{UserGroups: group}}
}

func groupKey(group *accessgraphv1alpha.AWSGroupV1) string {
	return fmt.Sprintf("%s;%s", group.AccountId, group.Arn)
}

func groupWrap(group *accessgraphv1alpha.AWSGroupV1) *accessgraphv1alpha.AWSResource {
	return &accessgraphv1alpha.AWSResource{Resource: &accessgraphv1alpha.AWSResource_Group{Group: group}}
}

func grpInlinePolKey(policy *accessgraphv1alpha.AWSGroupInlinePolicyV1) string {
	return fmt.Sprintf("%s;%s;%s", policy.Group.Name, policy.PolicyName, policy.AccountId)
}

func grpInlinePolWrap(policy *accessgraphv1alpha.AWSGroupInlinePolicyV1) *accessgraphv1alpha.AWSResource {
	return &accessgraphv1alpha.AWSResource{Resource: &accessgraphv1alpha.AWSResource_GroupInlinePolicy{GroupInlinePolicy: policy}}
}

func grpAttchPolKey(policy *accessgraphv1alpha.AWSGroupAttachedPolicies) string {
	return fmt.Sprintf("%s;%s", policy.Group.GetAccountId(), policy.Group.Arn)
}

func grpAttchPolWrap(policy *accessgraphv1alpha.AWSGroupAttachedPolicies) *accessgraphv1alpha.AWSResource {
	return &accessgraphv1alpha.AWSResource{Resource: &accessgraphv1alpha.AWSResource_GroupAttachedPolicies{GroupAttachedPolicies: policy}}
}

func policyKey(policy *accessgraphv1alpha.AWSPolicyV1) string {
	return fmt.Sprintf("%s;%s", policy.AccountId, policy.Arn)
}

func policyWrap(policy *accessgraphv1alpha.AWSPolicyV1) *accessgraphv1alpha.AWSResource {
	return &accessgraphv1alpha.AWSResource{Resource: &accessgraphv1alpha.AWSResource_Policy{Policy: policy}}
}

func s3bucketKey(s3 *accessgraphv1alpha.AWSS3BucketV1) string {
	return fmt.Sprintf("%s;%s", s3.AccountId, s3.Name)
}

func s3bucketWrap(s3 *accessgraphv1alpha.AWSS3BucketV1) *accessgraphv1alpha.AWSResource {
	return &accessgraphv1alpha.AWSResource{Resource: &accessgraphv1alpha.AWSResource_S3Bucket{S3Bucket: s3}}
}

func roleKey(role *accessgraphv1alpha.AWSRoleV1) string {
	return fmt.Sprintf("%s;%s", role.AccountId, role.Arn)
}

func roleWrap(role *accessgraphv1alpha.AWSRoleV1) *accessgraphv1alpha.AWSResource {
	return &accessgraphv1alpha.AWSResource{Resource: &accessgraphv1alpha.AWSResource_Role{Role: role}}
}

func roleInlinePolKey(policy *accessgraphv1alpha.AWSRoleInlinePolicyV1) string {
	return fmt.Sprintf("%s;%s;%s", policy.AccountId, policy.GetAwsRole().Arn, policy.PolicyName)
}

func roleInlinePolWrap(policy *accessgraphv1alpha.AWSRoleInlinePolicyV1) *accessgraphv1alpha.AWSResource {
	return &accessgraphv1alpha.AWSResource{Resource: &accessgraphv1alpha.AWSResource_RoleInlinePolicy{RoleInlinePolicy: policy}}
}

func roleAttchPolKey(policy *accessgraphv1alpha.AWSRoleAttachedPolicies) string {
	return fmt.Sprintf("%s;%s", policy.GetAwsRole().GetArn(), policy.AccountId)
}

func roleAttchPolWrap(policy *accessgraphv1alpha.AWSRoleAttachedPolicies) *accessgraphv1alpha.AWSResource {
	return &accessgraphv1alpha.AWSResource{Resource: &accessgraphv1alpha.AWSResource_RoleAttachedPolicies{RoleAttachedPolicies: policy}}
}

func instanceProfKey(profile *accessgraphv1alpha.AWSInstanceProfileV1) string {
	return fmt.Sprintf("%s;%s", profile.AccountId, profile.InstanceProfileId)
}

func instanceProfWrap(profile *accessgraphv1alpha.AWSInstanceProfileV1) *accessgraphv1alpha.AWSResource {
	return &accessgraphv1alpha.AWSResource{Resource: &accessgraphv1alpha.AWSResource_InstanceProfile{InstanceProfile: profile}}
}

func eksClusterKey(cluster *accessgraphv1alpha.AWSEKSClusterV1) string {
	return fmt.Sprintf("%s;%s", cluster.AccountId, cluster.Arn)
}

func eksClusterWrap(cluster *accessgraphv1alpha.AWSEKSClusterV1) *accessgraphv1alpha.AWSResource {
	return &accessgraphv1alpha.AWSResource{Resource: &accessgraphv1alpha.AWSResource_EksCluster{EksCluster: cluster}}
}

func assocAccPolKey(policy *accessgraphv1alpha.AWSEKSAssociatedAccessPolicyV1) string {
	return fmt.Sprintf("%s;%s;%s;%s", policy.AccountId, policy.Cluster.Arn, policy.PrincipalArn, policy.PolicyArn)
}

func assocAccPolWrap(policy *accessgraphv1alpha.AWSEKSAssociatedAccessPolicyV1) *accessgraphv1alpha.AWSResource {
	return &accessgraphv1alpha.AWSResource{Resource: &accessgraphv1alpha.AWSResource_EksClusterAssociatedPolicy{EksClusterAssociatedPolicy: policy}}
}

func accessEntryKey(entry *accessgraphv1alpha.AWSEKSClusterAccessEntryV1) string {
	return fmt.Sprintf("%s;%s;%s;%s", entry.AccountId, entry.Cluster.Arn, entry.PrincipalArn, entry.AccessEntryArn)
}

func accessEntryWrap(entry *accessgraphv1alpha.AWSEKSClusterAccessEntryV1) *accessgraphv1alpha.AWSResource {
	return &accessgraphv1alpha.AWSResource{Resource: &accessgraphv1alpha.AWSResource_EksClusterAccessEntry{EksClusterAccessEntry: entry}}
}

func rdsDbKey(db *accessgraphv1alpha.AWSRDSDatabaseV1) string {
	return fmt.Sprintf("%s;%s", db.AccountId, db.Arn)
}

func rdsDbWrap(db *accessgraphv1alpha.AWSRDSDatabaseV1) *accessgraphv1alpha.AWSResource {
	return &accessgraphv1alpha.AWSResource{Resource: &accessgraphv1alpha.AWSResource_Rds{Rds: db}}
}

func samlProvKey(provider *accessgraphv1alpha.AWSSAMLProviderV1) string {
	return fmt.Sprintf("%s;%s", provider.AccountId, provider.Arn)
}

func samlProvWrap(provider *accessgraphv1alpha.AWSSAMLProviderV1) *accessgraphv1alpha.AWSResource {
	return &accessgraphv1alpha.AWSResource{Resource: &accessgraphv1alpha.AWSResource_SamlProvider{SamlProvider: provider}}
}

func oidcProvKey(provider *accessgraphv1alpha.AWSOIDCProviderV1) string {
	return fmt.Sprintf("%s;%s", provider.AccountId, provider.Arn)
}

func oidcProvWrap(provider *accessgraphv1alpha.AWSOIDCProviderV1) *accessgraphv1alpha.AWSResource {
	return &accessgraphv1alpha.AWSResource{Resource: &accessgraphv1alpha.AWSResource_OidcProvider{OidcProvider: provider}}
}
