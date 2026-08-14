package sdk

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	idstoretypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	orgtypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestClientConnection(t *testing.T) {
	const (
		instanceARN = "arn:aws:sso:::instance/ssoins-1234567891234567"
	)
	ctx := context.Background()
	mockSDK := &mockAWSClient{}
	c := &client{
		Config: &Config{
			InstanceARN: instanceARN,
		},
		identityStoreClient: mockSDK,
		organizationsClient: mockSDK,
		ssoAdminClient:      mockSDK,
	}

	aResp, err := c.ListAccounts(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, accounts, aResp)

	gResp, err := c.ListGroups(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, groups, gResp)

	t.Run("ListGroupMemberships", func(t *testing.T) {
		t.Run("Lists members", func(t *testing.T) {
			gWithMembersResp, err := c.ListGroupMemberships(ctx, "g1")
			require.NoError(t, err)
			require.ElementsMatch(t, groupMembers, gWithMembersResp)
		})

		t.Run("Returns trace NotFound on unknown group", func(t *testing.T) {
			_, err := c.ListGroupMemberships(ctx, "no-such-group")
			require.True(t, trace.IsNotFound(err), "Expected trace.NotFoundError, got %T, %v", err, err)
		})
	})

	aWithPSetARNsResp, err := c.ListPermissionSetARNsForAccount(ctx, "a1")
	require.NoError(t, err)
	require.ElementsMatch(t, pARNFromPSetMap(permSetMap), aWithPSetARNsResp)

	pSetResp, err := c.ListPermissionSets(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, permMapToPermSet(permSetMap), pSetResp)

	uResp, err := c.ListUsers(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, users, uResp)

	caaResp, err := c.CreateAccountAssignment(ctx, &CreateAccountAssignmentRequest{
		PrincipalID:      "test@example.com",
		PrincipalType:    ssoadmintypes.PrincipalTypeUser,
		AccountID:        "some-account-id",
		PermissionSetARN: "arn:aws:sso:::permissionSet/Admin",
	})
	require.NoError(t, err)
	require.Equal(t, accountAssignmentRequestID, caaResp.RequestID)
	require.Equal(t, ssoadmintypes.StatusValuesInProgress, caaResp.Status)
	require.Empty(t, caaResp.FailureReason)

	daaResp, err := c.DeleteAccountAssignment(ctx, &DeleteAccountAssignmentRequest{
		PrincipalID:      "test@example.com",
		PrincipalType:    ssoadmintypes.PrincipalTypeUser,
		AccountID:        "some-account-id",
		PermissionSetARN: "arn:aws:sso:::permissionSet/Admin",
	})
	require.NoError(t, err)
	require.Equal(t, accountAssignmentRequestID, daaResp.RequestID)
	require.Equal(t, ssoadmintypes.StatusValuesInProgress, daaResp.Status)
	require.Empty(t, daaResp.FailureReason)
}

type mockAWSClient struct {
	ssoadmin.Client
}

func (mockAWSClient) ListAccounts(ctx context.Context, params *organizations.ListAccountsInput, optFns ...func(*organizations.Options)) (*organizations.ListAccountsOutput, error) {
	return &organizations.ListAccountsOutput{
		Accounts: toAccounts(accounts),
	}, nil
}

func (mockAWSClient) ListGroups(ctx context.Context, params *identitystore.ListGroupsInput, optFns ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error) {
	return &identitystore.ListGroupsOutput{
		Groups: toGroups(groups),
	}, nil
}

func (mockAWSClient) ListUsers(ctx context.Context, params *identitystore.ListUsersInput, optFns ...func(*identitystore.Options)) (*identitystore.ListUsersOutput, error) {
	return &identitystore.ListUsersOutput{
		Users: toUsers(users),
	}, nil
}

func (mockAWSClient) ListGroupMemberships(ctx context.Context, params *identitystore.ListGroupMembershipsInput, optFns ...func(*identitystore.Options)) (*identitystore.ListGroupMembershipsOutput, error) {
	gid := aws.ToString(params.GroupId)
	if slices.IndexFunc(groups, func(g *Group) bool { return g.ID == gid }) == -1 {
		return nil, mockAWSError(http.StatusBadRequest, "identitystore", "ListGroupMemberships",
			&idstoretypes.ResourceNotFoundException{
				Message:      aws.String("GROUP Not Found"),
				ResourceType: "GROUP",
				RequestId:    aws.String("some-request-id"),
			})
	}

	return &identitystore.ListGroupMembershipsOutput{
		GroupMemberships: toGroupMemberships(groupMembers),
	}, nil
}

func (mockAWSClient) ListPermissionSetsProvisionedToAccount(ctx context.Context, params *ssoadmin.ListPermissionSetsProvisionedToAccountInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.ListPermissionSetsProvisionedToAccountOutput, error) {
	return &ssoadmin.ListPermissionSetsProvisionedToAccountOutput{
		PermissionSets: pARNFromPSetMap(permSetMap),
	}, nil
}

func (mockAWSClient) ListPermissionSets(ctx context.Context, params *ssoadmin.ListPermissionSetsInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.ListPermissionSetsOutput, error) {
	// TODO(sshah): instead of hardcoded value, we can build this dynamically
	return &ssoadmin.ListPermissionSetsOutput{
		PermissionSets: pARNFromPSetMap(permSetMap),
	}, nil
}

func (mockAWSClient) ListAccountAssignmentsForPrincipal(ctx context.Context, params *ssoadmin.ListAccountAssignmentsForPrincipalInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.ListAccountAssignmentsForPrincipalOutput, error) {
	return &ssoadmin.ListAccountAssignmentsForPrincipalOutput{}, nil
}

func (mockAWSClient) DescribeInstance(ctx context.Context, params *ssoadmin.DescribeInstanceInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.DescribeInstanceOutput, error) {
	return &ssoadmin.DescribeInstanceOutput{}, nil
}

func (mockAWSClient) DescribePermissionSet(ctx context.Context, params *ssoadmin.DescribePermissionSetInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.DescribePermissionSetOutput, error) {
	return &ssoadmin.DescribePermissionSetOutput{
		PermissionSet: toPermissionSet(permSetMap[*params.PermissionSetArn]),
	}, nil
}

func (mockAWSClient) CreateAccountAssignment(ctx context.Context, req *ssoadmin.CreateAccountAssignmentInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.CreateAccountAssignmentOutput, error) {
	if aws.ToString(req.InstanceArn) == "" {
		return nil, trace.BadParameter("missing Instance ARN")
	}

	if aws.ToString(req.PermissionSetArn) == "" {
		return nil, trace.BadParameter("missing PermissionSet ARN")
	}

	if req.TargetType == "" {
		return nil, trace.BadParameter("missing Target Type")
	}

	if aws.ToString(req.TargetId) == "" {
		return nil, trace.BadParameter("missing Target ID")
	}

	if req.PrincipalType == "" {
		return nil, trace.BadParameter("missing Principal Type")
	}

	if aws.ToString(req.PrincipalId) == "" {
		return nil, trace.BadParameter("missing Principal ID")
	}

	return &ssoadmin.CreateAccountAssignmentOutput{
		AccountAssignmentCreationStatus: &ssoadmintypes.AccountAssignmentOperationStatus{
			Status:           ssoadmintypes.StatusValuesInProgress,
			RequestId:        aws.String(accountAssignmentRequestID),
			TargetType:       req.TargetType,
			TargetId:         req.TargetId,
			PrincipalType:    req.PrincipalType,
			PrincipalId:      req.PrincipalId,
			PermissionSetArn: req.PermissionSetArn,
		},
	}, nil
}

func (mockAWSClient) DeleteAccountAssignment(ctx context.Context, req *ssoadmin.DeleteAccountAssignmentInput, _ ...func(*ssoadmin.Options)) (*ssoadmin.DeleteAccountAssignmentOutput, error) {

	if aws.ToString(req.InstanceArn) == "" {
		return nil, trace.BadParameter("missing Instance ARN")
	}

	if aws.ToString(req.PermissionSetArn) == "" {
		return nil, trace.BadParameter("missing PermissionSet ARN")
	}

	if req.TargetType == "" {
		return nil, trace.BadParameter("missing Target Type")
	}

	if aws.ToString(req.TargetId) == "" {
		return nil, trace.BadParameter("missing Target ID")
	}

	if req.PrincipalType == "" {
		return nil, trace.BadParameter("missing Principal Type")
	}

	if aws.ToString(req.PrincipalId) == "" {
		return nil, trace.BadParameter("missing Principal ID")
	}

	return &ssoadmin.DeleteAccountAssignmentOutput{
		AccountAssignmentDeletionStatus: &ssoadmintypes.AccountAssignmentOperationStatus{
			Status:           ssoadmintypes.StatusValuesInProgress,
			RequestId:        aws.String(accountAssignmentRequestID),
			TargetType:       req.TargetType,
			TargetId:         req.TargetId,
			PrincipalType:    req.PrincipalType,
			PrincipalId:      req.PrincipalId,
			PermissionSetArn: req.PermissionSetArn,
		},
	}, nil
}

// mockAWSError wraps an underlying error in an approximation of the error chain
// returned from the AWS client on error.
func mockAWSError(httpStatus int, serviceID, operationName string, rootErr error) error {
	return &smithy.OperationError{
		ServiceID:     serviceID,
		OperationName: operationName,
		Err: &smithyhttp.ResponseError{
			Response: &smithyhttp.Response{Response: &http.Response{StatusCode: httpStatus}},
			Err:      rootErr,
		},
	}
}

const accountAssignmentRequestID = "some request id"

func toAccounts(acc []*Account) []orgtypes.Account {
	var out []orgtypes.Account
	for _, a := range acc {
		out = append(out, orgtypes.Account{
			Arn:  aws.String(a.ARN),
			Id:   aws.String(a.ID),
			Name: aws.String(a.Name),
		})
	}
	return out
}

func toGroups(groups []*Group) []idstoretypes.Group {
	var out []idstoretypes.Group
	for _, g := range groups {
		out = append(out, idstoretypes.Group{
			GroupId:     aws.String(g.ID),
			DisplayName: aws.String(g.DisplayName),
		})
	}
	return out
}

func toUsers(users []*User) []idstoretypes.User {
	var out []idstoretypes.User
	for _, u := range users {
		out = append(out, idstoretypes.User{
			UserId:   aws.String(u.ID),
			UserName: aws.String(u.UserName),
		})
	}
	return out
}

func toGroupMemberships(gMembers []*GroupMember) []idstoretypes.GroupMembership {
	var out []idstoretypes.GroupMembership
	for _, gm := range gMembers {
		out = append(out, idstoretypes.GroupMembership{
			IdentityStoreId: aws.String("id1"),
			GroupId:         aws.String("g1"),
			MembershipId:    aws.String("g1"),
			MemberId:        &idstoretypes.MemberIdMemberUserId{Value: gm.MemberID},
		})
	}
	return out
}

func toPermissionSet(pset *PermissionSet) *ssoadmintypes.PermissionSet {
	return &ssoadmintypes.PermissionSet{
		Name:             aws.String(pset.Name),
		PermissionSetArn: aws.String(pset.ARN),
		Description:      aws.String(pset.Description),
	}
}

func permMapToPermSet(m PermissionSetMap) []*PermissionSet {
	var out []*PermissionSet
	for _, pset := range m {
		out = append(out, &PermissionSet{
			Name:        pset.Name,
			ARN:         pset.ARN,
			Description: pset.Description,
		})
	}

	return out
}

func pARNFromPSetMap(pSets PermissionSetMap) []string {
	var out []string
	for _, pset := range pSets {
		out = append(out, pset.ARN)
	}
	return out
}

var accounts = []*Account{
	{
		Name: "a1",
		ID:   "a1",
		ARN:  "arn1",
	},
	{
		Name: "a2",
		ID:   "a2",
		ARN:  "arn2",
	},
}

var groups = []*Group{
	{
		DisplayName: "g1",
		ID:          "g1",
	},
	{
		DisplayName: "g2",
		ID:          "g2",
	},
}

var groupMembers = []*GroupMember{
	{
		MemberID: "u1",
	},
	{
		MemberID: "u2",
	},
	{
		MemberID: "u3",
	},
}

var users = []*User{
	{
		UserName: "u1",
		ID:       "u1",
	},
	{
		UserName: "u2",
		ID:       "u2",
	},
}

var permSetMap = PermissionSetMap{
	"parn1": &PermissionSet{

		Name:        "p1",
		ARN:         "parn1",
		Description: "p1",
	},
	"parn2": &PermissionSet{
		Name:        "p2",
		ARN:         "parn2",
		Description: "p2",
	},
}
