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

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

// ReconcileResults reconciles two Resources objects and returns the operations
// required to reconcile them into the new state.
// It returns two AWSResourceList objects, one for resources to upsert and one
// for resources to delete.
func ReconcileResults(old *Resources, new *Resources) (upsert, delete *accessgraphv1alpha.AWSResourceList) {
	upsert, delete = &accessgraphv1alpha.AWSResourceList{}, &accessgraphv1alpha.AWSResourceList{}

	for _, results := range []*reconcileIntermeditateResult{
		reconcileUsers(old.Users, new.Users),
		reconcileUserInlinePolicies(old.UserInlinePolicies, new.UserInlinePolicies),
		reconcileUserAttachedPolicies(old.UserAttachedPolicies, new.UserAttachedPolicies),
		reconcileUserGroups(old.UserGroups, new.UserGroups),
		reconcileGroups(old.Groups, new.Groups),
		reconcileGroupInlinePolicies(old.GroupInlinePolicies, new.GroupInlinePolicies),
		reconcileGroupAttachedPolicies(old.GroupAttachedPolicies, new.GroupAttachedPolicies),
		reconcilePolicies(old.Policies, new.Policies),
		reconcileInstances(old.Instances, new.Instances),
		reconcileS3(old.S3Buckets, new.S3Buckets),
		reconcileRoles(old.Roles, new.Roles),
		reconcileRoleInlinePolicies(old.RoleInlinePolicies, new.RoleInlinePolicies),
		reconcileRoleAttachedPolicies(old.RoleAttachedPolicies, new.RoleAttachedPolicies),
		reconcileInstanceProfiles(old.InstanceProfiles, new.InstanceProfiles),
		reconcileEKSClusters(old.EKSClusters, new.EKSClusters),
		reconcileAssociatedAccessPolicy(old.AssociatedAccessPolicies, new.AssociatedAccessPolicies),
		reconcileAccessEntry(old.AccessEntries, new.AccessEntries),
		reconcileAWSRDS(old.RDSDatabases, new.RDSDatabases),
	} {
		upsert.Resources = append(upsert.Resources, results.upsert.Resources...)
		delete.Resources = append(delete.Resources, results.delete.Resources...)
	}

	return upsert, delete
}

type reconcileIntermeditateResult struct {
	upsert, delete *accessgraphv1alpha.AWSResourceList
}

func reconcileInstances(old []*accessgraphv1alpha.AWSInstanceV1, new []*accessgraphv1alpha.AWSInstanceV1) *reconcileIntermeditateResult {
	upsert, delete := &accessgraphv1alpha.AWSResourceList{}, &accessgraphv1alpha.AWSResourceList{}

	toAdd, toRemove := reconcile(old, new, func(instance *accessgraphv1alpha.AWSInstanceV1) string {
		return fmt.Sprintf("%s;%s", instance.InstanceId, instance.Region)
	})

	for _, instance := range toAdd {
		upsert.Resources = append(upsert.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_Instance{
				Instance: instance,
			},
		})
	}
	for _, instance := range toRemove {
		delete.Resources = append(delete.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_Instance{
				Instance: instance,
			},
		})
	}
	return &reconcileIntermeditateResult{upsert, delete}
}

func reconcileUsers(
	old []*accessgraphv1alpha.AWSUserV1,
	new []*accessgraphv1alpha.AWSUserV1,
) *reconcileIntermeditateResult {
	upsert, delete := &accessgraphv1alpha.AWSResourceList{}, &accessgraphv1alpha.AWSResourceList{}

	toAdd, toRemove := reconcile(old, new, func(user *accessgraphv1alpha.AWSUserV1) string {
		return fmt.Sprintf("%s;%s", user.AccountId, user.Arn)
	})
	for _, user := range toAdd {
		upsert.Resources = append(upsert.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_User{
				User: user,
			},
		})
	}
	for _, user := range toRemove {
		delete.Resources = append(delete.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_User{
				User: user,
			},
		})
	}
	return &reconcileIntermeditateResult{upsert, delete}
}

func reconcileUserInlinePolicies(
	old []*accessgraphv1alpha.AWSUserInlinePolicyV1,
	new []*accessgraphv1alpha.AWSUserInlinePolicyV1,
) *reconcileIntermeditateResult {
	upsert, delete := &accessgraphv1alpha.AWSResourceList{}, &accessgraphv1alpha.AWSResourceList{}

	toAdd, toRemove := reconcile(old, new, func(policy *accessgraphv1alpha.AWSUserInlinePolicyV1) string {
		return fmt.Sprintf("%s;%s;%s", policy.AccountId, policy.GetUser().GetUserName(), policy.PolicyName)
	})
	for _, policy := range toAdd {
		upsert.Resources = append(upsert.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_UserInlinePolicy{
				UserInlinePolicy: policy,
			},
		})
	}
	for _, policy := range toRemove {
		delete.Resources = append(delete.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_UserInlinePolicy{
				UserInlinePolicy: policy,
			},
		})
	}
	return &reconcileIntermeditateResult{upsert, delete}
}

func reconcileUserAttachedPolicies(
	old []*accessgraphv1alpha.AWSUserAttachedPolicies,
	new []*accessgraphv1alpha.AWSUserAttachedPolicies,
) *reconcileIntermeditateResult {
	upsert, delete := &accessgraphv1alpha.AWSResourceList{}, &accessgraphv1alpha.AWSResourceList{}

	toAdd, toRemove := reconcile(old, new, func(policy *accessgraphv1alpha.AWSUserAttachedPolicies) string {
		return fmt.Sprintf("%s;%s", policy.AccountId, policy.User.Arn)
	})
	for _, policy := range toAdd {
		upsert.Resources = append(upsert.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_UserAttachedPolicies{
				UserAttachedPolicies: policy,
			},
		})
	}
	for _, policy := range toRemove {
		delete.Resources = append(delete.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_UserAttachedPolicies{
				UserAttachedPolicies: policy,
			},
		})
	}
	return &reconcileIntermeditateResult{upsert, delete}
}

func reconcileUserGroups(
	old []*accessgraphv1alpha.AWSUserGroupsV1,
	new []*accessgraphv1alpha.AWSUserGroupsV1,
) *reconcileIntermeditateResult {
	upsert, delete := &accessgraphv1alpha.AWSResourceList{}, &accessgraphv1alpha.AWSResourceList{}

	toAdd, toRemove := reconcile(old, new, func(group *accessgraphv1alpha.AWSUserGroupsV1) string {
		return fmt.Sprintf("%s;%s", group.User.AccountId, group.User.Arn)
	})
	for _, group := range toAdd {
		upsert.Resources = append(upsert.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_UserGroups{
				UserGroups: group,
			},
		})
	}
	for _, group := range toRemove {
		delete.Resources = append(delete.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_UserGroups{
				UserGroups: group,
			},
		})
	}
	return &reconcileIntermeditateResult{upsert, delete}
}

func reconcileGroups(
	old []*accessgraphv1alpha.AWSGroupV1,
	new []*accessgraphv1alpha.AWSGroupV1,
) *reconcileIntermeditateResult {
	upsert, delete := &accessgraphv1alpha.AWSResourceList{}, &accessgraphv1alpha.AWSResourceList{}

	toAdd, toRemove := reconcile(old, new, func(group *accessgraphv1alpha.AWSGroupV1) string {
		return fmt.Sprintf("%s;%s", group.AccountId, group.Arn)
	})
	for _, group := range toAdd {
		upsert.Resources = append(upsert.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_Group{
				Group: group,
			},
		})
	}
	for _, group := range toRemove {
		delete.Resources = append(delete.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_Group{
				Group: group,
			},
		})
	}
	return &reconcileIntermeditateResult{upsert, delete}
}

func reconcileGroupInlinePolicies(
	old []*accessgraphv1alpha.AWSGroupInlinePolicyV1,
	new []*accessgraphv1alpha.AWSGroupInlinePolicyV1,
) *reconcileIntermeditateResult {
	upsert, delete := &accessgraphv1alpha.AWSResourceList{}, &accessgraphv1alpha.AWSResourceList{}

	toAdd, toRemove := reconcile(old, new, func(policy *accessgraphv1alpha.AWSGroupInlinePolicyV1) string {
		return fmt.Sprintf("%s;%s;%s", policy.Group.Name, policy.PolicyName, policy.AccountId)
	})
	for _, policy := range toAdd {
		upsert.Resources = append(upsert.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_GroupInlinePolicy{
				GroupInlinePolicy: policy,
			},
		})
	}
	for _, policy := range toRemove {
		delete.Resources = append(delete.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_GroupInlinePolicy{
				GroupInlinePolicy: policy,
			},
		})
	}
	return &reconcileIntermeditateResult{upsert, delete}
}

func reconcileGroupAttachedPolicies(
	old []*accessgraphv1alpha.AWSGroupAttachedPolicies,
	new []*accessgraphv1alpha.AWSGroupAttachedPolicies,
) *reconcileIntermeditateResult {
	upsert, delete := &accessgraphv1alpha.AWSResourceList{}, &accessgraphv1alpha.AWSResourceList{}

	toAdd, toRemove := reconcile(old, new, func(policy *accessgraphv1alpha.AWSGroupAttachedPolicies) string {
		return fmt.Sprintf("%s;%s", policy.Group.GetAccountId(), policy.Group.Arn)
	})
	for _, policy := range toAdd {
		upsert.Resources = append(upsert.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_GroupAttachedPolicies{
				GroupAttachedPolicies: policy,
			},
		})
	}
	for _, policy := range toRemove {
		delete.Resources = append(delete.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_GroupAttachedPolicies{
				GroupAttachedPolicies: policy,
			},
		})
	}
	return &reconcileIntermeditateResult{upsert, delete}
}

func reconcilePolicies(
	old []*accessgraphv1alpha.AWSPolicyV1,
	new []*accessgraphv1alpha.AWSPolicyV1,
) *reconcileIntermeditateResult {
	upsert, delete := &accessgraphv1alpha.AWSResourceList{}, &accessgraphv1alpha.AWSResourceList{}
	toAdd, toRemove := reconcile(old, new, func(policy *accessgraphv1alpha.AWSPolicyV1) string {
		return fmt.Sprintf("%s;%s", policy.AccountId, policy.Arn)
	})
	for _, policy := range toAdd {
		upsert.Resources = append(upsert.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_Policy{
				Policy: policy,
			},
		})
	}
	for _, policy := range toRemove {
		delete.Resources = append(delete.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_Policy{
				Policy: policy,
			},
		})
	}
	return &reconcileIntermeditateResult{upsert, delete}
}

func reconcile[T protoreflect.ProtoMessage](old []T, new []T, key func(T) string) (upsert, delete []T) {
	if len(old) == 0 {
		return new, nil
	}
	if len(new) == 0 {
		return nil, old
	}

	oldMap := make(map[string]T, len(old))
	for _, item := range old {
		oldMap[key(item)] = item
	}

	newMap := make(map[string]T, len(new))
	for _, item := range new {
		newMap[key(item)] = item
	}

	for _, item := range new {
		if oldItem, ok := oldMap[key(item)]; !ok || !proto.Equal(oldItem, item) {
			upsert = append(upsert, item)
		}
	}
	for _, item := range old {
		if _, ok := newMap[key(item)]; !ok {
			delete = append(delete, item)
		}
	}
	return upsert, delete
}

func reconcileS3(old []*accessgraphv1alpha.AWSS3BucketV1, new []*accessgraphv1alpha.AWSS3BucketV1) *reconcileIntermeditateResult {
	upsert, delete := &accessgraphv1alpha.AWSResourceList{}, &accessgraphv1alpha.AWSResourceList{}

	toAdd, toRemove := reconcile(old, new, func(s3 *accessgraphv1alpha.AWSS3BucketV1) string {
		return fmt.Sprintf("%s;%s", s3.AccountId, s3.Name)
	})

	for _, s3 := range toAdd {
		upsert.Resources = append(upsert.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_S3Bucket{
				S3Bucket: s3,
			},
		})
	}
	for _, s3 := range toRemove {
		delete.Resources = append(delete.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_S3Bucket{
				S3Bucket: s3,
			},
		})
	}
	return &reconcileIntermeditateResult{upsert, delete}
}

func reconcileRoles(old []*accessgraphv1alpha.AWSRoleV1, new []*accessgraphv1alpha.AWSRoleV1) *reconcileIntermeditateResult {
	upsert, delete := &accessgraphv1alpha.AWSResourceList{}, &accessgraphv1alpha.AWSResourceList{}
	toAdd, toRemove := reconcile(old, new, func(role *accessgraphv1alpha.AWSRoleV1) string {
		return fmt.Sprintf("%s;%s", role.AccountId, role.Arn)
	})

	for _, role := range toAdd {
		upsert.Resources = append(upsert.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_Role{
				Role: role,
			},
		})
	}
	for _, role := range toRemove {
		delete.Resources = append(delete.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_Role{
				Role: role,
			},
		})
	}
	return &reconcileIntermeditateResult{upsert, delete}
}

func reconcileRoleInlinePolicies(
	old []*accessgraphv1alpha.AWSRoleInlinePolicyV1,
	new []*accessgraphv1alpha.AWSRoleInlinePolicyV1,
) *reconcileIntermeditateResult {
	upsert, delete := &accessgraphv1alpha.AWSResourceList{}, &accessgraphv1alpha.AWSResourceList{}
	toAdd, toRemove := reconcile(old, new, func(policy *accessgraphv1alpha.AWSRoleInlinePolicyV1) string {
		return fmt.Sprintf("%s;%s;%s", policy.AccountId, policy.GetAwsRole().Arn, policy.PolicyName)
	})

	for _, policy := range toAdd {
		upsert.Resources = append(upsert.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_RoleInlinePolicy{
				RoleInlinePolicy: policy,
			},
		})
	}
	for _, policy := range toRemove {
		delete.Resources = append(delete.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_RoleInlinePolicy{
				RoleInlinePolicy: policy,
			},
		})
	}
	return &reconcileIntermeditateResult{upsert, delete}
}

func reconcileRoleAttachedPolicies(
	old []*accessgraphv1alpha.AWSRoleAttachedPolicies,
	new []*accessgraphv1alpha.AWSRoleAttachedPolicies,
) *reconcileIntermeditateResult {
	upsert, delete := &accessgraphv1alpha.AWSResourceList{}, &accessgraphv1alpha.AWSResourceList{}
	toAdd, toRemove := reconcile(old, new, func(policy *accessgraphv1alpha.AWSRoleAttachedPolicies) string {
		return fmt.Sprintf("%s;%s", policy.GetAwsRole().GetArn(), policy.AccountId)
	})

	for _, policy := range toAdd {
		upsert.Resources = append(upsert.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_RoleAttachedPolicies{
				RoleAttachedPolicies: policy,
			},
		})
	}
	for _, policy := range toRemove {
		delete.Resources = append(delete.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_RoleAttachedPolicies{
				RoleAttachedPolicies: policy,
			},
		})
	}
	return &reconcileIntermeditateResult{upsert, delete}
}

func reconcileInstanceProfiles(
	old []*accessgraphv1alpha.AWSInstanceProfileV1,
	new []*accessgraphv1alpha.AWSInstanceProfileV1,
) *reconcileIntermeditateResult {
	upsert, delete := &accessgraphv1alpha.AWSResourceList{}, &accessgraphv1alpha.AWSResourceList{}
	toAdd, toRemove := reconcile(old, new, func(profile *accessgraphv1alpha.AWSInstanceProfileV1) string {
		return fmt.Sprintf("%s;%s", profile.AccountId, profile.InstanceProfileId)
	})

	for _, profile := range toAdd {
		upsert.Resources = append(upsert.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_InstanceProfile{
				InstanceProfile: profile,
			},
		})
	}
	for _, profile := range toRemove {
		delete.Resources = append(delete.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_InstanceProfile{
				InstanceProfile: profile,
			},
		})
	}
	return &reconcileIntermeditateResult{upsert, delete}
}

func reconcileEKSClusters(
	old []*accessgraphv1alpha.AWSEKSClusterV1,
	new []*accessgraphv1alpha.AWSEKSClusterV1,
) *reconcileIntermeditateResult {
	upsert, delete := &accessgraphv1alpha.AWSResourceList{}, &accessgraphv1alpha.AWSResourceList{}
	toAdd, toRemove := reconcile(old, new, func(cluster *accessgraphv1alpha.AWSEKSClusterV1) string {
		return fmt.Sprintf("%s;%s", cluster.AccountId, cluster.Arn)
	})

	for _, cluster := range toAdd {
		upsert.Resources = append(upsert.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_EksCluster{
				EksCluster: cluster,
			},
		})
	}
	for _, cluster := range toRemove {
		delete.Resources = append(delete.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_EksCluster{
				EksCluster: cluster,
			},
		})
	}
	return &reconcileIntermeditateResult{upsert, delete}
}

func reconcileAssociatedAccessPolicy(
	old []*accessgraphv1alpha.AWSEKSAssociatedAccessPolicyV1,
	new []*accessgraphv1alpha.AWSEKSAssociatedAccessPolicyV1,
) *reconcileIntermeditateResult {
	upsert, delete := &accessgraphv1alpha.AWSResourceList{}, &accessgraphv1alpha.AWSResourceList{}
	toAdd, toRemove := reconcile(old, new, func(profile *accessgraphv1alpha.AWSEKSAssociatedAccessPolicyV1) string {
		return fmt.Sprintf("%s;%s;%s;%s", profile.AccountId, profile.Cluster.Arn, profile.PrincipalArn, profile.PolicyArn)
	})

	for _, profile := range toAdd {
		upsert.Resources = append(upsert.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_EksClusterAssociatedPolicy{
				EksClusterAssociatedPolicy: profile,
			},
		})
	}
	for _, profile := range toRemove {
		delete.Resources = append(delete.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_EksClusterAssociatedPolicy{
				EksClusterAssociatedPolicy: profile,
			},
		})
	}
	return &reconcileIntermeditateResult{upsert, delete}
}

func reconcileAccessEntry(
	old []*accessgraphv1alpha.AWSEKSClusterAccessEntryV1,
	new []*accessgraphv1alpha.AWSEKSClusterAccessEntryV1,
) *reconcileIntermeditateResult {
	upsert, delete := &accessgraphv1alpha.AWSResourceList{}, &accessgraphv1alpha.AWSResourceList{}
	toAdd, toRemove := reconcile(old, new, func(profile *accessgraphv1alpha.AWSEKSClusterAccessEntryV1) string {
		return fmt.Sprintf("%s;%s;%s;%s", profile.AccountId, profile.Cluster.Arn, profile.PrincipalArn, profile.AccessEntryArn)
	})

	for _, profile := range toAdd {
		upsert.Resources = append(upsert.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_EksClusterAccessEntry{
				EksClusterAccessEntry: profile,
			},
		})
	}
	for _, profile := range toRemove {
		delete.Resources = append(delete.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_EksClusterAccessEntry{
				EksClusterAccessEntry: profile,
			},
		})
	}
	return &reconcileIntermeditateResult{upsert, delete}
}

func reconcileAWSRDS(
	old []*accessgraphv1alpha.AWSRDSDatabaseV1,
	new []*accessgraphv1alpha.AWSRDSDatabaseV1,
) *reconcileIntermeditateResult {
	upsert, delete := &accessgraphv1alpha.AWSResourceList{}, &accessgraphv1alpha.AWSResourceList{}
	toAdd, toRemove := reconcile(old, new, func(profile *accessgraphv1alpha.AWSRDSDatabaseV1) string {
		return fmt.Sprintf("%s;%s", profile.AccountId, profile.Arn)
	})

	for _, profile := range toAdd {
		upsert.Resources = append(upsert.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_Rds{
				Rds: profile,
			},
		})
	}
	for _, profile := range toRemove {
		delete.Resources = append(delete.Resources, &accessgraphv1alpha.AWSResource{
			Resource: &accessgraphv1alpha.AWSResource_Rds{
				Rds: profile,
			},
		})
	}
	return &reconcileIntermeditateResult{upsert, delete}
}
