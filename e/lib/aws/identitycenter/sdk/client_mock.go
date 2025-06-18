package sdk

import (
	"context"
	"slices"
	"sync"

	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
	"github.com/gravitational/trace"
)

// NewClientMock creates and returns a new instance of ClientMock.
func NewClientMock(customMockData *MockedAWSStateType) *ClientMock {
	client := &ClientMock{
		MockedAWSStateType: NewMockedAWSState(),
	}
	if customMockData != nil {
		client.MockedAWSStateType = *customMockData
	}
	return client
}

// ClientMock is a mock implementation of the Client interface for testing purposes.
type ClientMock struct {
	Mu sync.Mutex
	MockedAWSStateType

	// MonkeyPatch allows tests to override the default behavior of a mock
	// instance.
	MonkeyPatch struct {
		DescribeInstance               func(context.Context) (*InstanceInfo, error)
		ListPermissionSets             func(context.Context) ([]*PermissionSet, error)
		CreateAccountAssignment        func(context.Context, *CreateAccountAssignmentRequest) (*AccountAssignmentResponse, error)
		DeleteAccountAssignment        func(context.Context, *DeleteAccountAssignmentRequest) (*AccountAssignmentResponse, error)
		CreateAccountAssignmentCounter func()
		DeleteAccountAssignmentCounter func()
	}
}

// NewMockedAWSState returns a default mock state.
func NewMockedAWSState() MockedAWSStateType {
	return MockedAWSStateType{
		Info: InstanceInfo{
			OwnerAccountID:  "2222222222",
			Name:            "Mock Identity Center Instance",
			IdentityStoreID: "store1",
			Status:          ssoadmintypes.InstanceStatusActive,
		},
		Accounts: []*Account{
			{Name: "Account1", ID: "1111111111", ARN: "arn:aws:iam::1111111111:account/Account1"},
			{Name: "Account2", ID: "2222222222", ARN: "arn:aws:iam::2222222222:account/Account2"},
		},
		PermissionSets: []*PermissionSet{
			{Name: "Admin", ARN: "arn:aws:sso:::permissionSet/Admin", Description: "Admin permissions"},
			{Name: "ReadOnly", ARN: "arn:aws:sso:::permissionSet/ReadOnly", Description: "Read-only permissions"},
		},
		Users: []*User{
			{ID: "user1", UserName: "user_one"},
			{ID: "user2", UserName: "user_two"},
		},
		Groups: []*Group{
			{DisplayName: "Group1", ID: "group1", IdentityStoreID: "store1"},
			{DisplayName: "Group2", ID: "group2", IdentityStoreID: "store1"},
		},
		GroupMemberships: map[string][]*GroupMember{
			"group1": {
				{MemberID: "user1"},
				{MemberID: "user2"},
			},
			"group2": {
				{MemberID: "user2"},
			},
		},
		UserAssignments: map[string][]*Assignment{
			"user1": {
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:sso:::permissionSet/Admin"},
				{AccountID: "2222222222", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
			},
			"user2": {
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
			},
		},
		GroupAssignments: map[string][]*Assignment{
			"group1": {
				{AccountID: "1111111111", PermissionSetARN: "arn:aws:sso:::permissionSet/Admin"},
			},
			"group2": {
				{AccountID: "2222222222", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
			},
		},
		AccountPermAssignments: map[string][]string{
			"1111111111": {"arn:aws:sso:::permissionSet/Admin", "arn:aws:sso:::permissionSet/ReadOnly"},
			"2222222222": {"arn:aws:sso:::permissionSet/ReadOnly"},
		},
	}
}

// DefaultMockedData defines a default MockedAWSStateType values used for
// testing mocked AWS state.
var DefaultMockedData = MockedAWSStateType{
	Info: InstanceInfo{
		OwnerAccountID:  "2222222222",
		Name:            "Mock Identity Center Instance",
		IdentityStoreID: "store1",
		Status:          ssoadmintypes.InstanceStatusActive,
	},
	Accounts: []*Account{
		{Name: "Account1", ID: "1111111111", ARN: "arn:aws:iam::1111111111:account/Account1"},
		{Name: "Account2", ID: "2222222222", ARN: "arn:aws:iam::2222222222:account/Account2"},
	},
	PermissionSets: []*PermissionSet{
		{Name: "Admin", ARN: "arn:aws:sso:::permissionSet/Admin", Description: "Admin permissions"},
		{Name: "ReadOnly", ARN: "arn:aws:sso:::permissionSet/ReadOnly", Description: "Read-only permissions"},
	},
	Users: []*User{
		{ID: "user1", UserName: "user_one"},
		{ID: "user2", UserName: "user_two"},
	},
	Groups: []*Group{
		{DisplayName: "Group1", ID: "group1", IdentityStoreID: "store1"},
		{DisplayName: "Group2", ID: "group2", IdentityStoreID: "store1"},
	},
	GroupMemberships: map[string][]*GroupMember{
		"group1": {
			{MemberID: "user1"},
			{MemberID: "user2"},
		},
		"group2": {
			{MemberID: "user2"},
		},
	},
	UserAssignments: map[string][]*Assignment{
		"user1": {
			{AccountID: "1111111111", PermissionSetARN: "arn:aws:sso:::permissionSet/Admin"},
			{AccountID: "2222222222", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
		},
		"user2": {
			{AccountID: "1111111111", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
		},
	},
	GroupAssignments: map[string][]*Assignment{
		"group1": {
			{AccountID: "1111111111", PermissionSetARN: "arn:aws:sso:::permissionSet/Admin"},
		},
		"group2": {
			{AccountID: "2222222222", PermissionSetARN: "arn:aws:sso:::permissionSet/ReadOnly"},
		},
	},
	AccountPermAssignments: map[string][]string{
		"1111111111": {"arn:aws:sso:::permissionSet/Admin", "arn:aws:sso:::permissionSet/ReadOnly"},
		"2222222222": {"arn:aws:sso:::permissionSet/ReadOnly"},
	},
}

// MockedAWSStateType is a struct that holds the mocked AWS state.
type MockedAWSStateType struct {
	// Info holds information about the Identoty Center instance
	Info InstanceInfo
	// Accounts is a list of mocked accounts.
	Accounts []*Account
	// PermissionSets is a list of mocked permission sets.
	PermissionSets []*PermissionSet
	// Users is a list of mocked users.
	Users []*User
	// Groups is a list of mocked groups.
	Groups []*Group
	// GroupMemberships is a map of group ID to a list of group members.
	GroupMemberships map[string][]*GroupMember
	// UserAssignments is a map of user ID to a list of assignments.
	UserAssignments map[string][]*Assignment
	// GroupAssignments is a map of group ID to a list of assignments.
	GroupAssignments map[string][]*Assignment
	// AccountPermAssignments is a map of account ID to a list of permission set ARNs.
	AccountPermAssignments map[string][]string
}

// DescribeInstance returns a mocked InstanceInfo
func (c *ClientMock) DescribeInstance(ctx context.Context) (*InstanceInfo, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if c.MonkeyPatch.DescribeInstance != nil {
		return c.MonkeyPatch.DescribeInstance(ctx)
	}
	info := c.Info
	return &info, nil
}

// ListAccounts returns a list of mocked accounts.
func (c *ClientMock) ListAccounts(_ context.Context) ([]*Account, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.Accounts, nil
}

// ListPermissionSetARNsForAccount returns a list of permission set ARNs assigned to an account.
func (c *ClientMock) ListPermissionSetARNsForAccount(ctx context.Context, accountID string) ([]string, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.AccountPermAssignments[accountID], nil
}

// ListPermissionSets returns a list of mocked permission sets.
func (c *ClientMock) ListPermissionSets(ctx context.Context) ([]*PermissionSet, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if c.MonkeyPatch.ListPermissionSets != nil {
		return c.MonkeyPatch.ListPermissionSets(ctx)
	}
	return c.PermissionSets, nil
}

// ListGroups returns a list of mocked groups.
func (c *ClientMock) ListGroups(ctx context.Context) ([]*Group, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.Groups, nil
}

// ListGroupMemberships returns a list of group members for a given group ID.
func (c *ClientMock) ListGroupMemberships(_ context.Context, groupID string) ([]*GroupMember, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.GroupMemberships[groupID], nil
}

// ListUsers returns a list of mocked users.
func (c *ClientMock) ListUsers(context.Context) ([]*User, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.Users, nil
}

// ListUserAssignments returns a list of permission assignments for a given user ID.
func (c *ClientMock) ListUserAssignments(ctx context.Context, userID string) ([]*Assignment, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.UserAssignments[userID], nil
}

// ListGroupsAssignments returns a list of permission assignments for a given group ID.
func (c *ClientMock) ListGroupsAssignments(ctx context.Context, groupID string) ([]*Assignment, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return c.GroupAssignments[groupID], nil
}

// WaitForDeleteAccountAssignmentResult waits until the account assignment creation reaches a terminal state.
func (c *ClientMock) WaitForCreateAccountAssignmentResult(ctx context.Context, requestID string) error {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return nil
}

// CreateAccountAssignment adds a new assignment based on the request parameters.
func (c *ClientMock) CreateAccountAssignment(ctx context.Context, req *CreateAccountAssignmentRequest) (*AccountAssignmentResponse, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if c.MonkeyPatch.CreateAccountAssignmentCounter != nil {
		c.MonkeyPatch.CreateAccountAssignmentCounter()
	}

	switch req.PrincipalType {
	case ssoadmintypes.PrincipalTypeUser:
		c.UserAssignments[req.PrincipalID] = append(c.UserAssignments[req.PrincipalID], &Assignment{
			AccountID:        req.AccountID,
			PermissionSetARN: req.PermissionSetARN,
			PrincipalType:    req.PrincipalType,
		})
	case ssoadmintypes.PrincipalTypeGroup:
		c.GroupAssignments[req.PrincipalID] = append(c.GroupAssignments[req.PrincipalID], &Assignment{
			AccountID:        req.AccountID,
			PermissionSetARN: req.PermissionSetARN,
			PrincipalType:    req.PrincipalType,
		})
	default:
		return nil, trace.BadParameter("unsupported principal type")
	}

	return &AccountAssignmentResponse{
		RequestID: "mockRequestID",
		Status:    ssoadmintypes.StatusValuesSucceeded,
	}, nil
}

// DeleteAccountAssignment removes an assignment for the specified user.
func (c *ClientMock) DeleteAccountAssignment(ctx context.Context, req *DeleteAccountAssignmentRequest) (*AccountAssignmentResponse, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	if c.MonkeyPatch.DeleteAccountAssignmentCounter != nil {
		c.MonkeyPatch.DeleteAccountAssignmentCounter()
	}

	var principalAssignments map[string][]*Assignment
	var curr map[string][]*Assignment
	switch req.PrincipalType {
	case ssoadmintypes.PrincipalTypeUser:
		principalAssignments = c.UserAssignments
		curr = c.UserAssignments
	case ssoadmintypes.PrincipalTypeGroup:
		principalAssignments = c.GroupAssignments
		curr = c.GroupAssignments
	default:
		return nil, trace.BadParameter("unsupported principal target type %T", req.PrincipalType)
	}

	assignments, ok := principalAssignments[req.PrincipalID]
	if !ok {
		return nil, trace.BadParameter("assignment not found")
	}

	for i, assignment := range assignments {
		if assignment.PermissionSetARN == req.PermissionSetARN && assignment.AccountID == req.AccountID {
			curr[req.PrincipalID] = slices.Delete(assignments, i, i+1)
			return &AccountAssignmentResponse{
				RequestID: "mockRequestID",
				Status:    ssoadmintypes.StatusValuesSucceeded,
			}, nil
		}
	}
	return nil, trace.NotFound("assignment not found")
}

// WaitForDeleteAccountAssignmentResult waits until the account assignment deletion reaches a terminal state.
func (c *ClientMock) WaitForDeleteAccountAssignmentResult(ctx context.Context, requestID string) error {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	return nil
}

func (c *ClientMock) ListAssignments(ctx context.Context, principalID string, principalType ssoadmintypes.PrincipalType) ([]*Assignment, error) {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	var userAssignments, groupAssignments []*Assignment
	switch principalType {
	case ssoadmintypes.PrincipalTypeUser:
		userAssignments = append(userAssignments, c.UserAssignments[principalID]...)
		for _, v := range userAssignments {
			v.PrincipalType = ssoadmintypes.PrincipalTypeUser
		}
		for groupID, members := range c.GroupMemberships {
			for _, member := range members {
				if member.MemberID == principalID {
					// Add group assignments for this user
					groupAssignments = append(groupAssignments, c.GroupAssignments[groupID]...)
				}
			}
		}
		for _, v := range groupAssignments {
			v.PrincipalType = ssoadmintypes.PrincipalTypeGroup
		}
		return append(userAssignments, groupAssignments...), nil
	case ssoadmintypes.PrincipalTypeGroup:
		for _, v := range c.GroupAssignments[principalID] {
			v.PrincipalType = ssoadmintypes.PrincipalTypeGroup
		}
		return c.GroupAssignments[principalID], nil
	default:
		return nil, trace.BadParameter("unsupported principal type %q", principalType)
	}
}

// ValidateResourceSyncCredential only validates DescribeInstance permission.
func (c *ClientMock) ValidateResourceSyncCredential(ctx context.Context) error {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	if c.MonkeyPatch.DescribeInstance != nil {
		if _, err := c.MonkeyPatch.DescribeInstance(ctx); err != nil {
			return err
		}
	}

	return nil
}
