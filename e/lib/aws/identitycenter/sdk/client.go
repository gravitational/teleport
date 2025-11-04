package sdk

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	identitystoretypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/gravitational/trace"

	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
)

type InstanceDescriber interface {
	// DescribeInstance fetches data about the identity center instance itself.
	DescribeInstance(context.Context) (*InstanceInfo, error)
}

// Client interface exports methods supported by this AWS Identity Center SDK.
type Client interface {
	InstanceDescriber

	// ListAccounts lists Identity Center accounts.
	ListAccounts(ctx context.Context) ([]*Account, error)
	// ListPermissionSetARNsForAccount lists permission set ARNs that are currently assigned to an Identity Center account.
	ListPermissionSetARNsForAccount(ctx context.Context, accountID string) ([]string, error)
	// ListPermissionSets lists permission sets that exist in the Identity Center.
	ListPermissionSets(ctx context.Context) ([]*PermissionSet, error)
	// ListGroups lists Identity Center user groups.
	ListGroups(ctx context.Context) ([]*Group, error)
	// ListGroupMemberships lists members (users) currently assigned to the Identity Center user groups.
	ListGroupMemberships(ctx context.Context, groupID string) ([]*GroupMember, error)
	// ListUsers lists all available users in the Identity Center.
	ListUsers(ctx context.Context) ([]*User, error)
	// ListUserAssignments lists account assignment for a user.
	ListUserAssignments(ctx context.Context, userID string) ([]*Assignment, error)
	// ListGroupsAssignments lists account assignment for a user group.
	ListGroupsAssignments(ctx context.Context, groupID string) ([]*Assignment, error)
	// ListAssignments lists account assignment for a given principal, which can either be a user or a user group.
	ListAssignments(ctx context.Context, principalID string, principalType ssoadmintypes.PrincipalType) ([]*Assignment, error)
	// CreateAccountAssignment creates an account assignment for a user.
	CreateAccountAssignment(ctx context.Context, req *CreateAccountAssignmentRequest) (*AccountAssignmentResponse, error)
	// WaitForCreateAccountAssignmentResult waits until the account assignment creation reaches a terminal state
	// by tracking the status of the account assignment operation using the request ID.
	WaitForCreateAccountAssignmentResult(ctx context.Context, requestID string) error
	// DeleteAccountAssignment deletes an account assignment for a user.
	DeleteAccountAssignment(ctx context.Context, req *DeleteAccountAssignmentRequest) (*AccountAssignmentResponse, error)
	// WaitForDeleteAccountAssignmentResult waits until the account assignment deletion reaches a terminal state
	// by tracking the status of the account assignment operation using the request ID.
	WaitForDeleteAccountAssignmentResult(ctx context.Context, requestID string) error
	// ValidateResourceSyncCredential verifies that the credential set up for resource sync is valid.
	ValidateResourceSyncCredential(ctx context.Context) error
}

// ClientProvider is a function that creates a new AWS Identity Center SDK client.
// Please note that the ClientProvider is not thread-safe and
// should be used only in integration tests.
var ClientProvider = nativeClientProvider

// New creates a new AWS Identity Center SDK client.
func New(config Config) (Client, error) {
	c, err := ClientProvider(config)
	return c, trace.Wrap(err)
}

func nativeClientProvider(config Config) (Client, error) {
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

// DescribeInstance fetches information about the configured Identity Center
// instance
func (c *client) DescribeInstance(ctx context.Context) (*InstanceInfo, error) {
	c.Config.Logger.DebugContext(ctx, "Querying Identity Center instance data")

	instance, err := c.ssoAdminClient.DescribeInstance(ctx, &ssoadmin.DescribeInstanceInput{
		InstanceArn: aws.String(c.Config.InstanceARN),
	})
	if err != nil {
		return nil, trace.Wrap(err, "querying Identity Center instance")
	}

	return &InstanceInfo{
		Name:            aws.ToString(instance.Name),
		IdentityStoreID: IdentityStoreID(aws.ToString(instance.IdentityStoreId)),
		Status:          instance.Status,
		OwnerAccountID:  aws.ToString(instance.OwnerAccountId),
	}, nil
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
		listing, err := c.identityStoreClient.ListGroups(ctx, &identitystore.ListGroupsInput{
			IdentityStoreId: descResp.IdentityStoreId,
			NextToken:       nextToken,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, awsGroup := range listing.Groups {
			out = append(out, &Group{
				ID:              aws.ToString(awsGroup.GroupId),
				DisplayName:     aws.ToString(awsGroup.DisplayName),
				IdentityStoreID: aws.ToString(awsGroup.IdentityStoreId),
			})
		}

		nextToken = listing.NextToken
		if nextToken == nil {
			break
		}
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

	// 100 is max result size supported by AWS.
	// https://docs.aws.amazon.com/singlesignon/latest/IdentityStoreAPIReference/API_ListGroupMemberships.html#API_ListGroupMemberships_RequestSyntax
	maxResult := int32(100)

	var nextToken *string
	var out []*GroupMember
	for {
		resp, err := c.identityStoreClient.ListGroupMemberships(ctx, &identitystore.ListGroupMembershipsInput{
			IdentityStoreId: descResp.IdentityStoreId,
			GroupId:         aws.String(groupID),
			NextToken:       nextToken,
			MaxResults:      &maxResult,
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
		}

		nextToken = resp.NextToken
		if nextToken == nil {
			break
		}
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
			NextToken:       nextToken,
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
func (c *client) ListUserAssignments(ctx context.Context, userID string) ([]*Assignment, error) {
	result, err := c.ListAssignments(ctx, userID, ssoadmintypes.PrincipalTypeUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return result, nil
}

// ListGroupsAssignments lists account assignment for a user group.
func (c *client) ListGroupsAssignments(ctx context.Context, groupID string) ([]*Assignment, error) {
	result, err := c.ListAssignments(ctx, groupID, ssoadmintypes.PrincipalTypeGroup)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return result, nil
}

// ListAssignments lists account assignment for a given principal, which can either be a user or a user group.
// Note: The assignment struct returned by this method should always be aligned with convertToICSDKAssignments
// function in provisioning package. Otherwise the difference in field type will make diff calculator
// produce incorrect diff value.
func (c *client) ListAssignments(ctx context.Context, principalID string, principalType ssoadmintypes.PrincipalType) ([]*Assignment, error) {
	var nextToken *string
	var out []*Assignment
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
			out = append(out, &Assignment{
				AccountID:        aws.ToString(v.AccountId),
				PermissionSetARN: aws.ToString(v.PermissionSetArn),
				PrincipalType:    v.PrincipalType,
			})
		}
		nextToken = resp.NextToken
		if nextToken == nil {
			break
		}
	}
	return out, nil
}

// AccountAssignmentResponse represents the response of an account assignment operation.
type AccountAssignmentResponse struct {
	// RequestID is the ID of the request that allows to track the status of the account assignment operation.
	RequestID string
	// Status is the status of the account assignment operation.
	Status ssoadmintypes.StatusValues
	// FailureReason is the reason of the failure if the account assignment operation failed.
	FailureReason string
}

// CreateAccountAssignmentRequest represents the request to create an account assignment.
type CreateAccountAssignmentRequest struct {
	// PrincipalID is the ID of the principal AWS IC Group or User.
	PrincipalID string
	// PermissionSetARN is the ARN of the permission set.
	PermissionSetARN string
	// PrincipalType is the type of the principal.
	PrincipalType ssoadmintypes.PrincipalType
	// AccountID is the ID of the AWS account.
	AccountID string
}

// CreateAccountAssignment creates an account assignment between account/permission set and a principal User or Group.
func (c *client) CreateAccountAssignment(ctx context.Context, req *CreateAccountAssignmentRequest) (*AccountAssignmentResponse, error) {
	resp, err := c.ssoAdminClient.CreateAccountAssignment(ctx, &ssoadmin.CreateAccountAssignmentInput{
		InstanceArn:      aws.String(c.InstanceARN),
		PrincipalId:      aws.String(req.PrincipalID),
		PermissionSetArn: aws.String(req.PermissionSetARN),
		TargetId:         aws.String(req.AccountID),
		TargetType:       ssoadmintypes.TargetTypeAwsAccount,
		PrincipalType:    req.PrincipalType,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if resp.AccountAssignmentCreationStatus == nil {
		return nil, trace.BadParameter("missing account assignment creation status")
	}
	return &AccountAssignmentResponse{
		Status:        resp.AccountAssignmentCreationStatus.Status,
		FailureReason: aws.ToString(resp.AccountAssignmentCreationStatus.FailureReason),
		RequestID:     aws.ToString(resp.AccountAssignmentCreationStatus.RequestId),
	}, nil
}

// DeleteAccountAssignmentRequest represents the request to delete an account assignment.
type DeleteAccountAssignmentRequest struct {
	// PrincipalID is the ID of the principal AWS IC Group or User.
	PrincipalID string
	// PermissionSetARN is the ARN of the permission set.
	PermissionSetARN string
	// PrincipalType is the type of the principal.
	PrincipalType ssoadmintypes.PrincipalType
	// AccountID is the ID of the AWS account.
	AccountID string
}

// DeleteAccountAssignment deletes an account assignment for groups and users .
func (c *client) DeleteAccountAssignment(ctx context.Context, req *DeleteAccountAssignmentRequest) (*AccountAssignmentResponse, error) {
	resp, err := c.ssoAdminClient.DeleteAccountAssignment(ctx, &ssoadmin.DeleteAccountAssignmentInput{
		InstanceArn:      aws.String(c.InstanceARN),
		PermissionSetArn: aws.String(req.PermissionSetARN),
		TargetId:         aws.String(req.AccountID),
		TargetType:       ssoadmintypes.TargetTypeAwsAccount,
		PrincipalType:    req.PrincipalType,
		PrincipalId:      aws.String(req.PrincipalID),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if resp.AccountAssignmentDeletionStatus == nil {
		return nil, trace.BadParameter("account assignment deletion status is missing")
	}
	return &AccountAssignmentResponse{
		Status:        resp.AccountAssignmentDeletionStatus.Status,
		FailureReason: aws.ToString(resp.AccountAssignmentDeletionStatus.FailureReason),
		RequestID:     aws.ToString(resp.AccountAssignmentDeletionStatus.RequestId),
	}, nil
}

// WaitForCreateAccountAssignmentResult waits until the account assignment creation reaches a terminal state.
func (c *client) WaitForCreateAccountAssignmentResult(ctx context.Context, requestID string) error {
	for {
		resp, err := c.ssoAdminClient.DescribeAccountAssignmentCreationStatus(ctx, &ssoadmin.DescribeAccountAssignmentCreationStatusInput{
			InstanceArn:                        aws.String(c.InstanceARN),
			AccountAssignmentCreationRequestId: aws.String(requestID),
		})
		if err != nil {
			return trace.Wrap(err)
		}
		if resp.AccountAssignmentCreationStatus == nil {
			return trace.BadParameter("missing account assignment creation status")
		}

		switch resp.AccountAssignmentCreationStatus.Status {
		case ssoadmintypes.StatusValuesSucceeded:
			return nil
		case ssoadmintypes.StatusValuesFailed:
			return trace.Errorf("account assignment creation failed: %s", aws.ToString(resp.AccountAssignmentCreationStatus.FailureReason))
		case ssoadmintypes.StatusValuesInProgress:
			// continue waiting
		}

		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-c.Clock.After(2 * time.Second):
			continue
		}
	}
}

// WaitForDeleteAccountAssignmentResult waits until the account assignment deletion reaches a terminal state.
func (c *client) WaitForDeleteAccountAssignmentResult(ctx context.Context, requestID string) error {
	for {
		resp, err := c.ssoAdminClient.DescribeAccountAssignmentDeletionStatus(ctx, &ssoadmin.DescribeAccountAssignmentDeletionStatusInput{
			InstanceArn:                        aws.String(c.InstanceARN),
			AccountAssignmentDeletionRequestId: aws.String(requestID),
		})
		if err != nil {
			return trace.Wrap(err)
		}
		if resp.AccountAssignmentDeletionStatus == nil {
			return trace.BadParameter("missing account assignment deletion status")
		}

		switch resp.AccountAssignmentDeletionStatus.Status {
		case ssoadmintypes.StatusValuesSucceeded:
			return nil
		case ssoadmintypes.StatusValuesFailed:
			return trace.Errorf("account assignment deletion failed: %s", aws.ToString(resp.AccountAssignmentDeletionStatus.FailureReason))
		case ssoadmintypes.StatusValuesInProgress:
			// continue waiting
		}

		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-c.Clock.After(2 * time.Second):
			continue
		}
	}
}

// ValidateResourceSyncCredential fetches resources (with MaxResults=1) to validate that the credential
// set up for Identity Center resource sync is correctly configured. Endpoints used for principal
// permission assignments are not invoked. The following permissions are validated:
// - "sso:DescribeInstance"
// - "organizations:ListAccounts"
// - "identitystore:ListUsers"
// - "identitystore:ListGroups"
// - "identitystore:ListGroupMemberships" (skipped if 0 groups found)
// - "sso:ListPermissionSets"
// - "sso:DescribePermissionSet" (skipped if 0 permission sets found)
// - "sso:ListAccountAssignmentsForPrincipal" (skipped if 0 groups and/or 0 users found)
// - "sso:ListPermissionSetsProvisionedToAccount" (skipped if 0 accounts found)
// See StatementForAWSIdentityCenterAccess for a full list of permissions required for Identity Center integration.
func (c *client) ValidateResourceSyncCredential(ctx context.Context) error {
	c.Logger.InfoContext(ctx, "Validating credential set up for AWS IAM Identity Center integration resource sync.")
	instance, err := c.ssoAdminClient.DescribeInstance(ctx, &ssoadmin.DescribeInstanceInput{
		InstanceArn: aws.String(c.InstanceARN),
	})
	if err != nil {
		return trace.Wrap(traceError(err))
	}
	resultSize := int32(1)
	g, err := c.identityStoreClient.ListGroups(ctx, &identitystore.ListGroupsInput{
		IdentityStoreId: instance.IdentityStoreId,
		MaxResults:      &resultSize,
	})
	if err != nil {
		return trace.Wrap(traceError(err))
	}
	if len(g.Groups) > 0 {
		if _, err := c.ssoAdminClient.ListAccountAssignmentsForPrincipal(ctx, &ssoadmin.ListAccountAssignmentsForPrincipalInput{
			InstanceArn:   aws.String(c.InstanceARN),
			MaxResults:    &resultSize,
			PrincipalType: ssoadmintypes.PrincipalTypeGroup,
			PrincipalId:   g.Groups[0].GroupId,
		}); err != nil {
			return trace.Wrap(traceError(err))
		}
		if _, err := c.identityStoreClient.ListGroupMemberships(ctx, &identitystore.ListGroupMembershipsInput{
			IdentityStoreId: instance.IdentityStoreId,
			GroupId:         g.Groups[0].GroupId,
			MaxResults:      &resultSize,
		}); err != nil {
			return trace.Wrap(traceError(err))
		}
	} else {
		c.Logger.DebugContext(ctx, "Groups not found. Group member and permission assignment query will be skipped.")
	}

	u, err := c.identityStoreClient.ListUsers(ctx, &identitystore.ListUsersInput{
		IdentityStoreId: instance.IdentityStoreId,
		MaxResults:      &resultSize,
	})
	if err != nil {
		return trace.Wrap(traceError(err))
	}
	if len(u.Users) > 0 {
		if _, err := c.ssoAdminClient.ListAccountAssignmentsForPrincipal(ctx, &ssoadmin.ListAccountAssignmentsForPrincipalInput{
			InstanceArn:   aws.String(c.InstanceARN),
			MaxResults:    &resultSize,
			PrincipalType: ssoadmintypes.PrincipalTypeUser,
			PrincipalId:   u.Users[0].UserId,
		}); err != nil {
			return trace.Wrap(traceError(err))
		}
	} else {
		c.Logger.DebugContext(ctx, "Users not found. User permission assignment query is skipped.")
	}

	a, err := c.organizationsClient.ListAccounts(ctx, &organizations.ListAccountsInput{
		MaxResults: &resultSize,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if len(a.Accounts) > 0 {
		if _, err := c.ssoAdminClient.ListPermissionSetsProvisionedToAccount(ctx, &ssoadmin.ListPermissionSetsProvisionedToAccountInput{
			InstanceArn: aws.String(c.InstanceARN),
			AccountId:   a.Accounts[0].Id,
			MaxResults:  &resultSize,
		}); err != nil {
			return trace.Wrap(traceError(err))
		}
	} else {
		c.Logger.DebugContext(ctx, "Accounts not found. Account permission assignment query is skipped.")
	}

	ps, err := c.ssoAdminClient.ListPermissionSets(ctx, &ssoadmin.ListPermissionSetsInput{
		InstanceArn: aws.String(c.InstanceARN),
		MaxResults:  &resultSize,
	})
	if err != nil {
		return trace.Wrap(traceError(err))
	}
	if len(ps.PermissionSets) > 0 {
		if _, err := c.ssoAdminClient.DescribePermissionSet(ctx, &ssoadmin.DescribePermissionSetInput{
			InstanceArn:      aws.String(c.InstanceARN),
			PermissionSetArn: aws.String(ps.PermissionSets[0]),
		}); err != nil {
			return trace.Wrap(traceError(err))
		}
	} else {
		c.Logger.DebugContext(ctx, "Permission sets not found. Describe permission set query is skipped.")
	}

	return nil
}

// traceError converts AWS sdk v2 error type to trace error.
// AccessDeniedException error is converted to bad parameter error.
// Other error types are returned as their respective trace error types
// converted by ConvertRequestFailureError.
func traceError(err error) error {
	var ssoAdminErr *ssoadmintypes.AccessDeniedException
	var idStoreErr *identitystoretypes.AccessDeniedException
	if errors.As(err, &ssoAdminErr) {
		return trace.BadParameter("Invalid credential. %s", err.Error())
	}
	if errors.As(err, &idStoreErr) {
		return trace.BadParameter("Invalid credential. %s", err.Error())
	}

	return libcloudaws.ConvertRequestFailureError(err)
}
