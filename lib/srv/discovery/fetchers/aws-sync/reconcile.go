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
	return accessgraphv1alpha.AWSResourceList_builder{
		Resources: make([]*accessgraphv1alpha.AWSResource, 0),
	}.Build()
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
		reconcile(old.KMSKeys, new.KMSKeys, kmsKeyKey, kmsKeyWrap),
	}
	for _, res := range reconciledResources {
		upsert.SetResources(append(upsert.GetResources(), res.upsert.GetResources()...))
		delete.SetResources(append(delete.GetResources(), res.delete.GetResources()...))
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
			deleteRes.SetResources(append(deleteRes.GetResources(), wrapFn(item)))
		}
		return &reconcilePair{upsertRes, deleteRes}
	}

	// Create all new items if there are no old items
	if len(oldItems) == 0 {
		for _, item := range newItems {
			upsertRes.SetResources(append(upsertRes.GetResources(), wrapFn(item)))
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
			upsertRes.SetResources(append(upsertRes.GetResources(), wrapFn(item)))
		}
	}

	// Append removed items to the delete list
	for _, item := range oldItems {
		if _, ok := newMap[keyFn(item)]; !ok {
			deleteRes.SetResources(append(deleteRes.GetResources(), wrapFn(item)))
		}
	}
	return &reconcilePair{upsertRes, deleteRes}
}

func instanceKey(instance *accessgraphv1alpha.AWSInstanceV1) string {
	return fmt.Sprintf("%s;%s", instance.GetInstanceId(), instance.GetRegion())
}

func instanceWrap(instance *accessgraphv1alpha.AWSInstanceV1) *accessgraphv1alpha.AWSResource {
	return accessgraphv1alpha.AWSResource_builder{Instance: proto.ValueOrDefault(instance)}.Build()
}

func usersKey(user *accessgraphv1alpha.AWSUserV1) string {
	return fmt.Sprintf("%s;%s", user.GetAccountId(), user.GetArn())
}

func usersWrap(user *accessgraphv1alpha.AWSUserV1) *accessgraphv1alpha.AWSResource {
	return accessgraphv1alpha.AWSResource_builder{User: proto.ValueOrDefault(user)}.Build()
}

func userInlinePolKey(policy *accessgraphv1alpha.AWSUserInlinePolicyV1) string {
	return fmt.Sprintf("%s;%s;%s", policy.GetAccountId(), policy.GetUser().GetUserName(), policy.GetPolicyName())
}

func userInlinePolWrap(policy *accessgraphv1alpha.AWSUserInlinePolicyV1) *accessgraphv1alpha.AWSResource {
	return accessgraphv1alpha.AWSResource_builder{UserInlinePolicy: proto.ValueOrDefault(policy)}.Build()
}

func userAttchPolKey(policy *accessgraphv1alpha.AWSUserAttachedPolicies) string {
	return fmt.Sprintf("%s;%s", policy.GetAccountId(), policy.GetUser().GetArn())
}

func userAttchPolWrap(policy *accessgraphv1alpha.AWSUserAttachedPolicies) *accessgraphv1alpha.AWSResource {
	return accessgraphv1alpha.AWSResource_builder{UserAttachedPolicies: proto.ValueOrDefault(policy)}.Build()
}

func userGroupKey(group *accessgraphv1alpha.AWSUserGroupsV1) string {
	return fmt.Sprintf("%s;%s", group.GetUser().GetAccountId(), group.GetUser().GetArn())
}

func userGroupWrap(group *accessgraphv1alpha.AWSUserGroupsV1) *accessgraphv1alpha.AWSResource {
	return accessgraphv1alpha.AWSResource_builder{UserGroups: proto.ValueOrDefault(group)}.Build()
}

func groupKey(group *accessgraphv1alpha.AWSGroupV1) string {
	return fmt.Sprintf("%s;%s", group.GetAccountId(), group.GetArn())
}

func groupWrap(group *accessgraphv1alpha.AWSGroupV1) *accessgraphv1alpha.AWSResource {
	return accessgraphv1alpha.AWSResource_builder{Group: proto.ValueOrDefault(group)}.Build()
}

func grpInlinePolKey(policy *accessgraphv1alpha.AWSGroupInlinePolicyV1) string {
	return fmt.Sprintf("%s;%s;%s", policy.GetGroup().GetName(), policy.GetPolicyName(), policy.GetAccountId())
}

func grpInlinePolWrap(policy *accessgraphv1alpha.AWSGroupInlinePolicyV1) *accessgraphv1alpha.AWSResource {
	return accessgraphv1alpha.AWSResource_builder{GroupInlinePolicy: proto.ValueOrDefault(policy)}.Build()
}

func grpAttchPolKey(policy *accessgraphv1alpha.AWSGroupAttachedPolicies) string {
	return fmt.Sprintf("%s;%s", policy.GetGroup().GetAccountId(), policy.GetGroup().GetArn())
}

func grpAttchPolWrap(policy *accessgraphv1alpha.AWSGroupAttachedPolicies) *accessgraphv1alpha.AWSResource {
	return accessgraphv1alpha.AWSResource_builder{GroupAttachedPolicies: proto.ValueOrDefault(policy)}.Build()
}

func policyKey(policy *accessgraphv1alpha.AWSPolicyV1) string {
	return fmt.Sprintf("%s;%s", policy.GetAccountId(), policy.GetArn())
}

func policyWrap(policy *accessgraphv1alpha.AWSPolicyV1) *accessgraphv1alpha.AWSResource {
	return accessgraphv1alpha.AWSResource_builder{Policy: proto.ValueOrDefault(policy)}.Build()
}

func s3bucketKey(s3 *accessgraphv1alpha.AWSS3BucketV1) string {
	return fmt.Sprintf("%s;%s", s3.GetAccountId(), s3.GetName())
}

func s3bucketWrap(s3 *accessgraphv1alpha.AWSS3BucketV1) *accessgraphv1alpha.AWSResource {
	return accessgraphv1alpha.AWSResource_builder{S3Bucket: proto.ValueOrDefault(s3)}.Build()
}

func roleKey(role *accessgraphv1alpha.AWSRoleV1) string {
	return fmt.Sprintf("%s;%s", role.GetAccountId(), role.GetArn())
}

func roleWrap(role *accessgraphv1alpha.AWSRoleV1) *accessgraphv1alpha.AWSResource {
	return accessgraphv1alpha.AWSResource_builder{Role: proto.ValueOrDefault(role)}.Build()
}

func roleInlinePolKey(policy *accessgraphv1alpha.AWSRoleInlinePolicyV1) string {
	return fmt.Sprintf("%s;%s;%s", policy.GetAccountId(), policy.GetAwsRole().GetArn(), policy.GetPolicyName())
}

func roleInlinePolWrap(policy *accessgraphv1alpha.AWSRoleInlinePolicyV1) *accessgraphv1alpha.AWSResource {
	return accessgraphv1alpha.AWSResource_builder{RoleInlinePolicy: proto.ValueOrDefault(policy)}.Build()
}

func roleAttchPolKey(policy *accessgraphv1alpha.AWSRoleAttachedPolicies) string {
	return fmt.Sprintf("%s;%s", policy.GetAwsRole().GetArn(), policy.GetAccountId())
}

func roleAttchPolWrap(policy *accessgraphv1alpha.AWSRoleAttachedPolicies) *accessgraphv1alpha.AWSResource {
	return accessgraphv1alpha.AWSResource_builder{RoleAttachedPolicies: proto.ValueOrDefault(policy)}.Build()
}

func instanceProfKey(profile *accessgraphv1alpha.AWSInstanceProfileV1) string {
	return fmt.Sprintf("%s;%s", profile.GetAccountId(), profile.GetInstanceProfileId())
}

func instanceProfWrap(profile *accessgraphv1alpha.AWSInstanceProfileV1) *accessgraphv1alpha.AWSResource {
	return accessgraphv1alpha.AWSResource_builder{InstanceProfile: proto.ValueOrDefault(profile)}.Build()
}

func eksClusterKey(cluster *accessgraphv1alpha.AWSEKSClusterV1) string {
	return fmt.Sprintf("%s;%s", cluster.GetAccountId(), cluster.GetArn())
}

func eksClusterWrap(cluster *accessgraphv1alpha.AWSEKSClusterV1) *accessgraphv1alpha.AWSResource {
	return accessgraphv1alpha.AWSResource_builder{EksCluster: proto.ValueOrDefault(cluster)}.Build()
}

func assocAccPolKey(policy *accessgraphv1alpha.AWSEKSAssociatedAccessPolicyV1) string {
	return fmt.Sprintf("%s;%s;%s;%s", policy.GetAccountId(), policy.GetCluster().GetArn(), policy.GetPrincipalArn(), policy.GetPolicyArn())
}

func assocAccPolWrap(policy *accessgraphv1alpha.AWSEKSAssociatedAccessPolicyV1) *accessgraphv1alpha.AWSResource {
	return accessgraphv1alpha.AWSResource_builder{EksClusterAssociatedPolicy: proto.ValueOrDefault(policy)}.Build()
}

func accessEntryKey(entry *accessgraphv1alpha.AWSEKSClusterAccessEntryV1) string {
	return fmt.Sprintf("%s;%s;%s;%s", entry.GetAccountId(), entry.GetCluster().GetArn(), entry.GetPrincipalArn(), entry.GetAccessEntryArn())
}

func accessEntryWrap(entry *accessgraphv1alpha.AWSEKSClusterAccessEntryV1) *accessgraphv1alpha.AWSResource {
	return accessgraphv1alpha.AWSResource_builder{EksClusterAccessEntry: proto.ValueOrDefault(entry)}.Build()
}

func rdsDbKey(db *accessgraphv1alpha.AWSRDSDatabaseV1) string {
	return fmt.Sprintf("%s;%s", db.GetAccountId(), db.GetArn())
}

func rdsDbWrap(db *accessgraphv1alpha.AWSRDSDatabaseV1) *accessgraphv1alpha.AWSResource {
	return accessgraphv1alpha.AWSResource_builder{Rds: proto.ValueOrDefault(db)}.Build()
}

func samlProvKey(provider *accessgraphv1alpha.AWSSAMLProviderV1) string {
	return fmt.Sprintf("%s;%s", provider.GetAccountId(), provider.GetArn())
}

func samlProvWrap(provider *accessgraphv1alpha.AWSSAMLProviderV1) *accessgraphv1alpha.AWSResource {
	return accessgraphv1alpha.AWSResource_builder{SamlProvider: proto.ValueOrDefault(provider)}.Build()
}

func oidcProvKey(provider *accessgraphv1alpha.AWSOIDCProviderV1) string {
	return fmt.Sprintf("%s;%s", provider.GetAccountId(), provider.GetArn())
}

func oidcProvWrap(provider *accessgraphv1alpha.AWSOIDCProviderV1) *accessgraphv1alpha.AWSResource {
	return accessgraphv1alpha.AWSResource_builder{OidcProvider: proto.ValueOrDefault(provider)}.Build()
}

func kmsKeyKey(key *accessgraphv1alpha.AWSKMSKeyV1) string {
	return fmt.Sprintf("%s;%s", key.GetAccountId(), key.GetArn())
}

func kmsKeyWrap(key *accessgraphv1alpha.AWSKMSKeyV1) *accessgraphv1alpha.AWSResource {
	return accessgraphv1alpha.AWSResource_builder{KmsKey: proto.ValueOrDefault(key)}.Build()
}
