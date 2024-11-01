package sdk

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	identitystoretypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/gravitational/trace"
)

// Client interface exports methods supported by this AWS Identity Center SDK.
type Client interface {
	// ListAccounts lists Identity Center accounts.
	ListAccounts(ctx context.Context) ([]*Account, error)
	// ListAccountsWithAssignedPermissionSetARNs lists Identity Center accounts with assigned permission sets.
	ListAccountsWithAssignedPermissionSetARNs(ctx context.Context) ([]*AccountWithPermissionSetARNs, error)
	// ListPermissionSetARNsForAccount lists permission set ARNs that are currently assigned to an Identity Center account.
	ListPermissionSetARNsForAccount(ctx context.Context, accountID string) ([]string, error)
	// ListPermissionSets lists permission sets that exist in the Identity Center.
	ListPermissionSets(ctx context.Context) ([]*PermissionSet, error)
	// ListGroups lists Identity Center user groups.
	ListGroups(ctx context.Context) ([]*Group, error)
	// ListGroupMemberships lists members (users) currently assigned to the Identity Center user groups.
	ListGroupMemberships(ctx context.Context, groupID string) ([]*GroupMember, error)
	// ListGroupsWithMembers lists Identity Center user groups with its respective members.
	ListGroupsWithMembers(ctx context.Context) ([]*GroupWithMembers, error)
	// ListGroupsWithAccountAndPermAssignment lists Identity Center groups with assigned accounts and permission sets.
	ListGroupsWithAccountAndPermAssignment(ctx context.Context) ([]*GroupWithAssignment, error)
	// ListUsersWithAccountAndPermAssignment lists Identity Center users with assigned accounts and permission sets.
	ListUsersWithAccountAndPermAssignment(ctx context.Context) ([]*UserWithAssignment, error)
	// ListUsers lists all available users in the Identity Center.
	ListUsers(ctx context.Context) ([]*User, error)
	// ListUserAssignments lists account assignment for a user.
	ListUserAssignments(ctx context.Context, userID string) ([]*Assigment, error)
	// ListGroupsAssignments lists account assignment for a user group.
	ListGroupsAssignments(ctx context.Context, groupID string) ([]*Assigment, error)
}

// New creates a new AWS Identity Center SDK client.
func New(config Config) (Client, error) {
	if err := config.checkAndSetDefault(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &client{
		ssoAdminClient:      ssoadmin.NewFromConfig(*config.AWSConfig),
		identityStoreClient: identitystore.NewFromConfig(*config.AWSConfig),
		organizationsClient: organizations.NewFromConfig(*config.AWSConfig),
		Config:              &config,
	}, nil
}

// client is an AWS Identity Center SDK client.
type client struct {
	*Config

	identityStoreClient identityStoreClient
	ssoAdminClient      ssoAdminClient
	organizationsClient organizationsClient
}

// ListAccounts lists Identity Center accounts.
func (c *client) ListAccounts(ctx context.Context) ([]*Account, error) {
	var nextToken *string
	var out []*Account
	for {
		resp, err := c.organizationsClient.ListAccounts(ctx, &organizations.ListAccountsInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, v := range resp.Accounts {
			out = append(out, &Account{
				ARN:  aws.ToString(v.Arn),
				ID:   aws.ToString(v.Id),
				Name: aws.ToString(v.Name),
			})
		}
		nextToken = resp.NextToken
		if nextToken == nil {
			break
		}
	}
	return out, nil
}

// ListAccountsWithAssignedPermissionSetARNs lists Identity Center accounts with assigned permission sets.
func (c *client) ListAccountsWithAssignedPermissionSetARNs(ctx context.Context) ([]*AccountWithPermissionSetARNs, error) {
	accounts, err := c.ListAccounts(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]*AccountWithPermissionSetARNs, 0, len(accounts))
	for _, a := range accounts {
		permsetARNs, err := c.ListPermissionSetARNsForAccount(ctx, a.ID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, &AccountWithPermissionSetARNs{
			Account:           a,
			PermissionSetARNs: permsetARNs,
		})
	}

	return out, nil
}

// ListPermissionSetARNsForAccount lists permission set ARNs that are currently assigned to an Identity Center account.
func (c *client) ListPermissionSetARNsForAccount(ctx context.Context, accountID string) ([]string, error) {
	var nextToken *string
	var out []string
	for {
		resp, err := c.ssoAdminClient.ListPermissionSetsProvisionedToAccount(ctx, &ssoadmin.ListPermissionSetsProvisionedToAccountInput{
			InstanceArn: aws.String(c.InstanceARN),
			AccountId:   aws.String(accountID),
			NextToken:   nextToken,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, resp.PermissionSets...)
		nextToken = resp.NextToken
		if nextToken == nil {
			break
		}
	}
	return out, nil
}

// ListGroups lists Identity Center user groups.
func (c *client) ListGroups(ctx context.Context) ([]*Group, error) {
	descResp, err := c.ssoAdminClient.DescribeInstance(ctx, &ssoadmin.DescribeInstanceInput{
		InstanceArn: aws.String(c.InstanceARN),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var nextToken *string
	var out []*Group
	for {
		groups, err := c.identityStoreClient.ListGroups(ctx, &identitystore.ListGroupsInput{
			IdentityStoreId: descResp.IdentityStoreId,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, v := range groups.Groups {
			out = append(out, &Group{
				ID:              aws.ToString(v.GroupId),
				DisplayName:     aws.ToString(v.DisplayName),
				IdentityStoreID: aws.ToString(v.IdentityStoreId),
			})
		}
		nextToken = groups.NextToken
		if nextToken == nil {
			break
		}
	}
	return out, nil
}

// ListGroupsWithAccountAndPermAssignment lists Identity Center groups with assigned accounts and permission sets.
func (c *client) ListGroupsWithAccountAndPermAssignment(ctx context.Context) ([]*GroupWithAssignment, error) {
	groups, err := c.ListGroups(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := make([]*GroupWithAssignment, 0, len(groups))
	for _, g := range groups {
		assignments, err := c.ListGroupsAssignments(ctx, g.ID)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		out = append(out, &GroupWithAssignment{
			Group:       g,
			Assignments: assignments,
		})
	}
	return out, nil
}

// ListUsersWithAccountAndPermAssignment lists Identity Center users with assigned accounts and permission sets.
func (c *client) ListUsersWithAccountAndPermAssignment(ctx context.Context) ([]*UserWithAssignment, error) {
	users, err := c.ListUsers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]*UserWithAssignment, 0, len(users))
	for _, v := range users {
		assignments, err := c.ListUserAssignments(ctx, v.ID)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		out = append(out, &UserWithAssignment{
			User:        v,
			Assignments: assignments,
		})
	}
	return out, nil
}

// ListGroupMemberships lists members (users) currently assigned to the Identity Center user groups.
func (c *client) ListGroupMemberships(ctx context.Context, groupID string) ([]*GroupMember, error) {
	descResp, err := c.ssoAdminClient.DescribeInstance(ctx, &ssoadmin.DescribeInstanceInput{
		InstanceArn: aws.String(c.InstanceARN),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var nextToken *string
	var out []*GroupMember
	resp, err := c.identityStoreClient.ListGroupMemberships(ctx, &identitystore.ListGroupMembershipsInput{
		IdentityStoreId: descResp.IdentityStoreId,
		GroupId:         aws.String(groupID),
		NextToken:       nextToken,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, v := range resp.GroupMemberships {
		var memberID string
		switch t := v.MemberId.(type) {
		case *identitystoretypes.MemberIdMemberUserId:
			memberID = t.Value
		case *identitystoretypes.UnknownUnionMember:
			// TODO: investigate why this happen.
			continue
		default:
			return nil, trace.BadParameter("unexpected member ID type: %T", t)
		}
		out = append(out, &GroupMember{
			MemberID: memberID,
		})

		nextToken = resp.NextToken
		if nextToken == nil {
			break
		}
	}
	return out, nil
}

// ListGroupsWithMembers lists Identity Center user groups with its respective members.
func (c *client) ListGroupsWithMembers(ctx context.Context) ([]*GroupWithMembers, error) {
	groups, err := c.ListGroups(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []*GroupWithMembers
	for _, g := range groups {
		members, err := c.ListGroupMemberships(ctx, g.ID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, &GroupWithMembers{
			Group:   g,
			Members: members,
		})
	}
	return out, nil
}

// ListPermissionSets lists permission sets that exist in the Identity Center.
func (c *client) ListPermissionSets(ctx context.Context) ([]*PermissionSet, error) {
	var nextToken *string
	var allPermissionsSets []string
	for {
		groups, err := c.ssoAdminClient.ListPermissionSets(ctx, &ssoadmin.ListPermissionSetsInput{
			InstanceArn: aws.String(c.InstanceARN),
			NextToken:   nextToken,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		allPermissionsSets = append(allPermissionsSets, groups.PermissionSets...)

		nextToken = groups.NextToken
		if nextToken == nil {
			break
		}
	}

	var out []*PermissionSet
	for _, ps := range allPermissionsSets {
		descResp, err := c.ssoAdminClient.DescribePermissionSet(ctx, &ssoadmin.DescribePermissionSetInput{
			InstanceArn:      aws.String(c.InstanceARN),
			PermissionSetArn: aws.String(ps),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if descResp.PermissionSet == nil {
			continue
		}
		out = append(out, &PermissionSet{
			ARN:         aws.ToString(descResp.PermissionSet.PermissionSetArn),
			Name:        aws.ToString(descResp.PermissionSet.Name),
			Description: aws.ToString(descResp.PermissionSet.Description),
		})
	}
	return out, nil
}

// ListUsers lists all users available users in the Identity Center.
func (c *client) ListUsers(ctx context.Context) ([]*User, error) {
	descResp, err := c.ssoAdminClient.DescribeInstance(ctx, &ssoadmin.DescribeInstanceInput{
		InstanceArn: aws.String(c.InstanceARN),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var nextToken *string
	var out []*User
	for {
		resp, err := c.identityStoreClient.ListUsers(ctx, &identitystore.ListUsersInput{
			IdentityStoreId: descResp.IdentityStoreId,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, v := range resp.Users {
			out = append(out, &User{
				ID:       aws.ToString(v.UserId),
				UserName: aws.ToString(v.UserName),
			})
		}
		nextToken = resp.NextToken
		if nextToken == nil {
			break
		}
	}
	return out, nil
}

// ListUserAssignments lists account assignment for a user.
func (c *client) ListUserAssignments(ctx context.Context, userID string) ([]*Assigment, error) {
	result, err := c.listAssigment(ctx, userID, ssoadmintypes.PrincipalTypeUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return result, nil
}

// ListGroupsAssignments lists account assignment for a user group.
func (c *client) ListGroupsAssignments(ctx context.Context, groupID string) ([]*Assigment, error) {
	result, err := c.listAssigment(ctx, groupID, ssoadmintypes.PrincipalTypeGroup)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return result, nil
}

// listAssigment lists account assignment for a given principal, which can either be a user or a user group.
func (c *client) listAssigment(ctx context.Context, principalID string, principalType ssoadmintypes.PrincipalType) ([]*Assigment, error) {
	var nextToken *string
	var out []*Assigment
	for {
		resp, err := c.ssoAdminClient.ListAccountAssignmentsForPrincipal(ctx, &ssoadmin.ListAccountAssignmentsForPrincipalInput{
			InstanceArn:   aws.String(c.InstanceARN),
			NextToken:     nextToken,
			PrincipalType: principalType,
			PrincipalId:   aws.String(principalID),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, v := range resp.AccountAssignments {
			out = append(out, &Assigment{
				AccountID:        aws.ToString(v.AccountId),
				PermissionSetARN: aws.ToString(v.PermissionSetArn),
			})
		}
		nextToken = resp.NextToken
		if nextToken == nil {
			break
		}
	}
	return out, nil
}
