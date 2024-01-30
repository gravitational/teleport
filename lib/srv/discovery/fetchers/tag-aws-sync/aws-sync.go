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

package tag_aws_sync

import (
	"context"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/cloud"
)

var pageSize int64 = 500

type Config struct {
	CloudClients cloud.Clients
	AccountID    string
	Regions      []string
	AssumeRole   string
	ExternalID   string
}

type awsFetcher struct {
	Config
}

type AWSSync interface {
	Poll(ctx context.Context) (*PollResult, error)
	Close()
}

func NewAWSFetcher(cfg Config) AWSSync {
	return &awsFetcher{
		Config: cfg,
	}
}

type PollResult struct {
	Users                 []*accessgraphv1alpha.AWSUserV1
	UserInlinePolicies    []*accessgraphv1alpha.AWSUserInlinePolicyV1
	UserAttachedPolicies  []*accessgraphv1alpha.AWSUserAttachedPolicies
	UserGroups            []*accessgraphv1alpha.AWSUserGroupsV1
	Groups                []*accessgraphv1alpha.AWSGroupV1
	GroupInlinePolicies   []*accessgraphv1alpha.AWSGroupInlinePolicyV1
	GroupAttachedPolicies []*accessgraphv1alpha.AWSGroupAttachedPolicies
	Instances             []*accessgraphv1alpha.AWSInstanceV1
	Policies              []*accessgraphv1alpha.AWSPolicyV1
	S3Buckets             []*accessgraphv1alpha.AWSS3BucketV1
	Roles                 []*accessgraphv1alpha.AWSRoleV1
	RoleInlinePolicies    []*accessgraphv1alpha.AWSRoleInlinePolicyV1
	RoleAttachedPolicies  []*accessgraphv1alpha.AWSRoleAttachedPolicies
	InstanceProfiles      []*accessgraphv1alpha.AWSInstanceProfileV1
}

func MergePollResults(results ...*PollResult) *PollResult {
	result := &PollResult{}
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
	}
	return result
}

func (a *awsFetcher) Close() {
}

func (a *awsFetcher) Poll(ctx context.Context) (*PollResult, error) {
	result, err := a.syncIAM(ctx)
	return result, trace.Wrap(err)
}

func (a *awsFetcher) syncIAM(ctx context.Context) (*PollResult, error) {
	eGroup, ctx := errgroup.WithContext(ctx)
	eGroup.SetLimit(5)

	var (
		errs   []error
		errMu  sync.Mutex
		result = &PollResult{}
	)
	collectErr := func(err error) {
		errMu.Lock()
		defer errMu.Unlock()
		errs = append(errs, err)
	}

	eGroup.Go(func() error {
		var err error

		result.Users, err = a.fetchUsers(ctx)
		if err != nil {
			collectErr(trace.Wrap(err, "failed to fetch users"))
			return nil
		}

		eG, ctx := errgroup.WithContext(ctx)
		eG.SetLimit(10)
		usersMu := sync.Mutex{}
		for _, user := range result.Users {
			user := user
			eG.Go(func() error {
				userInlinePolicies, err := a.fetchUserInlinePolicies(ctx, user)
				if err != nil {
					collectErr(trace.Wrap(err, "failed to fetch user %q inline policies", user.UserName))
				}

				userAttachedPolicies, err := a.fetchUserAttachedPolicies(ctx, user)
				if err != nil {
					collectErr(trace.Wrap(err, "failed to fetch user %q attached policies", user.UserName))
				}

				userGroups, err := a.fetchGroupsForUser(ctx, user)
				if err != nil {
					collectErr(trace.Wrap(err, "failed to fetch user %q groups", user.UserName))
				}
				usersMu.Lock()
				result.UserInlinePolicies = append(result.UserInlinePolicies, userInlinePolicies...)
				if userAttachedPolicies != nil {
					result.UserAttachedPolicies = append(result.UserAttachedPolicies, userAttachedPolicies)
				}
				if userGroups != nil {
					result.UserGroups = append(result.UserGroups, userGroups)
				}
				usersMu.Unlock()
				return nil
			})
		}
		_ = eG.Wait()
		return nil
	})

	eGroup.Go(func() error {
		var err error

		result.Roles, err = a.fetchRoles(ctx)
		if err != nil {
			collectErr(trace.Wrap(err, "failed to fetch users"))
			return nil
		}

		eG, ctx := errgroup.WithContext(ctx)
		eG.SetLimit(10)
		roleMu := sync.Mutex{}
		for _, role := range result.Roles {
			role := role
			eG.Go(func() error {
				roleInlinePolicies, err := a.fetchRoleInlinePolicies(ctx, role)
				if err != nil {
					collectErr(trace.Wrap(err, "failed to fetch user %q inline policies", role.Name))
				}

				roleAttachedPolicies, err := a.fetchRoleAttachedPolicies(ctx, role)
				if err != nil {
					collectErr(trace.Wrap(err, "failed to fetch user %q attached policies", role.Name))
				}

				roleMu.Lock()
				result.RoleInlinePolicies = append(result.RoleInlinePolicies, roleInlinePolicies...)
				if roleAttachedPolicies != nil {
					result.RoleAttachedPolicies = append(result.RoleAttachedPolicies, roleAttachedPolicies)
				}
				roleMu.Unlock()
				return nil
			})
		}
		_ = eG.Wait()
		return nil
	})

	eGroup.Go(func() error {
		var err error
		result.Groups, err = a.fetchGroups(ctx)
		if err != nil {
			collectErr(trace.Wrap(err, "failed to fetch groups"))
			return nil
		}

		eG, ctx := errgroup.WithContext(ctx)
		eG.SetLimit(10)
		groupsMu := sync.Mutex{}
		for _, group := range result.Groups {
			group := group
			eG.Go(func() error {
				groupInlinePolicies, err := a.fetchGroupInlinePolicies(ctx, group)
				if err != nil {
					collectErr(trace.Wrap(err, "failed to fetch group %q inline policies", group.Name))
				}

				groupAttachedPolicies, err := a.fetchGroupAttachedPolicies(ctx, group)
				if err != nil {
					collectErr(trace.Wrap(err, "failed to fetch group %q attached policies", group.Name))
				}

				groupsMu.Lock()
				if groupAttachedPolicies != nil {
					result.GroupAttachedPolicies = append(result.GroupAttachedPolicies, groupAttachedPolicies)
				}
				result.GroupInlinePolicies = append(result.GroupInlinePolicies, groupInlinePolicies...)
				groupsMu.Unlock()
				return nil
			})
		}

		_ = eG.Wait()

		return nil
	})

	eGroup.Go(func() error {
		var err error
		result.Instances, err = a.fetchAWSEC2Instances(ctx)
		if err != nil {
			collectErr(trace.Wrap(err, "failed to fetch instances"))
		}
		result.InstanceProfiles, err = a.fetchInstanceProfiles(ctx)
		if err != nil {
			collectErr(trace.Wrap(err, "failed to fetch instance profiles"))
		}
		return nil
	})

	eGroup.Go(func() error {
		var err error
		result.Policies, err = a.fetchPolicies(ctx)
		if err != nil {
			collectErr(trace.Wrap(err, "failed to fetch policies"))
		}
		return nil
	})

	eGroup.Go(func() error {
		var err error
		result.S3Buckets, err = a.fetchS3Buckets(ctx)
		if err != nil {
			collectErr(trace.Wrap(err, "failed to fetch s3 buckets"))
		}
		return nil
	})

	if err := eGroup.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}
	return result, trace.NewAggregate(errs...)
}

func (a *awsFetcher) fetchAWSEC2Instances(ctx context.Context) ([]*accessgraphv1alpha.AWSInstanceV1, error) {
	var (
		hosts   []*accessgraphv1alpha.AWSInstanceV1
		hostsMu sync.Mutex
		errs    []error
	)
	eG, ctx := errgroup.WithContext(ctx)
	eG.SetLimit(10)
	collectHosts := func(lHosts []*accessgraphv1alpha.AWSInstanceV1, err error) {
		hostsMu.Lock()
		defer hostsMu.Unlock()
		if err != nil {
			errs = append(errs, err)
		}
		hosts = append(hosts, lHosts...)
	}

	for _, region := range a.Regions {
		region := region
		eG.Go(func() error {
			ec2Client, err := a.CloudClients.GetAWSEC2Client(ctx, region, a.getAWSOptions()...)
			if err != nil {
				collectHosts(nil, trace.Wrap(err))
				return nil
			}
			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			err = ec2Client.DescribeInstancesPagesWithContext(ctx, &ec2.DescribeInstancesInput{
				MaxResults: &pageSize,
			}, func(page *ec2.DescribeInstancesOutput, lastPage bool) bool {
				lHosts := make([]*accessgraphv1alpha.AWSInstanceV1, 0, len(page.Reservations))
				for _, reservation := range page.Reservations {
					for _, instance := range reservation.Instances {
						hosts = append(hosts, awsInstanceToProtoInstance(instance, a.AccountID))
					}
				}
				collectHosts(lHosts, nil)
				return !lastPage
			})

			if err != nil {
				collectHosts(hosts, trace.Wrap(err))
			}
			return nil
		})
	}

	err := eG.Wait()
	err = trace.NewAggregate(append(errs, err)...)
	return hosts, err
}

func awsInstanceToProtoInstance(instance *ec2.Instance, accountID string) *accessgraphv1alpha.AWSInstanceV1 {
	var tags []*accessgraphv1alpha.AWSTag
	for _, tag := range instance.Tags {
		tags = append(tags, &accessgraphv1alpha.AWSTag{
			Key:   aws.ToString(tag.Key),
			Value: strPtrToWrapper(tag.Value),
		})
	}

	var instanceProfileMetadata *wrapperspb.StringValue
	if instance.IamInstanceProfile != nil {
		instanceProfileMetadata = strPtrToWrapper(instance.IamInstanceProfile.Arn)
	}
	return &accessgraphv1alpha.AWSInstanceV1{
		InstanceId:            aws.ToString(instance.InstanceId),
		Region:                aws.ToString(instance.Placement.AvailabilityZone),
		PublicDnsName:         aws.ToString(instance.PublicDnsName),
		LaunchKeyName:         strPtrToWrapper(instance.KeyName),
		IamInstanceProfileArn: instanceProfileMetadata,
		AccountId:             accountID,
		Tags:                  tags,
		LaunchTime:            awsTimeToProtoTime(instance.LaunchTime),
	}
}

func (a *awsFetcher) fetchRoles(ctx context.Context) ([]*accessgraphv1alpha.AWSRoleV1, error) {
	var roles []*accessgraphv1alpha.AWSRoleV1

	iamClient, err := a.CloudClients.GetAWSIAMClient(
		ctx,
		"", /* region is empty because roles are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = iamClient.ListRolesPagesWithContext(ctx, &iam.ListRolesInput{
		MaxItems: &pageSize,
	},
		func(page *iam.ListRolesOutput, lastPage bool) bool {
			for _, role := range page.Roles {
				roles = append(roles, awsRoleToProtoRole(role, a.AccountID))
			}
			return !lastPage
		},
	)

	return roles, trace.Wrap(err)
}

func (a *awsFetcher) fetchRoleInlinePolicies(ctx context.Context, role *accessgraphv1alpha.AWSRoleV1) ([]*accessgraphv1alpha.AWSRoleInlinePolicyV1, error) {
	var policies []*accessgraphv1alpha.AWSRoleInlinePolicyV1
	var errs []error
	errCollect := func(err error) {
		errs = append(errs, err)
	}
	iamClient, err := a.CloudClients.GetAWSIAMClient(
		ctx,
		"", /* region is empty because users and groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = iamClient.ListRolePoliciesPagesWithContext(
		ctx,
		&iam.ListRolePoliciesInput{
			RoleName: aws.String(role.Name),
			MaxItems: &pageSize,
		},
		func(page *iam.ListRolePoliciesOutput, lastPage bool) bool {
			for _, policyName := range page.PolicyNames {
				policy, err := iamClient.GetRolePolicyWithContext(ctx, &iam.GetRolePolicyInput{
					RoleName:   aws.String(role.Name),
					PolicyName: policyName,
				})
				if err != nil {
					errCollect(trace.Wrap(err, "failed to fetch user %q inline policy %q", role.Name, *policyName))
					continue
				}

				policies = append(policies, awsRolePolicyToProtoUserPolicy(policy, role, a.AccountID))
			}
			return !lastPage
		})

	return policies, trace.NewAggregate(append(errs, err)...)
}

func (a *awsFetcher) fetchRoleAttachedPolicies(ctx context.Context, role *accessgraphv1alpha.AWSRoleV1) (*accessgraphv1alpha.AWSRoleAttachedPolicies, error) {
	rsp := &accessgraphv1alpha.AWSRoleAttachedPolicies{
		AwsRole:   role,
		AccountId: a.AccountID,
	}

	iamClient, err := a.CloudClients.GetAWSIAMClient(
		ctx,
		"", /* region is empty because users and groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = iamClient.ListAttachedRolePoliciesPagesWithContext(
		ctx,
		&iam.ListAttachedRolePoliciesInput{
			RoleName: aws.String(role.Name),
			MaxItems: &pageSize,
		},
		func(page *iam.ListAttachedRolePoliciesOutput, lastPage bool) bool {
			for _, policy := range page.AttachedPolicies {
				rsp.Policies = append(
					rsp.Policies,
					&accessgraphv1alpha.AttachedPolicyV1{
						Arn:        aws.ToString(policy.PolicyArn),
						PolicyName: aws.ToString(policy.PolicyName),
					},
				)
			}
			return !lastPage
		},
	)

	return rsp, trace.Wrap(err)
}

func (a *awsFetcher) fetchInstanceProfiles(ctx context.Context) ([]*accessgraphv1alpha.AWSInstanceProfileV1, error) {
	var profiles []*accessgraphv1alpha.AWSInstanceProfileV1
	iamClient, err := a.CloudClients.GetAWSIAMClient(
		ctx,
		"", /* region is empty because users and groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = iamClient.ListInstanceProfilesPagesWithContext(
		ctx,
		&iam.ListInstanceProfilesInput{
			MaxItems: &pageSize,
		},
		func(page *iam.ListInstanceProfilesOutput, lastPage bool) bool {
			for _, profile := range page.InstanceProfiles {
				profiles = append(
					profiles,
					awsInstanceProfileToProtoInstanceProfile(profile, a.AccountID),
				)
			}
			return !lastPage
		},
	)

	return profiles, trace.Wrap(err)
}

func awsInstanceProfileToProtoInstanceProfile(profile *iam.InstanceProfile, accountID string) *accessgraphv1alpha.AWSInstanceProfileV1 {
	tags := make([]*accessgraphv1alpha.AWSTag, 0, len(profile.Tags))
	for _, tag := range profile.Tags {
		tags = append(tags, &accessgraphv1alpha.AWSTag{
			Key:   aws.ToString(tag.Key),
			Value: strPtrToWrapper(tag.Value),
		})
	}

	out := &accessgraphv1alpha.AWSInstanceProfileV1{
		InstanceProfileId:   aws.ToString(profile.InstanceProfileId),
		InstanceProfileName: aws.ToString(profile.InstanceProfileName),
		Arn:                 aws.ToString(profile.Arn),
		Path:                aws.ToString(profile.Path),
		AccountId:           accountID,
		Tags:                tags,
		CreatedAt:           awsTimeToProtoTime(profile.CreateDate),
	}
	for _, role := range profile.Roles {
		if role == nil {
			continue
		}
		out.Roles = append(out.Roles, awsRoleToProtoRole(role, accountID))
	}
	return out
}

func (a *awsFetcher) fetchUsers(ctx context.Context) ([]*accessgraphv1alpha.AWSUserV1, error) {
	var users []*accessgraphv1alpha.AWSUserV1

	iamClient, err := a.CloudClients.GetAWSIAMClient(
		ctx,
		"", /* region is empty because users are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = iamClient.ListUsersPagesWithContext(ctx, &iam.ListUsersInput{
		MaxItems: &pageSize,
	},
		func(page *iam.ListUsersOutput, lastPage bool) bool {
			for _, user := range page.Users {
				users = append(users, awsUserToProtoUser(user, a.AccountID))
			}
			return !lastPage
		},
	)

	return users, trace.Wrap(err)
}

func (a *awsFetcher) fetchGroups(ctx context.Context) ([]*accessgraphv1alpha.AWSGroupV1, error) {
	var groups []*accessgraphv1alpha.AWSGroupV1

	iamClient, err := a.CloudClients.GetAWSIAMClient(
		ctx,
		"", /* region is empty because groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = iamClient.ListGroupsPagesWithContext(ctx, &iam.ListGroupsInput{
		MaxItems: &pageSize,
	},
		func(page *iam.ListGroupsOutput, lastPage bool) bool {
			for _, group := range page.Groups {
				groups = append(groups, awsGroupToProtoGroup(group, a.AccountID))
			}
			return !lastPage
		},
	)

	return groups, trace.Wrap(err)
}

func (a *awsFetcher) fetchGroupsForUser(ctx context.Context, user *accessgraphv1alpha.AWSUserV1) (*accessgraphv1alpha.AWSUserGroupsV1, error) {
	userGroups := &accessgraphv1alpha.AWSUserGroupsV1{
		User: user,
	}

	iamClient, err := a.CloudClients.GetAWSIAMClient(
		ctx,
		"", /* region is empty because users and groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = iamClient.ListGroupsForUserPagesWithContext(
		ctx,
		&iam.ListGroupsForUserInput{
			UserName: aws.String(user.UserName),
			MaxItems: &pageSize,
		},
		func(lgfuo *iam.ListGroupsForUserOutput, b bool) bool {
			for _, group := range lgfuo.Groups {
				userGroups.Groups = append(userGroups.Groups, awsGroupToProtoGroup(group, a.AccountID))
			}
			return !b
		},
	)

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return userGroups, nil
}

func (a *awsFetcher) fetchUserInlinePolicies(ctx context.Context, user *accessgraphv1alpha.AWSUserV1) ([]*accessgraphv1alpha.AWSUserInlinePolicyV1, error) {
	var policies []*accessgraphv1alpha.AWSUserInlinePolicyV1
	var errs []error
	errCollect := func(err error) {
		errs = append(errs, err)
	}
	iamClient, err := a.CloudClients.GetAWSIAMClient(
		ctx,
		"", /* region is empty because users and groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = iamClient.ListUserPoliciesPagesWithContext(
		ctx,
		&iam.ListUserPoliciesInput{
			UserName: aws.String(user.UserName),
			MaxItems: &pageSize,
		},
		func(page *iam.ListUserPoliciesOutput, lastPage bool) bool {
			for _, policyName := range page.PolicyNames {
				policy, err := iamClient.GetUserPolicyWithContext(ctx, &iam.GetUserPolicyInput{
					UserName:   aws.String(user.UserName),
					PolicyName: policyName,
				})
				if err != nil {
					errCollect(trace.Wrap(err, "failed to fetch user %q inline policy %q", user.UserName, *policyName))
					continue
				}

				policies = append(policies, awsUserPolicyToProtoUserPolicy(policy, user, a.AccountID))
			}
			return !lastPage
		})

	return policies, trace.NewAggregate(append(errs, err)...)
}

func (a *awsFetcher) fetchUserAttachedPolicies(ctx context.Context, user *accessgraphv1alpha.AWSUserV1) (*accessgraphv1alpha.AWSUserAttachedPolicies, error) {
	rsp := &accessgraphv1alpha.AWSUserAttachedPolicies{
		User:      user,
		AccountId: a.AccountID,
	}

	iamClient, err := a.CloudClients.GetAWSIAMClient(
		ctx,
		"", /* region is empty because users and groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = iamClient.ListAttachedUserPoliciesPagesWithContext(
		ctx,
		&iam.ListAttachedUserPoliciesInput{
			UserName: aws.String(user.UserName),
			MaxItems: &pageSize,
		},
		func(page *iam.ListAttachedUserPoliciesOutput, lastPage bool) bool {
			for _, policy := range page.AttachedPolicies {
				rsp.Policies = append(
					rsp.Policies,
					&accessgraphv1alpha.AttachedPolicyV1{
						Arn:        aws.ToString(policy.PolicyArn),
						PolicyName: aws.ToString(policy.PolicyName),
					},
				)
			}
			return !lastPage
		},
	)

	return rsp, trace.Wrap(err)
}

func (a *awsFetcher) fetchPolicies(ctx context.Context) ([]*accessgraphv1alpha.AWSPolicyV1, error) {
	var policies []*accessgraphv1alpha.AWSPolicyV1
	var errs []error
	var mu sync.Mutex
	collect := func(policy *accessgraphv1alpha.AWSPolicyV1, err error) {
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			errs = append(errs, err)
		}
		if policy != nil {
			policies = append(policies, policy)
		}
	}

	iamClient, err := a.CloudClients.GetAWSIAMClient(
		ctx,
		"", /* region is empty because users and groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	eGroup, ctx := errgroup.WithContext(ctx)
	eGroup.SetLimit(10)
	pageSize := int64(20)
	err = iamClient.ListPoliciesPagesWithContext(
		ctx,
		&iam.ListPoliciesInput{
			MaxItems: &pageSize,
		},
		func(page *iam.ListPoliciesOutput, lastPage bool) bool {
			pp := page.Policies
			eGroup.Go(func() error {
				for _, policy := range pp {
					out, err := iamClient.GetPolicyVersionWithContext(ctx, &iam.GetPolicyVersionInput{
						PolicyArn: policy.Arn,
						VersionId: policy.DefaultVersionId,
					})
					if err != nil {
						collect(nil, trace.Wrap(err, "failed to fetch policy %q", *policy.Arn))
						continue
					}
					collect(
						awsPolicyToProtoPolicy(
							policy,
							[]byte(aws.ToString(out.PolicyVersion.Document)),
							a.AccountID,
						),
						nil,
					)
				}
				return nil
			})
			return !lastPage
		},
	)
	eGroup.Wait()
	return policies, trace.NewAggregate(append(errs, err)...)
}

// awsGroupToProtoGroup converts an AWS IAM group to a proto group.
func awsGroupToProtoGroup(group *iam.Group, accountID string) *accessgraphv1alpha.AWSGroupV1 {
	return &accessgraphv1alpha.AWSGroupV1{
		Name:      aws.ToString(group.GroupName),
		Arn:       aws.ToString(group.Arn),
		Path:      aws.ToString(group.Path),
		GroupId:   aws.ToString(group.GroupId),
		CreatedAt: awsTimeToProtoTime(group.CreateDate),
		AccountId: accountID,
	}
}

// awsUserToProtoUser converts an AWS IAM user to a proto user.
func awsUserToProtoUser(user *iam.User, accountID string) *accessgraphv1alpha.AWSUserV1 {
	tags := make([]*accessgraphv1alpha.AWSTag, 0, len(user.Tags))
	for _, tag := range user.Tags {
		tags = append(tags, &accessgraphv1alpha.AWSTag{
			Key:   aws.ToString(tag.Key),
			Value: strPtrToWrapper(tag.Value),
		})
	}

	var permissionsBoundary *accessgraphv1alpha.UsersPermissionsBoundaryV1

	if user.PermissionsBoundary != nil {
		permissionsBoundary = &accessgraphv1alpha.UsersPermissionsBoundaryV1{
			PermissionsBoundaryArn:  aws.ToString(user.PermissionsBoundary.PermissionsBoundaryArn),
			PermissionsBoundaryType: accessgraphv1alpha.UsersPermissionsBoundaryType_USERS_PERMISSIONS_BOUNDARY_TYPE_PERMISSIONS_BOUNDARY_POLICY,
		}
	}

	return &accessgraphv1alpha.AWSUserV1{
		UserName:            aws.ToString(user.UserName),
		Arn:                 aws.ToString(user.Arn),
		Path:                aws.ToString(user.Path),
		UserId:              aws.ToString(user.UserId),
		CreatedAt:           awsTimeToProtoTime(user.CreateDate),
		AccountId:           accountID,
		PasswordLastUsed:    awsTimeToProtoTime(user.PasswordLastUsed),
		Tags:                tags,
		PermissionsBoundary: permissionsBoundary,
	}
}

func awsTimeToProtoTime(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

func strPtrToWrapper(s *string) *wrapperspb.StringValue {
	if s == nil {
		return nil
	}
	return &wrapperspb.StringValue{Value: *s}
}

func awsUserPolicyToProtoUserPolicy(policy *iam.GetUserPolicyOutput, user *accessgraphv1alpha.AWSUserV1, accountID string) *accessgraphv1alpha.AWSUserInlinePolicyV1 {
	return &accessgraphv1alpha.AWSUserInlinePolicyV1{
		PolicyName:     aws.ToString(policy.PolicyName),
		PolicyDocument: []byte(aws.ToString(policy.PolicyDocument)),
		User:           user,
		AccountId:      accountID,
	}
}

func awsRolePolicyToProtoUserPolicy(policy *iam.GetRolePolicyOutput, role *accessgraphv1alpha.AWSRoleV1, accountID string) *accessgraphv1alpha.AWSRoleInlinePolicyV1 {
	return &accessgraphv1alpha.AWSRoleInlinePolicyV1{
		PolicyName:     aws.ToString(policy.PolicyName),
		PolicyDocument: []byte(aws.ToString(policy.PolicyDocument)),
		AwsRole:        role,
		AccountId:      accountID,
	}
}

func awsPolicyToProtoPolicy(policy *iam.Policy, policyDoc []byte, accountID string) *accessgraphv1alpha.AWSPolicyV1 {
	tags := make([]*accessgraphv1alpha.AWSTag, 0, len(policy.Tags))
	for _, tag := range policy.Tags {
		tags = append(tags, &accessgraphv1alpha.AWSTag{
			Key:   aws.ToString(tag.Key),
			Value: strPtrToWrapper(tag.Value),
		})
	}
	return &accessgraphv1alpha.AWSPolicyV1{
		PolicyName:       aws.ToString(policy.PolicyName),
		Arn:              aws.ToString(policy.Arn),
		CreatedAt:        awsTimeToProtoTime(policy.CreateDate),
		DefaultVersionId: aws.ToString(policy.DefaultVersionId),
		Description:      aws.ToString(policy.Description),
		IsAttachable:     aws.ToBool(policy.IsAttachable),
		Path:             aws.ToString(policy.Path),
		UpdatedAt:        awsTimeToProtoTime(policy.UpdateDate),
		PolicyId:         aws.ToString(policy.PolicyId),
		Tags:             tags,
		PolicyDocument:   policyDoc,
		AccountId:        accountID,
	}
}

func (a *awsFetcher) fetchGroupAttachedPolicies(ctx context.Context, group *accessgraphv1alpha.AWSGroupV1) (*accessgraphv1alpha.AWSGroupAttachedPolicies, error) {
	rsp := &accessgraphv1alpha.AWSGroupAttachedPolicies{
		Group:     group,
		AccountId: a.AccountID,
	}
	iamClient, err := a.CloudClients.GetAWSIAMClient(
		ctx,
		"", /* region is empty because users and groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = iamClient.ListAttachedGroupPoliciesPagesWithContext(
		ctx,
		&iam.ListAttachedGroupPoliciesInput{
			GroupName: aws.String(group.Name),
			MaxItems:  &pageSize,
		},
		func(page *iam.ListAttachedGroupPoliciesOutput, lastPage bool) bool {
			for _, policy := range page.AttachedPolicies {
				rsp.Policies = append(
					rsp.Policies,
					&accessgraphv1alpha.AttachedPolicyV1{
						Arn:        aws.ToString(policy.PolicyArn),
						PolicyName: aws.ToString(policy.PolicyName),
					},
				)
			}
			return !lastPage
		},
	)

	return rsp, trace.Wrap(err)
}

func (a *awsFetcher) fetchGroupInlinePolicies(ctx context.Context, group *accessgraphv1alpha.AWSGroupV1) ([]*accessgraphv1alpha.AWSGroupInlinePolicyV1, error) {
	var policies []*accessgraphv1alpha.AWSGroupInlinePolicyV1
	var errs []error
	errCollect := func(err error) {
		errs = append(errs, err)
	}

	iamClient, err := a.CloudClients.GetAWSIAMClient(
		ctx,
		"", /* region is empty because users and groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = iamClient.ListGroupPoliciesPagesWithContext(
		ctx,
		&iam.ListGroupPoliciesInput{
			GroupName: aws.String(group.Name),
			MaxItems:  &pageSize,
		},
		func(page *iam.ListGroupPoliciesOutput, lastPage bool) bool {
			for _, policyName := range page.PolicyNames {
				policy, err := iamClient.GetGroupPolicyWithContext(ctx, &iam.GetGroupPolicyInput{
					GroupName:  aws.String(group.Name),
					PolicyName: policyName,
				})
				if err != nil {
					errCollect(trace.Wrap(err, "failed to fetch group %q inline policy %q", group.Name, *policyName))
					continue
				}
				policies = append(policies, awsGroupPolicyToProtoGroupPolicy(policy, a.AccountID, group))
			}
			return !lastPage
		},
	)

	return policies, trace.Wrap(err)
}

func awsGroupPolicyToProtoGroupPolicy(policy *iam.GetGroupPolicyOutput, accountID string, group *accessgraphv1alpha.AWSGroupV1) *accessgraphv1alpha.AWSGroupInlinePolicyV1 {
	return &accessgraphv1alpha.AWSGroupInlinePolicyV1{
		PolicyName:     aws.ToString(policy.PolicyName),
		PolicyDocument: []byte(aws.ToString(policy.PolicyDocument)),
		Group:          group,
		AccountId:      accountID,
	}
}

// getAWSOptions returns a list of AWSAssumeRoleOptionFn to be used when
// creating AWS clients.
func (s *awsFetcher) getAWSOptions() []cloud.AWSAssumeRoleOptionFn {
	opts := []cloud.AWSAssumeRoleOptionFn{
		cloud.WithAmbientCredentials(),
	}

	if s.Config.AssumeRole != "" {
		opts = append(opts, cloud.WithAssumeRole(s.Config.AssumeRole, s.Config.ExternalID))
	}
	return opts
}

func (a *awsFetcher) fetchS3Buckets(ctx context.Context) ([]*accessgraphv1alpha.AWSS3BucketV1, error) {
	var s3s []*accessgraphv1alpha.AWSS3BucketV1
	var errs []error
	var mu sync.Mutex
	eG, ctx := errgroup.WithContext(ctx)
	eG.SetLimit(10)
	collect := func(s3 *accessgraphv1alpha.AWSS3BucketV1, err error) {
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			errs = append(errs, err)
		}
		if s3 != nil {
			s3s = append(s3s, s3)
		}
	}

	s3Client, err := a.CloudClients.GetAWSS3Client(
		ctx,
		"", /* region is empty because users and groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rsp, err := s3Client.ListBucketsWithContext(
		ctx,
		&s3.ListBucketsInput{},
	)

	for _, bucket := range rsp.Buckets {
		bucket := bucket
		eG.Go(func() error {
			policy, err := s3Client.GetBucketPolicyWithContext(ctx, &s3.GetBucketPolicyInput{
				Bucket: bucket.Name,
			})
			if err != nil {
				collect(nil, trace.Wrap(err, "failed to fetch bucket %q inline policy", aws.ToString(bucket.Name)))
			}

			policyStatus, err := s3Client.GetBucketPolicyStatusWithContext(ctx, &s3.GetBucketPolicyStatusInput{
				Bucket: bucket.Name,
			})
			if err != nil {
				collect(nil, trace.Wrap(err, "failed to fetch bucket %q policy status", aws.ToString(bucket.Name)))
			}

			acls, err := s3Client.GetBucketAclWithContext(ctx, &s3.GetBucketAclInput{
				Bucket: bucket.Name,
			})
			if err != nil {
				collect(nil, trace.Wrap(err, "failed to fetch bucket %q acls policies", aws.ToString(bucket.Name)))
			}
			collect(
				awsS3Bucket(aws.ToString(bucket.Name), policy, policyStatus, acls, a.AccountID),
				nil)
			return nil
		})
	}

	_ = eG.Wait()

	return s3s, trace.Wrap(err)
}

func awsS3Bucket(name string, policy *s3.GetBucketPolicyOutput, policyStatus *s3.GetBucketPolicyStatusOutput, acls *s3.GetBucketAclOutput, accountID string) *accessgraphv1alpha.AWSS3BucketV1 {
	s3 := &accessgraphv1alpha.AWSS3BucketV1{
		Name:      name,
		AccountId: accountID,
	}
	if policy != nil {
		s3.PolicyDocument = []byte(aws.ToString(policy.Policy))
	}
	if policyStatus != nil && policyStatus.PolicyStatus != nil {
		s3.IsPublic = aws.ToBool(policyStatus.PolicyStatus.IsPublic)
	}
	if acls != nil {
		s3.Acls = awsACLsToProtoACLs(acls.Grants)
	}
	return s3
}

func awsACLsToProtoACLs(grants []*s3.Grant) []*accessgraphv1alpha.AWSS3BucketACL {
	var acls []*accessgraphv1alpha.AWSS3BucketACL
	for _, grant := range grants {
		acls = append(acls, &accessgraphv1alpha.AWSS3BucketACL{
			Grantee: &accessgraphv1alpha.AWSS3BucketACLGrantee{
				Id:           aws.ToString(grant.Grantee.ID),
				DisplayName:  aws.ToString(grant.Grantee.DisplayName),
				Type:         aws.ToString(grant.Grantee.Type),
				Uri:          aws.ToString(grant.Grantee.URI),
				EmailAddress: aws.ToString(grant.Grantee.EmailAddress),
			},
			Permission: aws.ToString(grant.Permission),
		})
	}
	return acls
}

// awsRoleToProtoRole converts an AWS IAM Role to a proto Role.
func awsRoleToProtoRole(role *iam.Role, accountID string) *accessgraphv1alpha.AWSRoleV1 {
	tags := make([]*accessgraphv1alpha.AWSTag, 0, len(role.Tags))
	for _, tag := range role.Tags {
		tags = append(tags, &accessgraphv1alpha.AWSTag{
			Key:   aws.ToString(tag.Key),
			Value: strPtrToWrapper(tag.Value),
		})
	}

	var permissionsBoundary *accessgraphv1alpha.RolePermissionsBoundaryV1

	if role.PermissionsBoundary != nil {
		permissionsBoundary = &accessgraphv1alpha.RolePermissionsBoundaryV1{
			PermissionsBoundaryArn:  aws.ToString(role.PermissionsBoundary.PermissionsBoundaryArn),
			PermissionsBoundaryType: accessgraphv1alpha.RolePermissionsBoundaryType_ROLE_PERMISSIONS_BOUNDARY_TYPE_PERMISSIONS_BOUNDARY_POLICY,
		}
	}

	var lastTimeUsed *accessgraphv1alpha.RoleLastUsedV1
	if role.RoleLastUsed != nil {
		lastTimeUsed = &accessgraphv1alpha.RoleLastUsedV1{
			LastUsedDate: awsTimeToProtoTime(role.RoleLastUsed.LastUsedDate),
			Region:       aws.ToString(role.RoleLastUsed.Region),
		}
	}

	return &accessgraphv1alpha.AWSRoleV1{
		Name:                     aws.ToString(role.RoleName),
		Arn:                      aws.ToString(role.Arn),
		AssumeRolePolicyDocument: strPtrToByteSlice(role.AssumeRolePolicyDocument),
		Path:                     aws.ToString(role.Path),
		Description:              aws.ToString(role.Description),
		MaxSessionDuration:       durationpb.New(time.Duration(aws.ToInt64(role.MaxSessionDuration)) * time.Second),
		RoleId:                   aws.ToString(role.RoleId),
		CreatedAt:                awsTimeToProtoTime(role.CreateDate),
		AccountId:                accountID,
		RoleLastUsed:             lastTimeUsed,
		Tags:                     tags,
		PermissionsBoundary:      permissionsBoundary,
	}
}

func strPtrToByteSlice(s *string) []byte {
	if s == nil {
		return nil
	}
	return []byte(*s)
}
