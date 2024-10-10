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
	"google.golang.org/protobuf/reflect/protoreflect"

	tag "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

func newResourceList() *tag.AWSResourceList {
	return &tag.AWSResourceList{
		Resources: make([]*tag.AWSResource, 0),
	}
}

// ReconcileResults reconciles two Resources objects and returns the operations
// required to reconcile them into the new state.
// It returns two AWSResourceList objects, one for resources to upsert and one
// for resources to delete.
func ReconcileResults(old *Resources, new *Resources) (upsert, delete *tag.AWSResourceList) {
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
	upsert, delete *tag.AWSResourceList
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

func reconcile[T protoreflect.ProtoMessage](
	oldItems []T,
	newItems []T,
	keyFn func(T) string,
	wrapFn func(T) *tag.AWSResource,
) *reconcilePair {
	// Remove duplicates from the new items
	newItems = deduplicateSlice(newItems, keyFn)
	upsertRes := newResourceList()
	deleteRes := newResourceList()

	// Return upsert if there are no old items, and vice versa
	if len(newItems) == 0 || len(oldItems) == 0 {
		for _, item := range newItems {
			upsertRes.Resources = append(upsertRes.Resources, wrapFn(item))
		}
		for _, item := range oldItems {
			deleteRes.Resources = append(deleteRes.Resources, wrapFn(item))
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

func instanceKey(instance *tag.AWSInstanceV1) string {
	return fmt.Sprintf("%s;%s", instance.InstanceId, instance.Region)
}

func instanceWrap(instance *tag.AWSInstanceV1) *tag.AWSResource {
	return &tag.AWSResource{Resource: &tag.AWSResource_Instance{Instance: instance}}
}

func usersKey(user *tag.AWSUserV1) string {
	return fmt.Sprintf("%s;%s", user.AccountId, user.Arn)
}

func usersWrap(user *tag.AWSUserV1) *tag.AWSResource {
	return &tag.AWSResource{Resource: &tag.AWSResource_User{User: user}}
}

func userInlinePolKey(policy *tag.AWSUserInlinePolicyV1) string {
	return fmt.Sprintf("%s;%s;%s", policy.AccountId, policy.GetUser().GetUserName(), policy.PolicyName)
}

func userInlinePolWrap(policy *tag.AWSUserInlinePolicyV1) *tag.AWSResource {
	return &tag.AWSResource{Resource: &tag.AWSResource_UserInlinePolicy{UserInlinePolicy: policy}}
}

func userAttchPolKey(policy *tag.AWSUserAttachedPolicies) string {
	return fmt.Sprintf("%s;%s", policy.AccountId, policy.User.Arn)
}

func userAttchPolWrap(policy *tag.AWSUserAttachedPolicies) *tag.AWSResource {
	return &tag.AWSResource{Resource: &tag.AWSResource_UserAttachedPolicies{UserAttachedPolicies: policy}}
}

func userGroupKey(group *tag.AWSUserGroupsV1) string {
	return fmt.Sprintf("%s;%s", group.User.AccountId, group.User.Arn)
}

func userGroupWrap(group *tag.AWSUserGroupsV1) *tag.AWSResource {
	return &tag.AWSResource{Resource: &tag.AWSResource_UserGroups{UserGroups: group}}
}

func groupKey(group *tag.AWSGroupV1) string {
	return fmt.Sprintf("%s;%s", group.AccountId, group.Arn)
}

func groupWrap(group *tag.AWSGroupV1) *tag.AWSResource {
	return &tag.AWSResource{Resource: &tag.AWSResource_Group{Group: group}}
}

func grpInlinePolKey(policy *tag.AWSGroupInlinePolicyV1) string {
	return fmt.Sprintf("%s;%s;%s", policy.Group.Name, policy.PolicyName, policy.AccountId)
}

func grpInlinePolWrap(policy *tag.AWSGroupInlinePolicyV1) *tag.AWSResource {
	return &tag.AWSResource{Resource: &tag.AWSResource_GroupInlinePolicy{GroupInlinePolicy: policy}}
}

func grpAttchPolKey(policy *tag.AWSGroupAttachedPolicies) string {
	return fmt.Sprintf("%s;%s", policy.Group.GetAccountId(), policy.Group.Arn)
}

func grpAttchPolWrap(policy *tag.AWSGroupAttachedPolicies) *tag.AWSResource {
	return &tag.AWSResource{Resource: &tag.AWSResource_GroupAttachedPolicies{GroupAttachedPolicies: policy}}
}

func policyKey(policy *tag.AWSPolicyV1) string {
	return fmt.Sprintf("%s;%s", policy.AccountId, policy.Arn)
}

func policyWrap(policy *tag.AWSPolicyV1) *tag.AWSResource {
	return &tag.AWSResource{Resource: &tag.AWSResource_Policy{Policy: policy}}
}

func s3bucketKey(s3 *tag.AWSS3BucketV1) string {
	return fmt.Sprintf("%s;%s", s3.AccountId, s3.Name)
}

func s3bucketWrap(s3 *tag.AWSS3BucketV1) *tag.AWSResource {
	return &tag.AWSResource{Resource: &tag.AWSResource_S3Bucket{S3Bucket: s3}}
}

func roleKey(role *tag.AWSRoleV1) string {
	return fmt.Sprintf("%s;%s", role.AccountId, role.Arn)
}

func roleWrap(role *tag.AWSRoleV1) *tag.AWSResource {
	return &tag.AWSResource{Resource: &tag.AWSResource_Role{Role: role}}
}

func roleInlinePolKey(policy *tag.AWSRoleInlinePolicyV1) string {
	return fmt.Sprintf("%s;%s;%s", policy.AccountId, policy.GetAwsRole().Arn, policy.PolicyName)
}

func roleInlinePolWrap(policy *tag.AWSRoleInlinePolicyV1) *tag.AWSResource {
	return &tag.AWSResource{Resource: &tag.AWSResource_RoleInlinePolicy{RoleInlinePolicy: policy}}
}

func roleAttchPolKey(policy *tag.AWSRoleAttachedPolicies) string {
	return fmt.Sprintf("%s;%s", policy.GetAwsRole().GetArn(), policy.AccountId)
}

func roleAttchPolWrap(policy *tag.AWSRoleAttachedPolicies) *tag.AWSResource {
	return &tag.AWSResource{Resource: &tag.AWSResource_RoleAttachedPolicies{RoleAttachedPolicies: policy}}
}

func instanceProfKey(profile *tag.AWSInstanceProfileV1) string {
	return fmt.Sprintf("%s;%s", profile.AccountId, profile.InstanceProfileId)
}

func instanceProfWrap(profile *tag.AWSInstanceProfileV1) *tag.AWSResource {
	return &tag.AWSResource{Resource: &tag.AWSResource_InstanceProfile{InstanceProfile: profile}}
}

func eksClusterKey(cluster *tag.AWSEKSClusterV1) string {
	return fmt.Sprintf("%s;%s", cluster.AccountId, cluster.Arn)
}

func eksClusterWrap(cluster *tag.AWSEKSClusterV1) *tag.AWSResource {
	return &tag.AWSResource{Resource: &tag.AWSResource_EksCluster{EksCluster: cluster}}
}

func assocAccPolKey(policy *tag.AWSEKSAssociatedAccessPolicyV1) string {
	return fmt.Sprintf("%s;%s;%s;%s", policy.AccountId, policy.Cluster.Arn, policy.PrincipalArn, policy.PolicyArn)
}

func assocAccPolWrap(policy *tag.AWSEKSAssociatedAccessPolicyV1) *tag.AWSResource {
	return &tag.AWSResource{Resource: &tag.AWSResource_EksClusterAssociatedPolicy{EksClusterAssociatedPolicy: policy}}
}

func accessEntryKey(entry *tag.AWSEKSClusterAccessEntryV1) string {
	return fmt.Sprintf("%s;%s;%s;%s", entry.AccountId, entry.Cluster.Arn, entry.PrincipalArn, entry.AccessEntryArn)
}

func accessEntryWrap(entry *tag.AWSEKSClusterAccessEntryV1) *tag.AWSResource {
	return &tag.AWSResource{Resource: &tag.AWSResource_EksClusterAccessEntry{EksClusterAccessEntry: entry}}
}

func rdsDbKey(db *tag.AWSRDSDatabaseV1) string {
	return fmt.Sprintf("%s;%s", db.AccountId, db.Arn)
}

func rdsDbWrap(db *tag.AWSRDSDatabaseV1) *tag.AWSResource {
	return &tag.AWSResource{Resource: &tag.AWSResource_Rds{Rds: db}}
}

func samlProvKey(provider *tag.AWSSAMLProviderV1) string {
	return fmt.Sprintf("%s;%s", provider.AccountId, provider.Arn)
}

func samlProvWrap(provider *tag.AWSSAMLProviderV1) *tag.AWSResource {
	return &tag.AWSResource{Resource: &tag.AWSResource_SamlProvider{SamlProvider: provider}}
}

func oidcProvKey(provider *tag.AWSOIDCProviderV1) string {
	return fmt.Sprintf("%s;%s", provider.AccountId, provider.Arn)
}

func oidcProvWrap(provider *tag.AWSOIDCProviderV1) *tag.AWSResource {
	return &tag.AWSResource{Resource: &tag.AWSResource_OidcProvider{OidcProvider: provider}}
}
